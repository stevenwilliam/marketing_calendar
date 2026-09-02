package target

import (
	"testing"

	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
)

// BR-2.3, the rule most likely to be "helpfully" broken by a well-meaning
// validation. A year of Rp 1.2bn with months summing to Rp 1.35bn is VALID.
// There is deliberately no code that refuses it; this test is the guard that
// makes adding such code fail loudly.
func TestMonthsNeedNotSumToYear(t *testing.T) {
	year := money.IDR(1_200_000_000)
	months := make([]money.IDR, 12)
	for i := range months {
		months[i] = 112_500_000 // 12 x 112.5m = 1.35bn
	}

	// Every individual target validates.
	for i, m := range months {
		tg := Target{PeriodKind: PeriodMonth, Year: 2026, Month: i + 1,
			SalesType: SalesNormal, AmountIDR: m}
		if err := tg.Validate(); err != nil {
			t.Fatalf("month %d rejected: %v", i+1, err)
		}
	}
	yt := Target{PeriodKind: PeriodYear, Year: 2026, SalesType: SalesNormal, AmountIDR: year}
	if err := yt.Validate(); err != nil {
		t.Fatalf("year target rejected: %v", err)
	}

	v, err := YearVersusMonths(&year, months)
	if err != nil {
		t.Fatal(err)
	}
	if v.DeltaIDR != 150_000_000 {
		t.Fatalf("delta = %d, want 150000000", v.DeltaIDR)
	}
	if v.DeltaBPS != 1250 { // +12.5%
		t.Fatalf("deltaBPS = %d, want 1250", v.DeltaBPS)
	}
}

// BR-2.4: zero is meaningful — a site closed that month. Negative is refused.
func TestZeroIsValidNegativeIsNot(t *testing.T) {
	zero := Target{PeriodKind: PeriodMonth, Year: 2026, Month: 2, SalesType: SalesNormal}
	if err := zero.Validate(); err != nil {
		t.Fatalf("zero must be accepted (a closed site): %v", err)
	}
	neg := Target{PeriodKind: PeriodMonth, Year: 2026, Month: 2, SalesType: SalesNormal, AmountIDR: -1}
	if err := neg.Validate(); err != ErrNegative {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

// BR-2.2: a YEAR target carries no month; a MONTH target carries 1-12.
func TestPeriodShape(t *testing.T) {
	bad := []Target{
		{PeriodKind: PeriodYear, Year: 2026, Month: 3, SalesType: SalesNormal},
		{PeriodKind: PeriodMonth, Year: 2026, Month: 0, SalesType: SalesNormal},
		{PeriodKind: PeriodMonth, Year: 2026, Month: 13, SalesType: SalesNormal},
		{PeriodKind: "QUARTER", Year: 2026, SalesType: SalesNormal},
		{PeriodKind: PeriodMonth, Year: 2026, Month: 1, SalesType: "both"},
	}
	for i, tg := range bad {
		if err := tg.Validate(); err == nil {
			t.Fatalf("case %d must be rejected: %+v", i, tg)
		}
	}
}

// A sparse set of months is normal in September. Reporting the missing nine as
// zero would show a fictional shortfall, so MonthCount says what was measured.
func TestSparseMonthsReportTheirCount(t *testing.T) {
	year := money.IDR(1_200_000_000)
	v, err := YearVersusMonths(&year, []money.IDR{100_000_000, 100_000_000, 100_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if v.MonthCount != 3 {
		t.Fatalf("MonthCount = %d, want 3", v.MonthCount)
	}
	if v.MonthsSum != 300_000_000 {
		t.Fatalf("sum = %d", v.MonthsSum)
	}
}

func TestNoYearTargetIsNotAnError(t *testing.T) {
	v, err := YearVersusMonths(nil, []money.IDR{5, 5})
	if err != nil {
		t.Fatal(err)
	}
	if v.HasYear {
		t.Fatal("HasYear must be false when no year target is set")
	}
}

// BR-7.7 and the money rule: achievement against a zero target is undefined,
// not 0% — a site with no target must not read as a total miss.
func TestAchievedAgainstZeroTargetIsUndefined(t *testing.T) {
	a, err := Achieved(0, 500_000)
	if err != nil {
		t.Fatal(err)
	}
	if a.Defined {
		t.Fatal("achievement against a zero target must be undefined")
	}
	if a.DeltaIDR != 500_000 {
		t.Fatalf("delta = %d", a.DeltaIDR)
	}
	b, _ := Achieved(1_000_000, 1_200_000)
	if !b.Defined || b.AchievedBPS != 12000 {
		t.Fatalf("AchievedBPS = %d, defined = %v; want 12000, true", b.AchievedBPS, b.Defined)
	}
}

// BR-2.6: a roll-up is arithmetic on read, never a stored aggregate.
func TestRollUpIsTheSumOfSites(t *testing.T) {
	got, err := RollUp([]money.IDR{100, 250, 0, 700})
	if err != nil || got != 1050 {
		t.Fatalf("RollUp = %d, %v", got, err)
	}
}
