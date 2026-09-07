package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/notify"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/postgres"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"golang.org/x/term"
	"gorm.io/gorm"
)

func runJob(ctx context.Context, gdb *gorm.DB, cfg config.Config, log *slog.Logger, name string) error {
	deps, err := newDeps(gdb, cfg, log, notify.NewSMTP(cfg, gdb, log))
	if err != nil {
		return err
	}
	switch name {
	case "auto-cancel":
		n, err := deps.AutoCancel(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("auto-cancel: %d rencana dibatalkan\n", n)
		// The job queues notifications; flushing here means the operator sees
		// one command do the whole thing rather than wondering why nothing
		// arrived.
		sent, failed, err := deps.FlushNotifications(ctx, 200)
		if err != nil {
			return err
		}
		fmt.Printf("notifikasi: %d terkirim, %d gagal\n", sent, failed)
		return nil

	case "import":
		results, err := deps.ImportDropPath(ctx, nil)
		if err != nil {
			return err
		}
		for _, r := range results {
			if r.Skipped {
				fmt.Printf("%-40s dilewati (checksum sudah pernah dimuat)\n", r.Run.FileName)
				continue
			}
			fmt.Printf("%-40s %s  dibaca=%d dimasukkan=%d dilewati=%d ditolak=%d %s\n",
				r.Run.FileName, r.Run.Outcome, r.Run.RowsRead, r.Run.RowsInserted,
				r.Run.RowsSkipped, r.Run.RowsRejected, r.Run.Message)
		}
		if len(results) == 0 {
			fmt.Println("tidak ada berkas untuk diimpor")
		}
		return nil

	case "notify":
		sent, failed, err := deps.FlushNotifications(ctx, 500)
		if err != nil {
			return err
		}
		fmt.Printf("notifikasi: %d terkirim, %d gagal\n", sent, failed)
		return nil

	case "purge-sessions":
		n, err := deps.PurgeSessions(ctx)
		if err != nil {
			return err
		}
		// Sessions only. BR-8.5: audit and approval history is never purged.
		fmt.Printf("%d baris sesi kedaluwarsa dihapus (audit tidak pernah dihapus)\n", n)
		return nil

	default:
		return errors.New("job auto-cancel | import | notify | purge-sessions")
	}
}

// userCommand creates a staff account interactively. Accounts are created by
// an administrator; there is no self-service registration (BR-5.2).
func userCommand(ctx context.Context, gdb *gorm.DB, sub string) error {
	if sub != "create" {
		return errors.New("user create")
	}
	in := bufio.NewReader(os.Stdin)

	email, err := prompt(in, "Surel: ")
	if err != nil {
		return err
	}
	email, err = sanitize.Email(email)
	if err != nil {
		return fmt.Errorf("alamat surel tidak sah: %w", err)
	}
	name, err := prompt(in, "Nama lengkap: ")
	if err != nil {
		return err
	}
	name, err = sanitize.Text(name, 200)
	if err != nil {
		return err
	}

	fmt.Printf("Kata sandi (minimal %d karakter, tidak ditampilkan): ",
		postgres.NewParamRepo(gdb).Int(ctx, app.ParamPasswordMinLen, security.DefaultPasswordMinLength))
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return err
	}
	minLen := postgres.NewParamRepo(gdb).Int(ctx, app.ParamPasswordMinLen,
		security.DefaultPasswordMinLength)
	if err := security.CheckPasswordStrength(string(pw), minLen); err != nil {
		return fmt.Errorf("kata sandi minimal %d karakter", minLen)
	}

	fmt.Println("\nPeran yang tersedia:")
	roles := map[string]uuid.UUID{}
	rows, err := gdb.Raw(`SELECT role_id, role_code, label_id FROM role ORDER BY role_code`).Rows()
	if err != nil {
		return err
	}
	for rows.Next() {
		var rid uuid.UUID
		var code, label string
		if err := rows.Scan(&rid, &code, &label); err != nil {
			rows.Close()
			return err
		}
		roles[code] = rid
		fmt.Printf("  %-18s %s\n", code, label)
	}
	rows.Close()

	roleCode, err := prompt(in, "Kode peran: ")
	if err != nil {
		return err
	}
	roleID, ok := roles[strings.TrimSpace(roleCode)]
	if !ok {
		return fmt.Errorf("peran tidak dikenal: %s", roleCode)
	}

	fmt.Println("\nPerusahaan yang tersedia:")
	companies := map[string]uuid.UUID{}
	crows, err := gdb.Raw(`SELECT company_id, company_code, company_name FROM company ORDER BY company_code`).Rows()
	if err != nil {
		return err
	}
	for crows.Next() {
		var cid uuid.UUID
		var code, cname string
		if err := crows.Scan(&cid, &code, &cname); err != nil {
			crows.Close()
			return err
		}
		companies[code] = cid
		fmt.Printf("  %-10s %s\n", code, cname)
	}
	crows.Close()

	// D37: one or more companies, explicitly. There is no "all" value.
	list, err := prompt(in, "Kode perusahaan (pisahkan dengan koma, minimal satu): ")
	if err != nil {
		return err
	}
	var grants []app.Grant
	for _, code := range strings.Split(list, ",") {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		cid, ok := companies[code]
		if !ok {
			return fmt.Errorf("perusahaan tidak dikenal: %s", code)
		}
		grants = append(grants, app.Grant{RoleID: roleID, CompanyID: cid})
	}
	if len(grants) == 0 {
		return errors.New("minimal satu perusahaan wajib dipilih (BR-5.4)")
	}

	hash, err := security.HashPassword(string(pw))
	if err != nil {
		return err
	}
	repo := postgres.NewUserRepo(gdb)
	uid, err := repo.Create(ctx, app.User{Email: email, FullName: name, PasswordHash: hash},
		grants, nil)
	if err != nil {
		return err
	}
	fmt.Printf("\nPengguna dibuat: %s (%s)\n", email, uid)
	fmt.Println("TOTP wajib: pendaftaran dilakukan saat login pertama (BR-5.3).")
	return nil
}

func prompt(r *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	s, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}
