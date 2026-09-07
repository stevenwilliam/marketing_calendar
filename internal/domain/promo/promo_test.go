package promo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
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
		Media: []Media{
			{LineNo: 1, Name: "Billboard Sudirman", PriceIDR: 45_000_000},
		},
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

// --- marketing media (BR-3.8, D53) ---------------------------------------

func TestMediaTotalIsIntegerArithmetic(t *testing.T) {
	lines := []Media{
		{LineNo: 1, Name: "Billboard Sudirman", PriceIDR: 45_000_000},
		{LineNo: 2, Name: "Instagram Ads", PriceIDR: 12_500_000},
		{LineNo: 3, Name: "Radio Prambors", PriceIDR: 7_250_000},
	}
	got, err := MediaTotal(lines)
	if err != nil {
		t.Fatal(err)
	}
	if got != 64_750_000 {
		t.Fatalf("total = %d, want 64750000", got)
	}
}

func TestMediaTotalOfNothingIsZeroNotAnError(t *testing.T) {
	got, err := MediaTotal(nil)
	if err != nil || got != 0 {
		t.Fatalf("got %d, %v; a promotion that buys no media is a real promotion", got, err)
	}
}

// The overflow guard has to reach this path too, or a pathological set of
// lines wraps the total into a negative number.
func TestMediaTotalRefusesOverflow(t *testing.T) {
	lines := []Media{{PriceIDR: money.MaxIDR}, {PriceIDR: money.MaxIDR}}
	if _, err := MediaTotal(lines); err == nil {
		t.Fatal("two enormous lines must not wrap the total")
	}
}

// Media is optional; a line that exists must be complete. A blank row carrying
// a price is a number nobody can account for.
func TestIncompleteMediaLineIsRefusedAndNamed(t *testing.T) {
	v := validVersion()
	v.Media = []Media{
		{LineNo: 1, Name: "Billboard", PriceIDR: 1_000_000},
		{LineNo: 2, Name: "   ", PriceIDR: 500_000},
		{LineNo: 3, Name: "Radio", PriceIDR: -1},
	}
	f := v.ValidateForSubmit()
	if f == nil {
		t.Fatal("an incomplete media line must be refused")
	}
	if _, ok := f["media.2.name"]; !ok {
		t.Fatalf("the blank name must be named by its line: %v", f)
	}
	if _, ok := f["media.3.price"]; !ok {
		t.Fatalf("the negative price must be named by its line: %v", f)
	}
	// And line 1 is fine, so it must NOT be named.
	if _, ok := f["media.1.name"]; ok {
		t.Fatalf("a valid line was reported as an error: %v", f)
	}
}

// Superseded by D54: media used to be optional and is now required at submit.
// The draft case it protected is covered where it actually lives — the draft
// path does not call ValidateForSubmit at all — and is asserted end to end in
// test/integration_test.go.
func TestValidVersionCarriesMedia(t *testing.T) {
	if len(validVersion().Media) == 0 {
		t.Fatal("the fixture must carry media, or every submit test passes for the wrong reason")
	}
}

// An edit that changed only the dates must not silently drop what the campaign
// is buying.
func TestNewVersionCarriesMediaForward(t *testing.T) {
	v := validVersion()
	v.Media = []Media{{LineNo: 1, Name: "Billboard", PriceIDR: 1_000_000}}
	n := v.NextVersion(date(2026, time.September, 2), uuid.New())
	if len(n.Media) != 1 || n.Media[0].Name != "Billboard" {
		t.Fatalf("media was lost: %+v", n.Media)
	}
	// And it must be a COPY: mutating the new version must not reach back into
	// the old one, which is still the approved record.
	n.Media[0].Name = "diubah"
	if v.Media[0].Name != "Billboard" {
		t.Fatal("the previous version was mutated through a shared slice")
	}
}

// D54: at least one media line to SUBMIT. A draft may still have none —
// BR-3.1 says a draft may be incomplete, and this is the submit gate.
func TestSubmitRequiresAtLeastOneMediaLine(t *testing.T) {
	v := validVersion()
	v.Media = nil

	f := v.ValidateForSubmit()
	if f == nil {
		t.Fatal("submitting with no media must be refused")
	}
	if _, ok := f["media"]; !ok {
		t.Fatalf("the refusal must name the media field: %v", f)
	}

	// One line is enough.
	v.Media = []Media{{LineNo: 1, Name: "Billboard Sudirman", PriceIDR: 45_000_000}}
	if f := v.ValidateForSubmit(); f != nil {
		t.Fatalf("one complete line is enough: %v", f)
	}
}

// A line priced at zero still counts. Owned media — a shop's own window, its
// social account — costs nothing to place and is still media.
func TestAZeroPricedMediaLineCounts(t *testing.T) {
	v := validVersion()
	v.Media = []Media{{LineNo: 1, Name: "Etalase toko sendiri", PriceIDR: 0}}
	if f := v.ValidateForSubmit(); f != nil {
		t.Fatalf("a zero-priced line is still a line: %v", f)
	}
	total, err := MediaTotal(v.Media)
	if err != nil || total != 0 {
		t.Fatalf("total = %d, %v", total, err)
	}
}

// The whole submit gate, in one place: no media is refused, and it is refused
// ALONGSIDE the other missing fields rather than instead of them.
func TestSubmitReportsEveryMissingFieldAtOnce(t *testing.T) {
	v := validVersion()
	v.Media = nil
	v.PromoName = "  "
	v.OrderMode = "both"

	f := v.ValidateForSubmit()
	for _, want := range []string{"media", "promo_name", "order_mode"} {
		if _, ok := f[want]; !ok {
			t.Fatalf("%s was not reported; a user should not fix one field at a time: %v", want, f)
		}
	}
}

// --- daily target and achievement bands (D55) ----------------------------

func TestDaysIsInclusiveOfBothEnds(t *testing.T) {
	v := validVersion()
	v.StartDate = date(2026, time.September, 15)
	v.EndDate = date(2026, time.September, 15)
	if got := v.Days(); got != 1 {
		t.Fatalf("a one-day promotion is %d days, want 1", got)
	}
	v.EndDate = date(2026, time.September, 30)
	if got := v.Days(); got != 16 {
		t.Fatalf("15th to 30th inclusive is %d days, want 16", got)
	}
}

func TestDailyTargetDividesTheTotal(t *testing.T) {
	v := validVersion()
	v.StartDate = date(2026, time.September, 1)
	v.EndDate = date(2026, time.September, 30)
	v.TargetSalesIDR = 250_000_000

	amount, days, ok := v.DailyTarget()
	if !ok || days != 30 {
		t.Fatalf("days = %d, ok = %v", days, ok)
	}
	// Integer division truncates, deliberately: this product has no fractional
	// rupiah, and the promotion's own variance is computed against the real
	// total rather than against this figure multiplied back up.
	if amount != 8_333_333 {
		t.Fatalf("daily target = %d, want 8333333", amount)
	}
	if amount*money.IDR(days) > v.TargetSalesIDR {
		t.Fatal("the daily target must never round UP: it would set a target above the plan's own")
	}
}

// A promotion nobody set a number on has no daily target, and a percentage of
// zero is undefined — not 0%, which would read as a total miss.
func TestNoDailyTargetWhenThereIsNoTarget(t *testing.T) {
	v := validVersion()
	v.TargetSalesIDR = 0
	if _, _, ok := v.DailyTarget(); ok {
		t.Fatal("a zero total must not produce a daily target")
	}
	if band, _, ok := BandFor(500_000, 0, DefaultAchievementGreenBPS); ok || band != BandNone {
		t.Fatalf("band = %s, ok = %v; a percentage of zero is undefined", band, ok)
	}
}

// D59: two bands, split at the green line, closed at the bottom. A day that
// hit its number exactly is green, not red.
func TestBandBoundaries(t *testing.T) {
	const daily = money.IDR(1_000_000)
	const green = DefaultAchievementGreenBPS // 80%
	cases := []struct {
		actual money.IDR
		want   Band
		note   string
	}{
		{0, BandUnder, "nothing sold"},
		{799_000, BandUnder, "79,90% is under"},
		{800_000, BandOver, "exactly 80% is over, not under"},
		{999_000, BandOver, "99,90% is over — there is no middle band any more"},
		{1_000_000, BandOver, "exactly 100%"},
		{2_500_000, BandOver, "well past"},
	}
	// The band follows the DISPLAYED percentage, so a chip can never show a
	// number that contradicts its colour. 799.999 rounds to 80,00% and is
	// therefore green, not red.
	if band, bps, _ := BandFor(799_999, daily, green); band != BandOver || bps != 8000 {
		t.Fatalf("799999 of 1000000 rounds to %d bps and banded %s; the colour must follow the number shown", bps, band)
	}
	for _, c := range cases {
		got, bps, ok := BandFor(c.actual, daily, green)
		if !ok {
			t.Fatalf("%s: no band", c.note)
		}
		if got != c.want {
			t.Fatalf("%s: actual %d of %d = %d bps -> %s, want %s",
				c.note, c.actual, daily, bps, got, c.want)
		}
	}
}

// The green line is a parameter (D59, CLAUDE.md §7). Moving it must move the
// band, and an unreadable value must not turn the whole calendar green.
func TestBandGreenLineIsConfigurable(t *testing.T) {
	const daily = money.IDR(1_000_000)
	if band, _, _ := BandFor(750_000, daily, 7000); band != BandOver {
		t.Fatalf("75%% against a 70%% line = %s, want over", band)
	}
	if band, _, _ := BandFor(750_000, daily, 8000); band != BandUnder {
		t.Fatalf("75%% against an 80%% line = %s, want under", band)
	}
	// A missing or nonsensical parameter falls back rather than banding
	// everything green, which would silently report a perfect month.
	for _, bad := range []int64{0, -1} {
		if band, _, _ := BandFor(750_000, daily, bad); band != BandUnder {
			t.Fatalf("greenBPS %d must fall back to the default; got %s", bad, band)
		}
	}
}

// BR-7.5b. A zero is only a miss where selling was possible.
func TestVerdict(t *testing.T) {
	const today = "2026-09-07"
	cases := []struct {
		name          string
		status        Status
		date          string
		hasActual     bool
		targetDefined bool
		want          NoVerdict
	}{
		{"released, past day, no sales -> 0% is a real miss",
			StatusReleased, "2026-09-06", false, true, VerdictBanded},
		{"released, today, no sales -> today counts",
			StatusReleased, today, false, true, VerdictBanded},
		{"released, tomorrow, no sales -> nobody has lived through it",
			StatusReleased, "2026-09-08", false, true, VerdictNotYet},
		{"draft, past day -> it could not have sold anything",
			StatusDraft, "2026-09-06", false, true, VerdictNotReleased},
		{"pending, past day -> still not released",
			StatusPending, "2026-09-06", false, true, VerdictNotReleased},
		{"rejected, past day", StatusRejected, "2026-09-06", false, true, VerdictNotReleased},
		{"cancelled, past day", StatusCancelled, "2026-09-06", false, true, VerdictNotReleased},
		{"zero target outranks everything else",
			StatusReleased, "2026-09-06", true, false, VerdictNoTarget},
		// The escape hatch. A recorded number is never suppressed, whatever
		// the status or the date says — otherwise a gate meant to avoid a
		// false accusation would start hiding real sales.
		{"draft WITH actuals is still banded",
			StatusDraft, "2026-09-06", true, true, VerdictBanded},
		{"future day WITH actuals is still banded",
			StatusReleased, "2026-12-31", true, true, VerdictBanded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Verdict(c.status, c.date, today, c.hasActual, c.targetDefined)
			if got != c.want {
				t.Fatalf("Verdict(%s, %s) = %q, want %q", c.status, c.date, got, c.want)
			}
		})
	}
}
