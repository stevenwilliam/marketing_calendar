// Package target is target arithmetic and variance (BR-2).
//
// The rule this package exists to protect: the twelve month targets need not
// sum to the year target. A year target of Rp 1.2bn with months summing to
// Rp 1.35bn is VALID and must not be blocked. The system displays the
// variance; it never enforces agreement (BR-2.3).
package target

import (
	"errors"

	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
)

type (
	PeriodKind string
	SalesType  string
)

const (
	PeriodYear  PeriodKind = "YEAR"
	PeriodMonth PeriodKind = "MONTH"

	SalesNormal SalesType = "normal"
	SalesPromo  SalesType = "promo"
)

var (
	ErrNegative    = errors.New("target tidak boleh negatif")
	ErrPeriodShape = errors.New("bentuk periode tidak valid")
	ErrMonthRange  = errors.New("bulan harus antara 1 dan 12")
	ErrSalesType   = errors.New("jenis penjualan harus normal atau promo")
)

// Target is one row: one site, one period, one sales type (BR-2.1).
type Target struct {
	PeriodKind PeriodKind
	Year       int
	Month      int // zero when PeriodKind is YEAR
	SalesType  SalesType
	AmountIDR  money.IDR
}

// Validate enforces BR-2.2 and BR-2.4. It deliberately does NOT compare a
// year target with the sum of its months — see the package comment and
// TestMonthsNeedNotSumToYear. The absence is the rule.
func (t Target) Validate() error {
	if t.AmountIDR < 0 {
		return ErrNegative // BR-2.4; zero is meaningful (a site closed that month)
	}
	switch t.SalesType {
	case SalesNormal, SalesPromo:
	default:
		return ErrSalesType
	}
	switch t.PeriodKind {
	case PeriodYear:
		if t.Month != 0 {
			return ErrPeriodShape
		}
	case PeriodMonth:
		if t.Month < 1 || t.Month > 12 {
			return ErrMonthRange
		}
	default:
		return ErrPeriodShape
	}
	if t.Year < 2000 || t.Year > 2999 {
		return ErrPeriodShape
	}
	return nil
}

// Variance is the difference between a year target and the sum of its months.
// It is a DISPLAY value, never an error (BR-2.3).
type Variance struct {
	YearTarget money.IDR
	MonthsSum  money.IDR
	DeltaIDR   money.IDR // MonthsSum - YearTarget; positive means the months are ambitious
	DeltaBPS   int64
	HasYear    bool
	MonthCount int
}

// YearVersusMonths computes the variance. months may be sparse: a site with
// only three months set is normal in September, and treating the missing nine
// as zero would report a fictional shortfall — MonthCount says how many were
// actually set so the UI can say so.
func YearVersusMonths(yearTarget *money.IDR, months []money.IDR) (Variance, error) {
	sum, err := money.Sum(months...)
	if err != nil {
		return Variance{}, err
	}
	v := Variance{MonthsSum: sum, MonthCount: len(months)}
	if yearTarget == nil {
		return v, nil
	}
	v.HasYear = true
	v.YearTarget = *yearTarget
	if v.DeltaIDR, err = money.Sub(sum, *yearTarget); err != nil {
		return Variance{}, err
	}
	if bps, ok := money.PercentOfBPS(sum, *yearTarget); ok {
		v.DeltaBPS = bps - 10000
	}
	return v, nil
}

// Achievement is actual against target, for the report (BR-7.7).
type Achievement struct {
	TargetIDR money.IDR
	ActualIDR money.IDR
	DeltaIDR  money.IDR
	// AchievedBPS is 10000 for exactly on target. Defined is false when the
	// target is zero — a percentage of nothing is undefined, and rendering 0%
	// would read as a total miss for a site with no target set.
	AchievedBPS int64
	Defined     bool
}

func Achieved(targetIDR, actualIDR money.IDR) (Achievement, error) {
	d, err := money.Sub(actualIDR, targetIDR)
	if err != nil {
		return Achievement{}, err
	}
	a := Achievement{TargetIDR: targetIDR, ActualIDR: actualIDR, DeltaIDR: d}
	a.AchievedBPS, a.Defined = money.PercentOfBPS(actualIDR, targetIDR)
	return a, nil
}

// RollUp sums site targets into a group, brand or company figure. BR-2.6:
// roll-up is arithmetic computed on read, never a stored aggregate that can
// drift from the rows it summarises.
func RollUp(siteTargets []money.IDR) (money.IDR, error) {
	return money.Sum(siteTargets...)
}
