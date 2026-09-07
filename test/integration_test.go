// Package test holds the integration, concurrency and authorisation tests.
//
// They run against a REAL PostgreSQL — marketing_calendar_test — because the
// invariants under test are database invariants. A mock cannot refuse a
// cross-brand foreign key or serialise two approvers with SELECT ... FOR
// UPDATE, so a mock cannot prove any of this.
package test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	mcdb "github.com/stevenwilliam/marketing_calendar/db"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/postgres"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/database"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

var (
	testDB   *gorm.DB
	setupErr error
	once     sync.Once
)

// db returns the test database, skipping the whole suite when it is not
// configured. A skip is honest; a pass against no database is not.
func db(t *testing.T) *gorm.DB {
	t.Helper()
	once.Do(func() {
		dsn := os.Getenv("MC_TEST_DATABASE_URL")
		if dsn == "" {
			return
		}
		testDB, setupErr = database.Open(dsn, false)
		if setupErr != nil {
			return
		}
		migrations, err := database.Load(mcdb.Migrations, "migrations")
		if err != nil {
			setupErr = err
			return
		}
		if _, err := database.Up(testDB, migrations); err != nil {
			setupErr = err
			return
		}
		if _, err := app.Seed(context.Background(), testDB); err != nil {
			setupErr = err
		}
	})
	if testDB == nil && setupErr == nil {
		t.Skip("MC_TEST_DATABASE_URL is not set; integration tests need a real PostgreSQL")
	}
	if setupErr != nil {
		t.Fatalf("test database setup failed: %v", setupErr)
	}
	return testDB
}

func mustUUID(t *testing.T, g *gorm.DB, query string, args ...any) uuid.UUID {
	t.Helper()
	var out uuid.UUID
	if err := g.Raw(query, args...).Row().Scan(&out); err != nil {
		t.Fatalf("lookup failed: %v (%s)", err, query)
	}
	return out
}

// --- BR-1.3 / D31 --------------------------------------------------------

// The rule is enforced by the DATABASE, so the test writes raw SQL and
// bypasses the application entirely. A rule only the application enforces is a
// rule the next writer can skip.
func TestCrossBrandGroupMemberRefusedByDatabase(t *testing.T) {
	g := db(t)
	maxxGroup := mustUUID(t, g, `SELECT site_group_id FROM site_group
	    WHERE site_group_name = 'Maxx Jakarta' AND NOT is_system LIMIT 1`)
	maxxCompany := mustUUID(t, g, `SELECT company_id FROM site_group WHERE site_group_id = ?`, maxxGroup)
	ruumaSite := mustUUID(t, g, `SELECT s.site_id FROM site s JOIN company c ON c.company_id = s.company_id
	    WHERE c.company_code = 'RUUMA' LIMIT 1`)

	err := g.Exec(`INSERT INTO site_group_member (site_group_id, site_id, company_id)
	               VALUES (?, ?, ?)`, maxxGroup, ruumaSite, maxxCompany).Error
	if err == nil {
		_ = g.Exec(`DELETE FROM site_group_member WHERE site_group_id = ? AND site_id = ?`,
			maxxGroup, ruumaSite).Error
		t.Fatal("a Ruuma site was accepted into a Maxx Coffee group: the composite FK is not enforcing D31")
	}
	t.Logf("refused as designed: %v", err)
}

// --- BR-8.1 append-only --------------------------------------------------

func TestAppendOnlyRefusesUpdateDeleteAndTruncate(t *testing.T) {
	g := db(t)
	probe := id.New()
	if err := g.Exec(`INSERT INTO audit_log (audit_id, action, subject_type)
	                  VALUES (?, 'test.append_only', 'probe')`, probe).Error; err != nil {
		t.Fatal(err)
	}
	// The row must EXIST before the trigger has anything to fire on. Probing
	// an empty table is how this check silently stopped checking once before.
	var n int64
	g.Raw(`SELECT count(*) FROM audit_log WHERE audit_id = ?`, probe).Scan(&n)
	if n != 1 {
		t.Fatal("the probe row was not inserted; the rest of this test would pass vacuously")
	}

	if err := g.Exec(`UPDATE audit_log SET action = 'tampered' WHERE audit_id = ?`, probe).Error; err == nil {
		t.Fatal("audit_log accepted an UPDATE")
	}
	if err := g.Exec(`DELETE FROM audit_log WHERE audit_id = ?`, probe).Error; err == nil {
		t.Fatal("audit_log accepted a DELETE")
	}
	// TRUNCATE is statement-level and never reaches a FOR EACH ROW trigger.
	// This is the hole migration 0008 closed.
	if err := g.Exec(`TRUNCATE audit_log`).Error; err == nil {
		t.Fatal("audit_log accepted a TRUNCATE: the append-only guarantee is not one")
	}
	for _, table := range []string{"approval_event", "import_run", "import_rejection"} {
		if err := g.Exec(`TRUNCATE ` + table).Error; err == nil {
			t.Fatalf("%s accepted a TRUNCATE", table)
		}
	}
}

// --- BR-4.10 concurrency -------------------------------------------------

// Two approvers acting at the same instant must not both succeed. This is the
// test CLAUDE.md requires: concurrency is tested, not assumed.
func TestTwoApproversRaceExactlyOneWins(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	repo := postgres.NewApprovalRepo(g)

	companyID := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	chain, _, err := repo.ActiveChain(ctx, companyID, approval.SubjectPromotionPlan)
	if err != nil {
		t.Fatal(err)
	}
	creator := mustUUID(t, g, `SELECT user_id FROM app_user WHERE email = 'rina.hartono@sfg.local'`)

	// Two DIFFERENT users who both hold the step-1 role in this company, so
	// the race is between two legitimate approvers rather than one user twice.
	step1Role := chain.Steps[0].RoleIDs[0]
	a := mustUUID(t, g, `SELECT user_id FROM user_role WHERE role_id = ? AND company_id = ? LIMIT 1`,
		step1Role, companyID)
	b := createApprover(t, g, step1Role, companyID)

	instanceID, err := repo.OpenInstance(ctx, approval.Instance{
		VersionID: chain.VersionID, CompanyID: companyID,
		SubjectType: approval.SubjectPromotionPlan, SubjectID: id.New(),
		CreatedBy: creator})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, actorID := range []uuid.UUID{a, b} {
		wg.Add(1)
		go func(uid uuid.UUID) {
			defer wg.Done()
			<-start // release both goroutines at the same instant
			_, err := repo.Decide(ctx, instanceID, "", func(in *approval.Instance, c approval.Chain) (approval.Decision, error) {
				return in.Approve(c, approval.Actor{UserID: uid,
					RoleIDs: []uuid.UUID{step1Role}, CompanyIDs: []uuid.UUID{companyID}},
					time.Now().UTC())
			})
			results <- err
		}(actorID)
	}
	close(start)
	wg.Wait()
	close(results)

	var succeeded int
	for err := range results {
		if err == nil {
			succeeded++
		} else {
			t.Logf("one approver was refused, as designed: %v", err)
		}
	}
	// ANY_OF: the first approval satisfies the step and advances it. The
	// second arrives at a step that has already moved and must be refused.
	if succeeded != 1 {
		t.Fatalf("%d of 2 concurrent approvals succeeded; exactly 1 must", succeeded)
	}

	var stepNo int
	var status string
	if err := g.Raw(`SELECT current_step_no, status FROM approval_instance WHERE instance_id = ?`,
		instanceID).Row().Scan(&stepNo, &status); err != nil {
		t.Fatal(err)
	}
	if stepNo != 2 {
		t.Fatalf("the chain advanced to step %d; one approval must advance it exactly one step", stepNo)
	}

	var events int64
	g.Raw(`SELECT count(*) FROM approval_event WHERE instance_id = ? AND step_no = 1
	        AND action = 'APPROVE'`, instanceID).Scan(&events)
	if events != 1 {
		t.Fatalf("%d approval events on step 1; the race wrote more than one", events)
	}
}

// The same approver twice must be refused even without concurrency (BR-4.10).
func TestSameApproverTwiceIsRefused(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	repo := postgres.NewApprovalRepo(g)

	companyID := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	chain, _, err := repo.ActiveChain(ctx, companyID, approval.SubjectPromotionPlan)
	if err != nil {
		t.Fatal(err)
	}
	creator := mustUUID(t, g, `SELECT user_id FROM app_user WHERE email = 'rina.hartono@sfg.local'`)
	role := chain.Steps[0].RoleIDs[0]
	actorID := createApprover(t, g, role, companyID)

	instanceID, err := repo.OpenInstance(ctx, approval.Instance{
		VersionID: chain.VersionID, CompanyID: companyID,
		SubjectType: approval.SubjectPromotionPlan, SubjectID: id.New(), CreatedBy: creator})
	if err != nil {
		t.Fatal(err)
	}
	act := approval.Actor{UserID: actorID, RoleIDs: []uuid.UUID{role}, CompanyIDs: []uuid.UUID{companyID}}
	decide := func() error {
		_, err := repo.Decide(ctx, instanceID, "", func(in *approval.Instance, c approval.Chain) (approval.Decision, error) {
			return in.Approve(c, act, time.Now().UTC())
		})
		return err
	}
	if err := decide(); err != nil {
		t.Fatalf("the first approval must succeed: %v", err)
	}
	// The instance is now on step 2, where this actor holds no role.
	if err := decide(); err == nil {
		t.Fatal("the same approver approved twice")
	}
}

// --- BR-4.3 chain versioning ---------------------------------------------

// An administrator editing the chain must not disturb a plan already in
// flight. The instance binds to a VERSION, not to a chain.
func TestChainChangeDoesNotAffectInFlight(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	repo := postgres.NewApprovalRepo(g)

	companyID := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'SUNSHINE'`)
	chain, _, err := repo.ActiveChain(ctx, companyID, approval.SubjectPromotionPlan)
	if err != nil {
		t.Fatal(err)
	}
	creator := mustUUID(t, g, `SELECT user_id FROM app_user WHERE email = 'rina.hartono@sfg.local'`)
	instanceID, err := repo.OpenInstance(ctx, approval.Instance{
		VersionID: chain.VersionID, CompanyID: companyID,
		SubjectType: approval.SubjectPromotionPlan, SubjectID: id.New(), CreatedBy: creator})
	if err != nil {
		t.Fatal(err)
	}
	stepsBefore := len(chain.Steps)

	// Rewrite the chain to a single step with a different role.
	cfoRole := mustUUID(t, g, `SELECT role_id FROM role WHERE role_code = 'cfo'`)
	admin := mustUUID(t, g, `SELECT user_id FROM app_user WHERE email = 'it.admin@sfg.local'`)
	newVersionNo, err := repo.SaveChain(ctx, companyID, approval.SubjectPromotionPlan,
		[]app.ChainStepRow{{StepNo: 1, StepName: "Hanya CFO",
			Satisfaction: approval.AnyOf, RoleIDs: []uuid.UUID{cfoRole}}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if newVersionNo < 2 {
		t.Fatalf("SaveChain must create a NEW version, got version_no %d", newVersionNo)
	}

	// The in-flight instance still points at the old version, with its steps.
	in, err := repo.InstanceByID(ctx, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	if in.VersionID != chain.VersionID {
		t.Fatal("the in-flight instance was rebound to the new chain version")
	}
	bound, err := repo.ChainVersion(ctx, in.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bound.Steps) != stepsBefore {
		t.Fatalf("the bound version now has %d steps, was %d: the edit reached a plan in flight",
			len(bound.Steps), stepsBefore)
	}

	// And a NEW instance gets the new chain.
	fresh, _, err := repo.ActiveChain(ctx, companyID, approval.SubjectPromotionPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Steps) != 1 {
		t.Fatalf("a new instance must get the new chain: %d steps", len(fresh.Steps))
	}
}

// --- BR-6.3 importer idempotency -----------------------------------------

func TestReimportIsIdempotent(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	facts := postgres.NewFactRepo(g)

	site := mustUUID(t, g, `SELECT site_id FROM site WHERE site_code = 'MXX-001'`)
	company := mustUUID(t, g, `SELECT company_id FROM site WHERE site_id = ?`, site)
	day, _ := calendar.ParseDate("2025-01-15")

	rows := []app.TxnRow{
		{CompanyID: company, SiteID: site, BusinessDate: day, ReceiptNo: "IDEMP-1",
			SalesType: "normal", OrderMode: "dine_in", GrossIDR: 50_000},
		{CompanyID: company, SiteID: site, BusinessDate: day, ReceiptNo: "IDEMP-2",
			SalesType: "normal", OrderMode: "take_away", GrossIDR: 70_000},
	}
	run := app.ImportRun{FileName: "idempotency.csv", Checksum: "sum-" + id.NewString(),
		RowsRead: 2, Outcome: "OK"}

	first, err := facts.LoadFile(ctx, run, rows, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.RowsInserted != 2 {
		t.Fatalf("first load inserted %d rows, want 2", first.RowsInserted)
	}

	// The same file again, by checksum: refused before parsing.
	seen, err := facts.SeenChecksum(ctx, run.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("the checksum of a loaded file must be remembered")
	}

	// A DIFFERENT file covering the same site-day replaces it rather than
	// doubling the revenue (BR-6.3).
	corrected := app.ImportRun{FileName: "idempotency-corrected.csv",
		Checksum: "sum-" + id.NewString(), RowsRead: 2, Outcome: "OK"}
	rows[0].GrossIDR = 55_000
	second, err := facts.LoadFile(ctx, corrected, rows, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.RowsInserted != 2 {
		t.Fatalf("the corrected file inserted %d rows, want 2", second.RowsInserted)
	}

	var count int64
	var total int64
	g.Raw(`SELECT count(*), COALESCE(sum(gross_amount_idr),0) FROM history_txn
	        WHERE site_id = ? AND business_date = ?`, site, day).Row().Scan(&count, &total)
	if count != 2 {
		t.Fatalf("the site-day holds %d rows after two imports; re-import doubled the sales", count)
	}
	if total != 125_000 {
		t.Fatalf("the site-day totals %d, want 125000 (the corrected figures)", total)
	}
}

// createApprover makes a throwaway user holding one role in one company, so a
// race can be run between two genuinely distinct approvers.
func createApprover(t *testing.T, g *gorm.DB, roleID, companyID uuid.UUID) uuid.UUID {
	t.Helper()
	hash, err := security.HashPassword("kata sandi uji yang panjang")
	if err != nil {
		t.Fatal(err)
	}
	uid := id.New()
	email := "uji-" + uid.String() + "@sfg.local" // the WHOLE uuid: v7 shares its timestamp prefix, so a short slice collides
	if err := g.Exec(`INSERT INTO app_user (user_id, email, full_name, password_hash)
	                  VALUES (?, ?, 'Penguji Otomatis', ?)`, uid, email, hash).Error; err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(`INSERT INTO user_role (user_id, role_id, company_id) VALUES (?, ?, ?)`,
		uid, roleID, companyID).Error; err != nil {
		t.Fatal(err)
	}
	return uid
}

// --- the trailer rule (BR-6.4) -------------------------------------------

// The trailer exists to catch a TRUNCATED upload, and the row count is what
// catches that. The total is a second opinion and is only meaningful when
// every row loaded — a file with one bad line legitimately sums lower than its
// trailer states. Enforcing the total regardless failed the whole file for one
// bad row, which loses a night of trading to guard against something the row
// count already catches. Found by running the nightly job against a real file.
func TestTrailerTotalDoesNotFailAFileWithRejections(t *testing.T) {
	g := db(t)
	ctx := context.Background()

	dir := t.TempDir()
	// The receipt numbers carry a fresh id so each run has its own checksum.
	// Without it the SECOND run of this test is skipped as an already-loaded
	// file and the assertions below pass vacuously — idempotency by checksum
	// is real, and it is proven in TestReimportIsIdempotent, not here.
	//
	// The WHOLE uuid, not a prefix: a UUIDv7 begins with a millisecond
	// timestamp, so its first eight hex characters are identical for every
	// run inside the same ~65 second window. That is the second time that
	// trap has bitten in this suite.
	tag := id.NewString()
	good := "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr\n" +
		"MXX-001|2025-02-10|TR-" + tag + "-1|normal||dine_in|10000\n" +
		"MXX-001|2025-02-10|TR-" + tag + "-2|normal||dine_in|20000\n" +
		"MXX-001|2025-02-10|TR-" + tag + "-3|normal||dine_in|30000\n" +
		"NOPE-1|2025-02-10|TR-" + tag + "-4|normal||dine_in|40000\n" +
		"#TOTAL|4|100000\n"
	partial := filepath.Join(dir, "partial.csv")
	if err := os.WriteFile(partial, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}

	deps := testDeps(t, g)
	res, err := deps.ImportFile(ctx, partial, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped {
		t.Fatal("the file was skipped as already loaded; the rest of this test would pass vacuously")
	}
	if res.Run.Outcome != "PARTIAL" {
		t.Fatalf("outcome = %s (%s); one bad row must not fail the whole file",
			res.Run.Outcome, res.Run.Message)
	}
	if res.Run.RowsInserted != 3 {
		t.Fatalf("inserted %d rows, want 3", res.Run.RowsInserted)
	}
	if res.Run.RowsRejected != 1 {
		t.Fatalf("rejected %d rows, want 1", res.Run.RowsRejected)
	}
	// The shortfall must be EXPLAINED, not hidden: the reconciliation screen
	// has to show why the loaded total is below the file's own.
	if !strings.Contains(res.Run.Message, "ditolak") {
		t.Fatalf("the message must explain the shortfall, got %q", res.Run.Message)
	}

	// A genuinely truncated file STILL fails, on the row count.
	truncated := "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr\n" +
		"MXX-001|2025-02-11|TR-" + tag + "-9|normal||dine_in|10000\n" +
		"#TOTAL|9|90000\n"
	short := filepath.Join(dir, "truncated.csv")
	if err := os.WriteFile(short, []byte(truncated), 0o600); err != nil {
		t.Fatal(err)
	}
	res2, err := deps.ImportFile(ctx, short, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Run.Outcome != "FAILED" {
		t.Fatalf("a truncated file must FAIL, got %s", res2.Run.Outcome)
	}
	if res2.Run.RowsInserted != 0 {
		t.Fatalf("a truncated file must load nothing, loaded %d", res2.Run.RowsInserted)
	}
}

// testDeps builds the minimum app.Deps the importer needs.
func testDeps(t *testing.T, g *gorm.DB) *app.Deps {
	t.Helper()
	return &app.Deps{
		DB:     g,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return time.Now().UTC() },
		Params: postgres.NewParamRepo(g),
		Facts:  postgres.NewFactRepo(g),
		Audit:  postgres.NewAuditRepo(g),
	}
}

// --- media required on submit (BR-3.8, D54) -------------------------------

// The rule is enforced by a TRIGGER as well as by the application, so the test
// bypasses the application entirely: it moves a plan to PENDING with raw SQL,
// which is what a repair script at 2am would do.
func TestSubmitWithoutMediaIsRefusedByTheDatabase(t *testing.T) {
	g := db(t)

	company := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	group := mustUUID(t, g, `SELECT site_group_id FROM site_group
	    WHERE company_id = ? AND NOT is_system LIMIT 1`, company)
	creator := mustUUID(t, g, `SELECT user_id FROM app_user WHERE email = 'rina.hartono@sfg.local'`)

	planID, versionID := id.New(), id.New()
	code := "P-TEST-" + planID.String()[:8]
	if err := g.Exec(`INSERT INTO promotion_plan (plan_id, company_id, plan_code, site_group_id, status, created_by)
	                  VALUES (?, ?, ?, ?, 'DRAFT', ?)`,
		planID, company, code, group, creator).Error; err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(`
		INSERT INTO promotion_plan_version (version_id, plan_id, version_no, promo_name,
		    start_date, end_date, target_sales_idr, target_receipt_count, order_mode,
		    promo_rule, created_by)
		VALUES (?, ?, 1, 'Uji media wajib', '2027-06-01', '2027-06-30', 1000, 10,
		        'dine_in', '<p>x</p>', ?)`, versionID, planID, creator).Error; err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(`UPDATE promotion_plan SET current_version_id = ? WHERE plan_id = ?`,
		versionID, planID).Error; err != nil {
		t.Fatal(err)
	}

	// A DRAFT with no media is legitimate — BR-3.1, a draft may be incomplete.
	var status string
	g.Raw(`SELECT status FROM promotion_plan WHERE plan_id = ?`, planID).Row().Scan(&status)
	if status != "DRAFT" {
		t.Fatalf("the draft did not save: %s", status)
	}

	// Submitting it must be refused BY THE DATABASE.
	err := g.Exec(`UPDATE promotion_plan SET status = 'PENDING' WHERE plan_id = ?`, planID).Error
	if err == nil {
		t.Fatal("a plan with no media entered the approval chain: the trigger is not enforcing D54")
	}
	t.Logf("refused as designed: %v", err)

	// With one line it goes through.
	if err := g.Exec(`
		INSERT INTO promotion_media (media_id, version_id, line_no, media_name, price_idr)
		VALUES (?, ?, 1, 'Billboard Sudirman', 45000000)`, id.New(), versionID).Error; err != nil {
		t.Fatal(err)
	}
	if err := g.Exec(`UPDATE promotion_plan SET status = 'PENDING' WHERE plan_id = ?`, planID).Error; err != nil {
		t.Fatalf("one media line should be enough: %v", err)
	}

	// And the trigger must not re-fire on a later transition, or a plan could
	// never be released.
	if err := g.Exec(`UPDATE promotion_plan SET status = 'RELEASED' WHERE plan_id = ?`, planID).Error; err != nil {
		t.Fatalf("moving PENDING -> RELEASED must not be re-validated: %v", err)
	}

	_ = g.Exec(`DELETE FROM promotion_media WHERE version_id = ?`, versionID).Error
	_ = g.Exec(`DELETE FROM promotion_plan_version WHERE plan_id = ?`, planID).Error
	_ = g.Exec(`DELETE FROM promotion_plan WHERE plan_id = ?`, planID).Error
}
