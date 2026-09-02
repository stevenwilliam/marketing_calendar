package calendar

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, Jakarta)
}

// BR-3.3, the worked example from the brief: today Tue 1 Sep 2026, N=7,
// no holidays -> earliest start Thu 10 Sep 2026.
func TestEarliestStartWorkedExample(t *testing.T) {
	h := NewHolidaySet(nil)
	got := h.EarliestStart(date(2026, time.September, 1), 7)
	want := date(2026, time.September, 10)
	if !got.Equal(want) {
		t.Fatalf("EarliestStart = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// The rule that weekday arithmetic alone gets wrong. Idul Fitri 2027 falls in
// March; with the holiday week loaded the lead time must push PAST it, and the
// answer differs from the weekday-only one by exactly the holidays crossed.
func TestEarliestStartCrossesIdulFitri(t *testing.T) {
	// Mon 8 - Fri 12 March 2027 as the national holiday week.
	h := NewHolidaySet(map[string]string{
		"2027-03-08": "Cuti Bersama Idul Fitri",
		"2027-03-09": "Cuti Bersama Idul Fitri",
		"2027-03-10": "Idul Fitri 1448 H",
		"2027-03-11": "Idul Fitri 1448 H",
		"2027-03-12": "Cuti Bersama Idul Fitri",
	})
	today := date(2027, time.March, 4) // Thursday
	withHolidays := h.EarliestStart(today, 7)
	withoutHolidays := NewHolidaySet(nil).EarliestStart(today, 7)

	if withHolidays.Equal(withoutHolidays) {
		t.Fatal("the holiday calendar changed nothing — weekday arithmetic is not sufficient (BR-3.3)")
	}
	// Weekday-only: Fri 5, Mon 8, Tue 9, Wed 10, Thu 11, Fri 12, Mon 15.
	if want := date(2027, time.March, 15); !withoutHolidays.Equal(want) {
		t.Fatalf("weekday-only = %s, want %s", withoutHolidays.Format("2006-01-02"), want.Format("2006-01-02"))
	}
	// With the week excluded: Fri 5, Mon 15, Tue 16, Wed 17, Thu 18, Fri 19, Mon 22.
	if want := date(2027, time.March, 22); !withHolidays.Equal(want) {
		t.Fatalf("with holidays = %s, want %s", withHolidays.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestEarliestStartSkipsWeekend(t *testing.T) {
	h := NewHolidaySet(nil)
	// Friday 4 Sep 2026 + 1 working day = Monday 7 Sep.
	if got, want := h.EarliestStart(date(2026, time.September, 4), 1), date(2026, time.September, 7); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestIsWorkingDay(t *testing.T) {
	h := NewHolidaySet(map[string]string{"2026-12-25": "Hari Natal"})
	cases := map[string]bool{
		"2026-09-01": true,  // Tuesday
		"2026-09-05": false, // Saturday
		"2026-09-06": false, // Sunday
		"2026-12-25": false, // Christmas, a Friday
	}
	for s, want := range cases {
		d, err := ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := h.IsWorkingDay(d); got != want {
			t.Fatalf("IsWorkingDay(%s) = %v, want %v", s, got, want)
		}
	}
}

// The lead time is evaluated at submit. A date on the boundary is legal;
// an off-by-one here rejects a plan that is exactly on time.
func TestCheckLeadTimeBoundary(t *testing.T) {
	h := NewHolidaySet(nil)
	today := date(2026, time.September, 1)
	if err := h.CheckLeadTime(today, date(2026, time.September, 10), 7); err != nil {
		t.Fatalf("the earliest permitted date must be accepted: %v", err)
	}
	if err := h.CheckLeadTime(today, date(2026, time.September, 9), 7); err != ErrLeadTime {
		t.Fatalf("one day early must be refused, got %v", err)
	}
	if err := h.CheckLeadTime(today, date(2026, time.September, 30), 7); err != nil {
		t.Fatalf("a late start is fine: %v", err)
	}
}

// BR-4.6: M calendar days before start, because the job runs daily and the
// parameter is expressed in days.
func TestAutoCancelDate(t *testing.T) {
	if got, want := AutoCancelDate(date(2026, time.September, 10), 5), date(2026, time.September, 5); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// Storage is UTC; the business date is Jakarta. An instant late on 31 August
// in UTC is already 1 September in Jakarta, and getting this backwards moves
// every business date by a day for seven hours out of every twenty-four.
func TestTodayConvertsFromUTCExplicitly(t *testing.T) {
	utc := time.Date(2026, time.August, 31, 18, 30, 0, 0, time.UTC) // 01:30 on 1 Sep in Jakarta
	if got, want := Today(utc), date(2026, time.September, 1); !got.Equal(want) {
		t.Fatalf("Today(%s) = %s, want %s", utc, got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestWorkingDaysBetween(t *testing.T) {
	h := NewHolidaySet(nil)
	if got := h.WorkingDaysBetween(date(2026, time.September, 1), date(2026, time.September, 10)); got != 7 {
		t.Fatalf("WorkingDaysBetween = %d, want 7", got)
	}
	if got := h.WorkingDaysBetween(date(2026, time.September, 10), date(2026, time.September, 1)); got != -7 {
		t.Fatalf("reversed = %d, want -7", got)
	}
}
