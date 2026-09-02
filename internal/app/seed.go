package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

// Seed loads reference and demo data. It is re-runnable and idempotent, and it
// is NOT a migration: relative dates in a migration are wrong tomorrow (99 §7).
func Seed(ctx context.Context, db *gorm.DB) (int, error) {
	n := 0
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, step := range []func(*gorm.DB) (int, error){
			seedPermissions, seedRoles, seedRolePermissions, seedCompanies,
			seedSites, seedGroups, seedParameters, seedHolidays, seedUsers, seedChains,
			seedTargets, seedPlans, seedTransactions,
		} {
			c, err := step(tx)
			if err != nil {
				return err
			}
			n += c
		}
		return nil
	})
	return n, err
}

// lookupUUID reads a single id.
//
// gorm's Raw(...).Scan(&dest) cannot fill a uuid.UUID: the type is [16]byte,
// so gorm treats it as a []uint8 and tries to parse the 36-character text form
// into one byte. Row().Scan uses database/sql directly, where uuid.UUID's own
// sql.Scanner does the right thing. A missing row returns uuid.Nil rather than
// an error, which is what every caller here wants.
func lookupUUID(tx *gorm.DB, query string, args ...any) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.Raw(query, args...).Row().Scan(&out)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	return out, nil
}

// The permission set of 12-security.md §4.
var permissions = [][2]string{
	{"promo.view", "Melihat rencana promo"},
	{"promo.create", "Membuat rencana promo"},
	{"promo.manage", "Mengubah dan mengajukan rencana promo milik tim"},
	{"target.view", "Melihat target"},
	{"target.manage", "Menetapkan target"},
	{"report.view", "Melihat laporan"},
	{"report.export", "Mengunduh laporan sebagai CSV"},
	{"import.run", "Menjalankan impor transaksi"},
	{"import.view", "Melihat riwayat dan rekonsiliasi impor"},
	{"site.manage", "Mengelola perusahaan, toko dan kelompok toko"},
	{"user.manage", "Mengelola pengguna dan peran"},
	{"settings.manage", "Mengelola parameter sistem dan rantai persetujuan"},
	{"audit.view", "Melihat jejak audit"},
	{"force_release", "Merilis paksa dan menghidupkan kembali rencana"},
}

func seedPermissions(tx *gorm.DB) (int, error) {
	for _, p := range permissions {
		if err := tx.Exec(`
			INSERT INTO permission (permission_code, description) VALUES (?, ?)
			ON CONFLICT (permission_code) DO UPDATE SET description = EXCLUDED.description`,
			p[0], p[1]).Error; err != nil {
			return 0, err
		}
	}
	return len(permissions), nil
}

// The group's real roles (BR-5.7, D28 + CFO).
var roles = [][3]string{
	{"marketing_staff", "Staf Marketing", "Marketing Staff"},
	{"marketing_head", "Kepala Marketing", "Marketing Head"},
	{"business_analyst", "Analis Bisnis", "Business Analyst"},
	{"finance_head", "Kepala Keuangan", "Finance Head"},
	{"operation", "Operasional", "Operation"},
	{"cfo", "CFO", "CFO"},
	{"it", "IT", "IT"},
	{"superadmin", "Superadmin", "Superadmin"},
}

func seedRoles(tx *gorm.DB) (int, error) {
	for _, r := range roles {
		if err := tx.Exec(`
			INSERT INTO role (role_id, role_code, label_id, label_en, is_system)
			VALUES (?, ?, ?, ?, true)
			ON CONFLICT (role_code) DO UPDATE
			   SET label_id = EXCLUDED.label_id, label_en = EXCLUDED.label_en`,
			id.New(), r[0], r[1], r[2]).Error; err != nil {
			return 0, err
		}
	}
	return len(roles), nil
}

// The matrix of 12-security.md §4, after D38 (report.export reaches every
// business role; IT stays denied) and D35 (superadmin is the only
// force_release).
var rolePermissions = map[string][]string{
	"marketing_staff":  {"promo.view", "promo.create", "target.view", "report.view", "report.export"},
	"marketing_head":   {"promo.view", "promo.create", "promo.manage", "target.view", "target.manage", "report.view", "report.export", "import.view"},
	"business_analyst": {"promo.view", "target.view", "report.view", "report.export", "import.view"},
	"finance_head":     {"promo.view", "target.view", "target.manage", "report.view", "report.export", "import.view"},
	"operation":        {"promo.view", "target.view", "report.view", "report.export"},
	"cfo":              {"promo.view", "target.view", "target.manage", "report.view", "report.export", "audit.view"},
	"it":               {"promo.view", "target.view", "report.view", "import.run", "import.view", "site.manage", "user.manage", "settings.manage", "audit.view"},
	"superadmin":       {"promo.view", "promo.create", "promo.manage", "target.view", "target.manage", "report.view", "report.export", "import.run", "import.view", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
}

func seedRolePermissions(tx *gorm.DB) (int, error) {
	n := 0
	for code, perms := range rolePermissions {
		roleID, err := lookupUUID(tx, `SELECT role_id FROM role WHERE role_code = ?`, code)
		if err != nil {
			return 0, err
		}
		if err := tx.Exec(`DELETE FROM role_permission WHERE role_id = ?`, roleID).Error; err != nil {
			return 0, err
		}
		for _, p := range perms {
			if err := tx.Exec(`
				INSERT INTO role_permission (role_id, permission_code) VALUES (?, ?)
				ON CONFLICT DO NOTHING`, roleID, p).Error; err != nil {
				return 0, err
			}
			n++
		}
	}
	return n, nil
}

var companies = [][2]string{
	{"MAXX", "Maxx Coffee"},
	{"RUUMA", "Ruuma"},
	{"SUNSHINE", "Sunshine"},
}

func seedCompanies(tx *gorm.DB) (int, error) {
	for _, c := range companies {
		if err := tx.Exec(`
			INSERT INTO company (company_id, company_code, company_name)
			VALUES (?, ?, ?) ON CONFLICT (company_code) DO NOTHING`,
			id.New(), c[0], c[1]).Error; err != nil {
			return 0, err
		}
	}
	return len(companies), nil
}

type seedSite struct{ company, code, name, siteType string }

var sites = []seedSite{
	{"MAXX", "MXX-001", "Maxx Coffee Plaza Indonesia", "coffee_shop"},
	{"MAXX", "MXX-002", "Maxx Coffee Grand Indonesia", "coffee_shop"},
	{"MAXX", "MXX-003", "Maxx Coffee Kelapa Gading", "coffee_shop"},
	{"MAXX", "MXX-004", "Maxx Coffee Bandung Dago", "coffee_shop"},
	{"RUUMA", "RUM-001", "Ruuma Senopati", "restaurant"},
	{"RUUMA", "RUM-002", "Ruuma PIK Avenue", "restaurant"},
	{"RUUMA", "RUM-003", "Ruuma Surabaya Tunjungan", "restaurant"},
	{"SUNSHINE", "SUN-001", "Sunshine Catering Jakarta Pusat", "catering"},
	{"SUNSHINE", "SUN-002", "Sunshine Catering Tangerang", "catering"},
}

func seedSites(tx *gorm.DB) (int, error) {
	for _, s := range sites {
		companyID, err := lookupUUID(tx, `SELECT company_id FROM company WHERE company_code = ?`, s.company)
		if err != nil {
			return 0, err
		}
		existing, err := lookupUUID(tx, `SELECT site_id FROM site WHERE company_id = ? AND site_code = ?`,
			companyID, s.code)
		if err != nil {
			return 0, err
		}
		if existing != uuid.Nil {
			continue
		}
		siteID := id.New()
		if err := tx.Exec(`
			INSERT INTO site (site_id, company_id, site_code, site_name, site_type)
			VALUES (?, ?, ?, ?, ?)`, siteID, companyID, s.code, s.name, s.siteType).Error; err != nil {
			return 0, err
		}
		// The system group, in the same transaction as the site (BR-1.3).
		groupID := id.New()
		if err := tx.Exec(`
			INSERT INTO site_group (site_group_id, company_id, site_group_name, is_system, auto_for_site_id)
			VALUES (?, ?, ?, true, ?)`, groupID, companyID, s.name, siteID).Error; err != nil {
			return 0, err
		}
		if err := tx.Exec(`
			INSERT INTO site_group_member (site_group_id, site_id, company_id)
			VALUES (?, ?, ?)`, groupID, siteID, companyID).Error; err != nil {
			return 0, err
		}
	}
	return len(sites), nil
}

// Two hand-made multi-site groups, both SINGLE-BRAND. A cross-brand group
// cannot be seeded because the database refuses it (D31).
func seedGroups(tx *gorm.DB) (int, error) {
	type g struct {
		company, name string
		siteCodes     []string
	}
	groups := []g{
		{"MAXX", "Maxx Jakarta", []string{"MXX-001", "MXX-002", "MXX-003"}},
		{"RUUMA", "Ruuma Jabodetabek", []string{"RUM-001", "RUM-002"}},
	}
	for _, grp := range groups {
		companyID, err := lookupUUID(tx, `SELECT company_id FROM company WHERE company_code = ?`, grp.company)
		if err != nil {
			return 0, err
		}
		existing, err := lookupUUID(tx,
			`SELECT site_group_id FROM site_group WHERE company_id = ? AND site_group_name = ? AND NOT is_system`,
			companyID, grp.name)
		if err != nil {
			return 0, err
		}
		if existing != uuid.Nil {
			continue
		}
		groupID := id.New()
		if err := tx.Exec(`
			INSERT INTO site_group (site_group_id, company_id, site_group_name, is_system)
			VALUES (?, ?, ?, false)`, groupID, companyID, grp.name).Error; err != nil {
			return 0, err
		}
		for _, code := range grp.siteCodes {
			siteID, err := lookupUUID(tx, `SELECT site_id FROM site WHERE company_id = ? AND site_code = ?`,
				companyID, code)
			if err != nil {
				return 0, err
			}
			if err := tx.Exec(`
				INSERT INTO site_group_member (site_group_id, site_id, company_id)
				VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, groupID, siteID, companyID).Error; err != nil {
				return 0, err
			}
		}
	}
	return len(groups), nil
}

type seedParam struct {
	key, value, vtype, desc string
	secret                  bool
}

func seedParameters(tx *gorm.DB) (int, error) {
	params := []seedParam{
		{ParamLeadTimeDays, "7", "int", "Masa tenggang minimum promo, dalam hari kerja (BR-3.3)", false},
		{ParamAutoCancelDays, "5", "int", "Hari sebelum mulai saat rencana tanpa persetujuan lengkap dibatalkan (BR-4.6)", false},
		{ParamOverlapWarning, "true", "bool", "Aktifkan peringatan promo tumpang tindih (BR-3.6)", false},
		{ParamReleaseRecipients, "marketing@sfg.local,operasional@sfg.local", "list",
			"Daftar tetap penerima surel rilis. Ini adalah SATU-SATUNYA penerima; aktor rantai tidak ditambahkan otomatis (D32)", false},
		{ParamImportDropPath, "/srv/marketing_calendar/import", "string", "Direktori berkas impor nightly", false},
		{ParamMaxExportRows, "100000", "int", "Batas baris satu ekspor CSV", false},
		{ParamLockoutThreshold, "5", "int", "Jumlah kegagalan masuk sebelum akun terkunci", false},
		{ParamLockoutMinutes, "15", "int", "Lama penguncian akun, dalam menit", false},
	}
	for _, p := range params {
		if err := tx.Exec(`
			INSERT INTO sys_parameters (param_key, param_value, value_type, description, is_secret)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (param_key) DO UPDATE SET description = EXCLUDED.description`,
			p.key, p.value, p.vtype, p.desc, p.secret).Error; err != nil {
			return 0, err
		}
	}
	return len(params), nil
}

// Indonesian public holidays for the current and next year (D22).
//
// These are the fixed-date and announced dates for 2026 and 2027. Idul Fitri
// and the other lunar dates move each year and are the reason this table is
// administrator-maintained rather than computed.
func seedHolidays(tx *gorm.DB) (int, error) {
	holidays := map[string]string{
		"2026-01-01": "Tahun Baru Masehi",
		"2026-01-17": "Isra Mikraj Nabi Muhammad SAW",
		"2026-02-17": "Tahun Baru Imlek 2577",
		"2026-03-19": "Hari Suci Nyepi Tahun Baru Saka 1948",
		"2026-03-20": "Idul Fitri 1447 H",
		"2026-03-21": "Idul Fitri 1447 H",
		"2026-03-23": "Cuti Bersama Idul Fitri",
		"2026-03-24": "Cuti Bersama Idul Fitri",
		"2026-04-03": "Wafat Isa Almasih",
		"2026-05-01": "Hari Buruh Internasional",
		"2026-05-14": "Kenaikan Isa Almasih",
		"2026-05-27": "Idul Adha 1447 H",
		"2026-05-31": "Hari Raya Waisak 2570",
		"2026-06-01": "Hari Lahir Pancasila",
		"2026-06-16": "Tahun Baru Islam 1448 H",
		"2026-08-17": "Hari Kemerdekaan Republik Indonesia",
		"2026-08-25": "Maulid Nabi Muhammad SAW",
		"2026-12-25": "Hari Raya Natal",
		"2027-01-01": "Tahun Baru Masehi",
		"2027-02-06": "Tahun Baru Imlek 2578",
		"2027-03-09": "Idul Fitri 1448 H",
		"2027-03-10": "Idul Fitri 1448 H",
		"2027-03-11": "Cuti Bersama Idul Fitri",
		"2027-03-12": "Cuti Bersama Idul Fitri",
		"2027-03-26": "Wafat Isa Almasih",
		"2027-04-08": "Hari Suci Nyepi Tahun Baru Saka 1949",
		"2027-05-01": "Hari Buruh Internasional",
		"2027-05-06": "Kenaikan Isa Almasih",
		"2027-05-16": "Idul Adha 1448 H",
		"2027-05-20": "Hari Raya Waisak 2571",
		"2027-06-01": "Hari Lahir Pancasila",
		"2027-06-06": "Tahun Baru Islam 1449 H",
		"2027-08-15": "Maulid Nabi Muhammad SAW",
		"2027-08-17": "Hari Kemerdekaan Republik Indonesia",
		"2027-12-25": "Hari Raya Natal",
	}
	for d, name := range holidays {
		day, err := calendar.ParseDate(d)
		if err != nil {
			return 0, err
		}
		if err := tx.Exec(`
			INSERT INTO holiday (holiday_id, holiday_date, holiday_name, country)
			VALUES (?, ?, ?, 'ID')
			ON CONFLICT (country, holiday_date) DO UPDATE SET holiday_name = EXCLUDED.holiday_name`,
			id.New(), day, name).Error; err != nil {
			return 0, err
		}
	}
	return len(holidays), nil
}

type seedUser struct {
	email, name, role string
	companies         []string
}

// SeedPassword is the demo password. It is printed by `mc seed` and must be
// changed before the system carries real data; the deployment handbook says so
// in the same breath as it says to run the seed.
const SeedPassword = "MarketingCalendar2026!"

func seedUsers(tx *gorm.DB) (int, error) {
	users := []seedUser{
		{"rina.hartono@sfg.local", "Rina Hartono", "marketing_staff", []string{"MAXX"}},
		{"budi.santoso@sfg.local", "Budi Santoso", "marketing_head", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		{"sari.dewi@sfg.local", "Sari Dewi", "business_analyst", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		{"agus.pratama@sfg.local", "Agus Pratama", "finance_head", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		// Operation is seeded TWICE on purpose: once scoped to a single brand
		// and once across all three, so BR-4.4a's company-scoped eligibility is
		// exercised by the seed and not only by a test.
		{"dedi.kurniawan@sfg.local", "Dedi Kurniawan", "operation", []string{"MAXX"}},
		{"lina.wijaya@sfg.local", "Lina Wijaya", "operation", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		{"hendra.gunawan@sfg.local", "Hendra Gunawan", "cfo", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		{"it.support@sfg.local", "IT Support", "it", []string{"MAXX", "RUUMA", "SUNSHINE"}},
		// D35: superadmin is held by IT.
		{"it.admin@sfg.local", "IT Admin", "superadmin", []string{"MAXX", "RUUMA", "SUNSHINE"}},
	}
	hash, err := security.HashPassword(SeedPassword)
	if err != nil {
		return 0, err
	}
	for _, u := range users {
		userID, err := lookupUUID(tx, `SELECT user_id FROM app_user WHERE lower(email) = lower(?)`, u.email)
		if err != nil {
			return 0, err
		}
		if userID == uuid.Nil {
			userID = id.New()
			if err := tx.Exec(`
				INSERT INTO app_user (user_id, email, full_name, password_hash)
				VALUES (?, ?, ?, ?)`, userID, u.email, u.name, hash).Error; err != nil {
				return 0, err
			}
		}
		roleID, err := lookupUUID(tx, `SELECT role_id FROM role WHERE role_code = ?`, u.role)
		if err != nil {
			return 0, err
		}
		for _, c := range u.companies {
			companyID, err := lookupUUID(tx, `SELECT company_id FROM company WHERE company_code = ?`, c)
			if err != nil {
				return 0, err
			}
			if err := tx.Exec(`
				INSERT INTO user_role (user_id, role_id, company_id) VALUES (?, ?, ?)
				ON CONFLICT DO NOTHING`, userID, roleID, companyID).Error; err != nil {
				return 0, err
			}
		}
	}
	return len(users), nil
}

// The default chain of BR-4.2 after D36: five steps, per company.
func seedChains(tx *gorm.DB) (int, error) {
	steps := []struct {
		name string
		role string
	}{
		{"Marketing Head", "marketing_head"},
		{"Business Analyst", "business_analyst"},
		{"Finance Head", "finance_head"},
		{"Operation", "operation"},
		{"CFO", "cfo"},
	}
	rows, err := tx.Raw(`SELECT company_id FROM company`).Rows()
	if err != nil {
		return 0, err
	}
	var companyIDs []uuid.UUID
	for rows.Next() {
		var c uuid.UUID
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return 0, err
		}
		companyIDs = append(companyIDs, c)
	}
	rows.Close()

	n := 0
	for _, companyID := range companyIDs {
		chainID, err := lookupUUID(tx, `SELECT chain_id FROM approval_chain
		         WHERE company_id = ? AND subject_type = 'promotion_plan' AND is_active`,
			companyID)
		if err != nil {
			return 0, err
		}
		if chainID != uuid.Nil {
			continue // already configured; the seed never rewrites a live chain
		}
		chainID = id.New()
		if err := tx.Exec(`
			INSERT INTO approval_chain (chain_id, company_id, subject_type, chain_name)
			VALUES (?, ?, 'promotion_plan', 'Rantai persetujuan promo')`,
			chainID, companyID).Error; err != nil {
			return 0, err
		}
		versionID := id.New()
		if err := tx.Exec(`
			INSERT INTO approval_chain_version (version_id, chain_id, version_no)
			VALUES (?, ?, 1)`, versionID, chainID).Error; err != nil {
			return 0, err
		}
		for i, s := range steps {
			stepID := id.New()
			if err := tx.Exec(`
				INSERT INTO approval_step (step_id, version_id, step_no, step_name, satisfaction)
				VALUES (?, ?, ?, ?, 'ANY_OF')`, stepID, versionID, i+1, s.name).Error; err != nil {
				return 0, err
			}
			roleID, err := lookupUUID(tx, `SELECT role_id FROM role WHERE role_code = ?`, s.role)
			if err != nil {
				return 0, err
			}
			if err := tx.Exec(`
				INSERT INTO approval_step_role (step_id, role_id) VALUES (?, ?)`,
				stepID, roleID).Error; err != nil {
				return 0, err
			}
			n++
		}
	}
	return n, nil
}

func seedTargets(tx *gorm.DB) (int, error) {
	year := calendar.Today(time.Now()).Year()
	rows, err := tx.Raw(`SELECT site_id, company_id, site_type FROM site`).Rows()
	if err != nil {
		return 0, err
	}
	type s struct {
		siteID, companyID uuid.UUID
		siteType          string
	}
	var all []s
	for rows.Next() {
		var x s
		if err := rows.Scan(&x.siteID, &x.companyID, &x.siteType); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, x)
	}
	rows.Close()

	n := 0
	for _, x := range all {
		// Whole rupiah, integer arithmetic. Coffee shops turn over less per
		// site than restaurants; catering is lumpier and larger.
		var monthly int64
		switch x.siteType {
		case "coffee_shop":
			monthly = 850_000_000
		case "restaurant":
			monthly = 1_400_000_000
		default:
			monthly = 2_100_000_000
		}
		// The YEAR target is deliberately NOT twelve times the month target.
		// BR-2.3 says they need not agree, and a seed where they do agree
		// hides the variance display that the rule exists to produce.
		yearTarget := monthly * 12 * 95 / 100
		if err := upsertTarget(tx, x.companyID, x.siteID, "YEAR", year, 0, "normal", yearTarget); err != nil {
			return 0, err
		}
		if err := upsertTarget(tx, x.companyID, x.siteID, "YEAR", year, 0, "promo", yearTarget/10); err != nil {
			return 0, err
		}
		n += 2
		for m := 1; m <= 12; m++ {
			if err := upsertTarget(tx, x.companyID, x.siteID, "MONTH", year, m, "normal", monthly); err != nil {
				return 0, err
			}
			if err := upsertTarget(tx, x.companyID, x.siteID, "MONTH", year, m, "promo", monthly/10); err != nil {
				return 0, err
			}
			n += 2
		}
	}
	return n, nil
}

func upsertTarget(tx *gorm.DB, companyID, siteID uuid.UUID, kind string, year, month int, salesType string, amount int64) error {
	var m any
	if kind == "MONTH" {
		m = month
	}
	return tx.Exec(`
		INSERT INTO sales_target (target_id, company_id, site_id, period_kind, period_year,
		                          period_month, sales_type, target_amount_idr)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (site_id, period_kind, period_year, COALESCE(period_month, 0), sales_type)
		DO NOTHING`, id.New(), companyID, siteID, kind, year, m, salesType, amount).Error
}

func seedPlans(tx *gorm.DB) (int, error) {
	var existing int64
	tx.Raw(`SELECT count(*) FROM promotion_plan`).Scan(&existing)
	if existing > 0 {
		return 0, nil
	}
	today := calendar.Today(time.Now())
	year := today.Year()

	type plan struct {
		companyCode, groupName, name, mode, rule string
		startOffset, endOffset                   int
		status                                   string
		sales                                    int64
		receipts                                 int
		stepsApproved                            int
	}
	plans := []plan{
		{"MAXX", "Maxx Jakarta", "Kopi Sore Hemat", "dine_in",
			"Diskon 20% untuk gelas kedua, setiap hari 15.00-18.00.", 14, 44, "RELEASED", 250_000_000, 4_000, 5},
		{"MAXX", "Maxx Coffee Plaza Indonesia", "Promo Pagi Karyawan", "take_away",
			"Beli 2 gratis 1 untuk pembelian sebelum jam 09.00.", 21, 51, "PENDING", 90_000_000, 1_800, 2},
		{"RUUMA", "Ruuma Jabodetabek", "Makan Malam Berdua", "dine_in",
			"Paket berdua Rp 250.000 termasuk dua hidangan utama dan satu pencuci mulut.", 30, 60, "PENDING", 400_000_000, 1_200, 1},
		{"SUNSHINE", "Sunshine Catering Jakarta Pusat", "Paket Rapat Kantor", "take_away",
			"Paket rapat minimum 20 porsi, harga khusus korporat.", 10, 40, "DRAFT", 300_000_000, 60, 0},
		{"RUUMA", "Ruuma Senopati", "Akhir Pekan Keluarga", "dine_in",
			"Anak di bawah 12 tahun makan gratis setiap Sabtu dan Minggu.", -20, 10, "REJECTED", 180_000_000, 900, 0},
	}

	creator, err := lookupUUID(tx, `SELECT user_id FROM app_user WHERE email = 'rina.hartono@sfg.local'`)
	if err != nil {
		return 0, err
	}

	for i, p := range plans {
		companyID, err := lookupUUID(tx, `SELECT company_id FROM company WHERE company_code = ?`, p.companyCode)
		if err != nil {
			return 0, err
		}
		groupID, err := lookupUUID(tx,
			`SELECT site_group_id FROM site_group WHERE company_id = ? AND site_group_name = ?`,
			companyID, p.groupName)
		if err != nil {
			return 0, err
		}
		if groupID == uuid.Nil {
			continue
		}
		planID, versionID := id.New(), id.New()
		code := fmt.Sprintf("P-%d-%04d", year, i+1)
		if err := tx.Exec(`
			INSERT INTO promotion_plan (plan_id, company_id, plan_code, site_group_id, status, created_by)
			VALUES (?, ?, ?, ?, 'DRAFT', ?)`,
			planID, companyID, code, groupID, creator).Error; err != nil {
			return 0, err
		}
		if err := tx.Exec(`
			INSERT INTO promotion_plan_version (version_id, plan_id, version_no, promo_name,
			    start_date, end_date, target_sales_idr, target_receipt_count, order_mode,
			    promo_rule, created_by)
			VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
			versionID, planID, p.name,
			today.AddDate(0, 0, p.startOffset), today.AddDate(0, 0, p.endOffset),
			p.sales, p.receipts, p.mode, p.rule, creator).Error; err != nil {
			return 0, err
		}
		if err := tx.Exec(`UPDATE promotion_plan SET current_version_id = ? WHERE plan_id = ?`,
			versionID, planID).Error; err != nil {
			return 0, err
		}
		if p.status == "DRAFT" {
			continue
		}
		if err := seedApproval(tx, companyID, planID, creator, p.status, p.stepsApproved); err != nil {
			return 0, err
		}
	}
	return len(plans), nil
}

// seedApproval walks a plan through the real chain so the demo data has a
// genuine approval history rather than a status set by hand.
func seedApproval(tx *gorm.DB, companyID, planID, creator uuid.UUID, status string, approvedSteps int) error {
	versionID, err := lookupUUID(tx, `
		SELECT cv.version_id FROM approval_chain c
		  JOIN approval_chain_version cv ON cv.chain_id = c.chain_id
		 WHERE c.company_id = ? AND c.subject_type = 'promotion_plan' AND c.is_active
		 ORDER BY cv.version_no DESC LIMIT 1`, companyID)
	if err != nil {
		return err
	}
	instanceID := id.New()
	currentStep := approvedSteps + 1
	instanceStatus := "PENDING"
	var closedAt any
	if status == "RELEASED" {
		instanceStatus = "APPROVED"
		currentStep = approvedSteps
		closedAt = time.Now().UTC()
	} else if status == "REJECTED" {
		instanceStatus = "REJECTED"
		currentStep = 1
		closedAt = time.Now().UTC()
	}
	if err := tx.Exec(`
		INSERT INTO approval_instance (instance_id, version_id, company_id, subject_type,
		    subject_id, created_by, current_step_no, status, closed_at)
		VALUES (?, ?, ?, 'promotion_plan', ?, ?, ?, ?, ?)`,
		instanceID, versionID, companyID, planID, creator, currentStep, instanceStatus, closedAt).Error; err != nil {
		return err
	}

	for step := 1; step <= approvedSteps; step++ {
		roleID, err := lookupUUID(tx, `
			SELECT sr.role_id FROM approval_step s JOIN approval_step_role sr ON sr.step_id = s.step_id
			 WHERE s.version_id = ? AND s.step_no = ? LIMIT 1`, versionID, step)
		if err != nil {
			return err
		}
		approver, err := lookupUUID(tx, `
			SELECT ur.user_id FROM user_role ur
			 WHERE ur.role_id = ? AND ur.company_id = ? AND ur.user_id <> ? LIMIT 1`,
			roleID, companyID, creator)
		if err != nil {
			return err
		}
		if approver == uuid.Nil {
			continue
		}
		// The interval is built as a full string rather than concatenated in
		// SQL: passing an int into `(? || ' hours')` leaves the driver with no
		// type to infer, and it refuses rather than guessing.
		ago := fmt.Sprintf("%d hours", (approvedSteps-step+1)*6)
		if err := tx.Exec(`
			INSERT INTO approval_event (event_id, instance_id, step_no, action, actor_id, occurred_at)
			VALUES (?, ?, ?, 'APPROVE', ?, now() - ?::interval)`,
			id.New(), instanceID, step, approver, ago).Error; err != nil {
			return err
		}
	}

	if status == "REJECTED" {
		approver, err := lookupUUID(tx, `SELECT user_id FROM app_user WHERE email = 'budi.santoso@sfg.local'`)
		if err != nil {
			return err
		}
		if approver != uuid.Nil {
			if err := tx.Exec(`
				INSERT INTO approval_event (event_id, instance_id, step_no, action, actor_id, reason)
				VALUES (?, ?, 1, 'REJECT', ?, ?)`,
				id.New(), instanceID, approver,
				"Margin terlalu tipis untuk akhir pekan panjang; ajukan ulang dengan diskon maksimal 15%.").Error; err != nil {
				return err
			}
		}
	}

	return tx.Exec(`
		UPDATE promotion_plan SET status = ?, approval_instance_id = ? WHERE plan_id = ?`,
		status, instanceID, planID).Error
}

// seedTransactions generates enough history for the promotion report to be
// non-trivial. Amounts are integer rupiah throughout.
func seedTransactions(tx *gorm.DB) (int, error) {
	var existing int64
	tx.Raw(`SELECT count(*) FROM history_txn`).Scan(&existing)
	if existing > 0 {
		return 0, nil
	}
	runID := id.New()
	if err := tx.Exec(`
		INSERT INTO import_run (import_run_id, file_name, file_checksum, rows_read,
		    rows_inserted, outcome, message, finished_at)
		VALUES (?, 'seed', ?, 0, 0, 'OK', 'data contoh dari perintah seed', now())`,
		runID, "seed-"+id.NewString()).Error; err != nil {
		return 0, err
	}

	rows, err := tx.Raw(`SELECT site_id, company_id, site_type FROM site`).Rows()
	if err != nil {
		return 0, err
	}
	type s struct {
		siteID, companyID uuid.UUID
		siteType          string
	}
	var all []s
	for rows.Next() {
		var x s
		if err := rows.Scan(&x.siteID, &x.companyID, &x.siteType); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, x)
	}
	rows.Close()

	// The released plan, so its actuals are attributable by promo_id (BR-7.6).
	var releasedPlan, releasedGroup uuid.UUID
	if err := tx.Raw(`SELECT plan_id, site_group_id FROM promotion_plan WHERE status = 'RELEASED' LIMIT 1`).
		Row().Scan(&releasedPlan, &releasedGroup); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	promoSites := map[uuid.UUID]bool{}
	if releasedGroup != uuid.Nil {
		m, _ := tx.Raw(`SELECT site_id FROM site_group_member WHERE site_group_id = ?`, releasedGroup).Rows()
		if m != nil {
			for m.Next() {
				var sid uuid.UUID
				if err := m.Scan(&sid); err == nil {
					promoSites[sid] = true
				}
			}
			m.Close()
		}
	}

	today := calendar.Today(time.Now())
	n := 0
	seq := 0
	for _, x := range all {
		var ticket int64
		switch x.siteType {
		case "coffee_shop":
			ticket = 48_000
		case "restaurant":
			ticket = 185_000
		default:
			ticket = 1_250_000
		}
		// 60 days of history, a handful of receipts a day. Enough to make the
		// report non-trivial without making the seed slow.
		for day := 60; day >= 0; day-- {
			d := today.AddDate(0, 0, -day)
			receipts := 6
			if x.siteType == "catering" {
				receipts = 2
			}
			for i := 0; i < receipts; i++ {
				seq++
				// Deterministic variation without floats: the amount moves
				// with the day and the receipt index.
				amount := ticket + int64((day*7+i*13)%25)*1_000
				salesType := "normal"
				var promoID any
				mode := "dine_in"
				if i%3 == 0 {
					mode = "take_away"
				}
				if releasedPlan != uuid.Nil && promoSites[x.siteID] && day < 20 && i%2 == 0 {
					salesType = "promo"
					promoID = releasedPlan
				}
				if err := tx.Exec(`
					INSERT INTO history_txn (txn_id, company_id, site_id, business_date,
					    pos_receipt_no, sales_type, promo_id, order_mode, gross_amount_idr, import_run_id)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT DO NOTHING`,
					id.New(), x.companyID, x.siteID, d,
					fmt.Sprintf("R-%08d", seq), salesType, promoID, mode, amount, runID).Error; err != nil {
					return 0, err
				}
				n++
			}
		}
	}
	return n, nil
}
