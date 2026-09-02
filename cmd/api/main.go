// Command mc is the marketing_calendar service and its operational commands.
//
// Thin by design (CLAUDE.md §2): it wires and runs. Every behaviour lives in
// a package that can be tested without it.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stevenwilliam/marketing_calendar/db"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/httpapi"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/notify"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/database"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/logging"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() string {
	return `mc — marketing_calendar

  mc serve                 run the HTTP service
  mc migrate up            apply pending migrations
  mc migrate down          roll back the most recent migration (development only)
  mc migrate status        show applied and pending migrations
  mc seed                  load demo data (idempotent, re-runnable)
  mc job auto-cancel       cancel plans whose approval is incomplete (BR-4.6)
  mc job import            import transaction CSVs from the drop path
  mc job notify            flush the notification queue
  mc user create           create a staff account interactively
`
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Print(usage())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.LogLevel)

	gdb, err := database.Open(cfg.DatabaseURL, cfg.LogLevel == "debug")
	if err != nil {
		return err
	}

	migrations, err := database.Load(db.Migrations, "migrations")
	if err != nil {
		return err
	}

	switch os.Args[1] {
	case "migrate":
		return migrate(gdb, migrations, arg(2, "status"))

	case "seed":
		n, err := app.Seed(context.Background(), gdb)
		if err != nil {
			return err
		}
		log.Info("seed complete", slog.Int("rows", n))
		return nil

	case "job":
		return runJob(context.Background(), gdb, cfg, log, arg(2, ""))

	case "user":
		return userCommand(context.Background(), gdb, arg(2, ""))

	case "serve":
		return serve(gdb, cfg, log, migrations)

	default:
		fmt.Print(usage())
		return fmt.Errorf("perintah tidak dikenal: %s", os.Args[1])
	}
}

func arg(i int, def string) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return def
}

func migrate(gdb *gorm.DB, migrations []database.Migration, sub string) error {
	switch sub {
	case "up":
		done, err := database.Up(gdb, migrations)
		if err != nil {
			return err
		}
		if len(done) == 0 {
			fmt.Println("tidak ada migrasi yang tertunda")
			return nil
		}
		for _, m := range done {
			fmt.Printf("applied %04d_%s\n", m.Version, m.Name)
		}
		return nil
	case "down":
		m, err := database.Down(gdb, migrations)
		if err != nil {
			return err
		}
		if m == nil {
			fmt.Println("tidak ada migrasi untuk dibatalkan")
			return nil
		}
		fmt.Printf("rolled back %04d_%s\n", m.Version, m.Name)
		return nil
	case "status":
		applied, pending, err := database.Status(gdb, migrations)
		if err != nil {
			return err
		}
		fmt.Printf("%d applied, %d pending\n", len(applied), len(pending))
		for _, a := range applied {
			fmt.Printf("  ✓ %04d_%s  %s\n", a.Version, a.Name, a.AppliedAt.Format(time.RFC3339))
		}
		for _, p := range pending {
			fmt.Printf("  · %04d_%s  pending\n", p.Version, p.Name)
		}
		return nil
	default:
		return errors.New("migrate up | down | status")
	}
}

func serve(gdb *gorm.DB, cfg config.Config, log *slog.Logger, migrations []database.Migration) error {
	// A service that serves against a schema it has not migrated will fail in
	// a handler, hours later, as a 500 nobody can place. Refuse at boot.
	_, pending, err := database.Status(gdb, migrations)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("%d migrasi tertunda — jalankan `mc migrate up` sebelum serve", len(pending))
	}

	deps, err := newDeps(gdb, cfg, log, notify.NewSMTP(cfg, gdb, log))
	if err != nil {
		return err
	}
	handler := httpapi.NewRouter(deps)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	attrs := []any{}
	for k, v := range cfg.Redacted() {
		attrs = append(attrs, slog.String(k, v))
	}
	log.Info("marketing_calendar starting", attrs...)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
