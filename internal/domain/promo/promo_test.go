package promo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, calendar.Jakarta)
}

func validVersion() Version {
	return Version{
		VersionID: uuid.New(), PlanID: uuid.New(), VersionNo: 1,
		PromoName: "Kopi Sore Hemat", StartDate: date(2026, time.September, 15),
		EndDate: date(2026, time.September, 30), TargetSalesIDR: 250_000_000,
		TargetReceiptCount: 4_000, OrderMode: DineIn,
		PromoRule: "Diskon 20% untuk pembelian kedua, 15.00-18.00",
	}
}

func draftPlan() Plan {
	return Plan{PlanID: uuid.New(), CompanyID: uuid.New(), PlanCode: "P-2026-0147",
		SiteGroupID: uuid.New(), Status: StatusDraft, CreatedBy: uuid.New()}
}

func check() SubmitCheck {
	return SubmitCheck{Now: date(2026, time.September, 1),
		Holidays: calendar.NewHolidaySet(nil), LeadTimeWorkingDays: 7}
}

func TestValidSubmit(t *testing.T) {
	if _, err := validVersion().CanSubmit(draftPlan(), check()); err != nil {
		t.Fatalf("a complete plan past its lead time must submit: %v", err)
	}
}

// BR-3.1: every substantive field is required AT SUBMIT. A draft may be
// incomplete, which is why this is not enforced on save.
func TestSubmitRequiresEveryField(t *testing.T) {
	cases := map[string]func(*Version){
		"promo_name":           func(v *Version) { v.PromoName = "  " },
		"promo_rule":           func(v *Version) { v.PromoRule = "" },
		"order_mode":           func(v *Version) { v.OrderMode = "both" },
		"target_sales_idr":     func(v *Version) { v.TargetSalesIDR = -1 },
		"target_receipt_count": func(v *Version) { v.TargetReceiptCount = -1 },
	}
	for field, mutate := range cases {
		v := validVersion()
		mutate(&v)
		r, err := v.CanSubmit(draftPlan(), check())
		if err == nil {
			t.Fatalf("%s: must be refused", field)
		}
		if _, named := r.Fields[field]; !named {
			t.Fatalf("%s: the error must name the field, got %v", field, r.Fields)
		}
	}
}

// BR-3.2: end must be on or after start, and a range may span months.
func TestPeriodOrderAndSpan(t *testing.T) {
	v := validVersion()
	v.EndDate = v.StartDate.AddDate(0, 0, -1)
	if _, err := v.CanSubmit(draftPlan(), check()); err == nil {
		t.Fatal("end before start must be refused")
	}
	spanning := validVersion()
	spanning.StartDate = date(2026, time.December, 20)
	spanning.EndDate = date(2027, time.January, 10)
	if _, err := spanning.CanSubmit(draftPlan(), check()); err != nil {
		t.Fatalf("a range spanning a year boundary is legal: %v", err)
	}
}

// BR-3.3: the lead time is evaluated at submit, against the holiday calendar.
func TestLeadTimeRefusedAndNamed(t *testing.T) {
	v := validVersion()
	v.StartDate = date(2026, time.September, 9) // one day inside the lead time
	r, err := v.CanSubmit(draftPlan(), check())
	if err != calendar.ErrLeadTime {
		t.Fatalf("want ErrLeadTime, got %v", err)
	}
	if !r.LeadTimeFailed {
		t.Fatal("the result must say the lead time was what failed")
	}
	// The response carries the earliest permitted date so the UI can say it
	// rather than leaving the user to guess (the disabled-states rule).
	if want := date(2026, time.September, 10); !r.EarliestStart.Equal(want) {
		t.Fatalf("EarliestStart = %s, want %s", r.EarliestStart, want)
	}
}

// A superadmin may override the lead time WITH a typed reason (BR-4.8).
// Without the reason, or without superadmin, the override does nothing.
func TestLeadTimeOverrideNeedsSuperadminAndReason(t *testing.T) {
	base := validVersion()
	base.StartDate = date(2026, time.September, 9)

	v := base
	v.LeadTimeOverridden = true
	v.LeadTimeOverrideReason = "arahan direksi, kampanye nasional"
	c := check()
	c.IsSuperadmin = true
	if _, err := v.CanSubmit(draftPlan(), c); err != nil {
		t.Fatalf("superadmin with a reason may override: %v", err)
	}

	noReason := base
	noReason.LeadTimeOverridden = true
	if _, err := noReason.CanSubmit(draftPlan(), c); err != calendar.ErrLeadTime {
		t.Fatalf("an override without a reason must not work, got %v", err)
	}

	notSuper := v
	c2 := check()
	if _, err := notSuper.CanSubmit(draftPlan(), c2); err != calendar.ErrLeadTime {
		t.Fatalf("a non-superadmin must not override, got %v", err)
	}
}

// BR-3.6: overlaps are allowed, and warned. The first submit is refused with
// the list; acknowledging it lets the same plan through.
func TestOverlapWarnsThenAccepts(t *testing.T) {
	v := validVersion()
	c := check()
	c.Overlaps = []Overlap{{PlanID: uuid.New(), PlanCode: "P-2026-0101",
		PromoName: "Promo Akhir Pekan", StartDate: date(2026, time.September, 12),
		EndDate: date(2026, time.September, 20)}}

	r, err := v.CanSubmit(draftPlan(), c)
	if err != ErrOverlapUnacked {
		t.Fatalf("want ErrOverlapUnacked, got %v", err)
	}
	if len(r.Overlaps) != 1 {
		t.Fatal("the warning must list what it overlaps with")
	}

	v.OverlapAcknowledged = true
	if _, err := v.CanSubmit(draftPlan(), c); err != nil {
		t.Fatalf("an acknowledged overlap must submit: %v", err)
	}
}

// Inclusive on both ends: two promotions both running on the 10th overlap.
// Adjacent ranges do not. This is the case a naive implementation misses.
func TestRangesOverlapIsInclusive(t *testing.T) {
	a1, a2 := date(2026, time.September, 1), date(2026, time.September, 10)
	if !RangesOverlap(a1, a2, date(2026, time.September, 10), date(2026, time.September, 20)) {
		t.Fatal("ranges sharing their boundary day overlap")
	}
	if RangesOverlap(a1, a2, date(2026, time.September, 11), date(2026, time.September, 20)) {
		t.Fatal("adjacent ranges (end = start-1) do not overlap")
	}
	if !RangesOverlap(a1, a2, date(2026, time.September, 3), date(2026, time.September, 4)) {
		t.Fatal("a range fully inside another overlaps")
	}
}

// BR-4.7, the most important control in the module: an approved plan cannot
// be edited. PENDING refuses too — approvers must approve the text that
// releases.
func TestApprovedPlanIsImmutable(t *testing.T) {
	for _, s := range []Status{StatusPending, StatusReleased, StatusCancelled} {
		p := draftPlan()
		p.Status = s
		if err := p.CanEdit(); err != ErrLocked {
			t.Fatalf("status %s must refuse an edit, got %v", s, err)
		}
	}
	for _, s := range []Status{StatusDraft, StatusRejected} {
		p := draftPlan()
		p.Status = s
		if err := p.CanEdit(); err != nil {
			t.Fatalf("status %s must allow an edit: %v", s, err)
		}
	}
}

// An edit creates a NEW version; the previous one is retained. The new version
// clears its acknowledgements: an overlap acknowledged against the old dates
// says nothing about the new ones.
func TestNextVersionClearsAcknowledgements(t *testing.T) {
	v := validVersion()
	v.OverlapAcknowledged = true
	v.LeadTimeOverridden = true
	v.LeadTimeOverrideReason = "arahan direksi"

	by := uuid.New()
	n := v.NextVersion(date(2026, time.September, 2), by)

	if n.VersionNo != 2 {
		t.Fatalf("VersionNo = %d, want 2", n.VersionNo)
	}
	if n.VersionID == v.VersionID {
		t.Fatal("a new version needs its own id; reusing it overwrites history")
	}
	if n.OverlapAcknowledged || n.LeadTimeOverridden || n.LeadTimeOverrideReason != "" {
		t.Fatal("acknowledgements must not carry into a new version")
	}
	if n.CreatedBy != by {
		t.Fatal("the new version is attributed to whoever made it")
	}
	// The original is untouched — it is still readable exactly as approved.
	if !v.OverlapAcknowledged || v.VersionNo != 1 {
		t.Fatal("the previous version must be retained unmodified")
	}
}

// BR-4.6: the daily job cancels a plan still pending on the Mth day before
// start. The boundary day itself cancels; the day before does not.
func TestShouldAutoCancelBoundary(t *testing.T) {
	p := draftPlan()
	p.Status = StatusPending
	v := validVersion() // starts 15 Sep
	m := 5              // cancellation date is 10 Sep

	if ShouldAutoCancel(p, v, date(2026, time.September, 9), m) {
		t.Fatal("the day before the cancellation date must not cancel")
	}
	if !ShouldAutoCancel(p, v, date(2026, time.September, 10), m) {
		t.Fatal("the cancellation date itself must cancel")
	}
	if !ShouldAutoCancel(p, v, date(2026, time.September, 12), m) {
		t.Fatal("a job that missed a day must still catch the plan")
	}
	// Only PENDING plans are touched. A released plan is never auto-cancelled.
	for _, s := range []Status{StatusDraft, StatusReleased, StatusRejected, StatusCancelled} {
		p.Status = s
		if ShouldAutoCancel(p, v, date(2026, time.September, 12), m) {
			t.Fatalf("status %s must not be auto-cancelled", s)
		}
	}
}

// A rejected plan may be edited and resubmitted (BR-4.5).
func TestRejectedPlanMayResubmit(t *testing.T) {
	p := draftPlan()
	p.Status = StatusRejected
	if _, err := validVersion().CanSubmit(p, check()); err != nil {
		t.Fatalf("a rejected plan must be resubmittable: %v", err)
	}
	p.Status = StatusReleased
	if _, err := validVersion().CanSubmit(p, check()); err != ErrNotSubmittable {
		t.Fatalf("a released plan must not resubmit, got %v", err)
	}
}
