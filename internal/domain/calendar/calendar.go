// Package calendar is working-day arithmetic in Asia/Jakarta.
//
// BR-3.3, and the single most error-prone calculation in the system. Weekday
// arithmetic alone is not sufficient: around Idul Fitri, Christmas and Nyepi
// it yields a date the lead time never legitimately allowed, and the error is
// invisible because the number still looks like seven.
package calendar

import (
	"errors"
	"time"
)

// Jakarta is the operating zone. Storage is UTC (BR-1.4); every business date
// is evaluated here, explicitly, never in the server's local zone.
var Jakarta = mustLoad("Asia/Jakarta")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// A missing tzdata is a deployment fault that must be loud at boot,
		// not a silent fallback to UTC that shifts every business date by
		// seven hours.
		panic("calendar: zona waktu " + name + " tidak tersedia: " + err.Error())
	}
	return loc
}

var ErrLeadTime = errors.New("tanggal mulai lebih awal dari masa tenggang")

// HolidaySet is the administrator-maintained Indonesian holiday calendar
// (D22). A date is keyed by its yyyy-mm-dd in Jakarta.
type HolidaySet map[string]string // date -> holiday name

func Key(t time.Time) string { return t.In(Jakarta).Format("2006-01-02") }

func NewHolidaySet(byDate map[string]string) HolidaySet {
	h := make(HolidaySet, len(byDate))
	for k, v := range byDate {
		h[k] = v
	}
	return h
}

func (h HolidaySet) Name(t time.Time) (string, bool) {
	n, ok := h[Key(t)]
	return n, ok
}

// Today is the business date in Jakarta for an instant.
func Today(now time.Time) time.Time {
	t := now.In(Jakarta)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Jakarta)
}

// IsWorkingDay excludes Saturdays, Sundays and any active holiday.
func (h HolidaySet) IsWorkingDay(t time.Time) bool {
	d := t.In(Jakarta)
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	_, isHoliday := h[Key(d)]
	return !isHoliday
}

// EarliestStart returns the first permitted promotion start date: n working
// days after today, counting exclusive of today and inclusive of the result.
//
// With n=7 and today Tuesday 1 September 2026 (no holidays) it counts Wed 2,
// Thu 3, Fri 4, Mon 7, Tue 8, Wed 9, Thu 10 and returns 10 September 2026 —
// the worked example in the brief and in BR-3.3.
func (h HolidaySet) EarliestStart(today time.Time, n int) time.Time {
	d := Today(today)
	if n <= 0 {
		return d
	}
	counted := 0
	// Bounded so a pathological holiday table cannot spin forever: 4n days is
	// far beyond any real run of non-working days, and running out is a
	// configuration fault worth surfacing as a wrong date rather than a hang.
	for i := 0; counted < n && i < n*4+400; i++ {
		d = d.AddDate(0, 0, 1)
		if h.IsWorkingDay(d) {
			counted++
		}
	}
	return d
}

// WorkingDaysBetween counts working days in (from, to], the same convention
// EarliestStart uses. Negative when to is before from.
func (h HolidaySet) WorkingDaysBetween(from, to time.Time) int {
	a, b := Today(from), Today(to)
	sign := 1
	if b.Before(a) {
		a, b = b, a
		sign = -1
	}
	n := 0
	for d := a.AddDate(0, 0, 1); !d.After(b); d = d.AddDate(0, 0, 1) {
		if h.IsWorkingDay(d) {
			n++
		}
	}
	return n * sign
}

// CheckLeadTime is the rule as the submit path applies it (BR-3.3).
func (h HolidaySet) CheckLeadTime(today, start time.Time, n int) error {
	if Today(start).Before(h.EarliestStart(today, n)) {
		return ErrLeadTime
	}
	return nil
}

// AutoCancelDate is the day an incomplete chain is cancelled: m calendar days
// before the start date (BR-4.6). Calendar days, not working days — the job
// runs daily and the parameter is expressed in days.
func AutoCancelDate(start time.Time, m int) time.Time {
	return Today(start).AddDate(0, 0, -m)
}

// MonthBounds returns the first and last business date of a month in Jakarta.
func MonthBounds(year int, month time.Month) (time.Time, time.Time) {
	first := time.Date(year, month, 1, 0, 0, 0, 0, Jakarta)
	return first, first.AddDate(0, 1, -1)
}

// ParseDate reads a yyyy-mm-dd business date in Jakarta.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Jakarta)
}
