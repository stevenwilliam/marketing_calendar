package calendar

import (
	"testing"
	"time"
)

// The fixed dates are certain and must never be marked provisional.
func TestFixedHolidaysAreCertain(t *testing.T) {
	got := Generate(2028)
	want := map[string]string{
		"2028-01-01": "Tahun Baru Masehi",
		"2028-05-01": "Hari Buruh Internasional",
		"2028-06-01": "Hari Lahir Pancasila",
		"2028-08-17": "Hari Kemerdekaan Republik Indonesia",
		"2028-12-25": "Hari Raya Natal",
	}
	for iso, name := range want {
		var found *Generated
		for i := range got {
			if got[i].Date.Format("2006-01-02") == iso && got[i].Name == name {
				found = &got[i]
			}
		}
		if found == nil {
			t.Fatalf("%s %s is missing", iso, name)
		}
		if found.Provisional {
			t.Fatalf("%s is a fixed date and must not be provisional", iso)
		}
	}
}

// Easter is computable exactly, so Good Friday and Ascension are certain too.
// These are the published Western Easter dates; if the computus is wrong the
// lead time is wrong every spring.
func TestEasterDerivedDatesAreExact(t *testing.T) {
	easters := map[int]string{
		2026: "2026-04-05", 2027: "2027-03-28", 2028: "2028-04-16",
		2029: "2029-04-01", 2030: "2030-04-21",
	}
	for year, iso := range easters {
		want, _ := time.Parse("2006-01-02", iso)
		got := gregorianEaster(year)
		if got.Format("2006-01-02") != iso {
			t.Fatalf("Easter %d = %s, want %s", year, got.Format("2006-01-02"), iso)
		}
		// Good Friday is Easter - 2, Ascension is Easter + 39.
		var goodFriday, ascension *Generated
		for i, h := range Generate(year) {
			_ = i
			switch h.Name {
			case "Wafat Isa Almasih":
				g := h
				goodFriday = &g
			case "Kenaikan Isa Almasih":
				g := h
				ascension = &g
			}
		}
		if goodFriday == nil || ascension == nil {
			t.Fatalf("%d: the Easter-derived holidays are missing", year)
		}
		if goodFriday.Provisional || ascension.Provisional {
			t.Fatalf("%d: Easter-derived dates are exact and must not be provisional", year)
		}
		if !goodFriday.Date.Equal(want.AddDate(0, 0, -2).In(Jakarta).Truncate(0)) &&
			goodFriday.Date.Format("2006-01-02") != want.AddDate(0, 0, -2).Format("2006-01-02") {
			t.Fatalf("%d: Good Friday = %s", year, goodFriday.Date.Format("2006-01-02"))
		}
		if ascension.Date.Format("2006-01-02") != want.AddDate(0, 0, 39).Format("2006-01-02") {
			t.Fatalf("%d: Ascension = %s, want %s", year,
				ascension.Date.Format("2006-01-02"), want.AddDate(0, 0, 39).Format("2006-01-02"))
		}
	}
}

// Every lunar-calendar date must be marked provisional. This is the test that
// stops an estimate being presented as fact: the lead time is computed from
// these dates, and BR-3.3 exists because a wrong holiday yields a wrong date
// that still looks like seven days.
func TestEveryLunarDateIsMarkedProvisional(t *testing.T) {
	lunar := map[string]bool{
		"Idul Fitri": true, "Idul Adha": true, "Tahun Baru Islam": true,
		"Maulid Nabi Muhammad SAW": true, "Isra Mikraj Nabi Muhammad SAW": true,
	}
	for year := 2026; year <= 2030; year++ {
		for _, h := range Generate(year) {
			if lunar[h.Name] && !h.Provisional {
				t.Fatalf("%d %s (%s) is an estimate and is not marked as one",
					year, h.Name, h.Date.Format("2006-01-02"))
			}
		}
	}
}

// The Islamic approximation should land within a couple of days of the
// announced dates. This is a SANITY check on the arithmetic, not a claim of
// accuracy — the test tolerance is the honest statement of what the tabular
// calendar can promise.
func TestIslamicDatesAreInTheRightPartOfTheYear(t *testing.T) {
	// Announced Idul Fitri dates: 2026-03-20, 2027-03-09, 2028-02-26.
	want := map[int]string{2026: "2026-03-20", 2027: "2027-03-09", 2028: "2028-02-26"}
	for year, iso := range want {
		target, _ := time.Parse("2006-01-02", iso)
		var closest time.Time
		best := 1 << 30
		for _, h := range Generate(year) {
			if h.Name != "Idul Fitri" {
				continue
			}
			// Whole days between two midnights, not a truncated hour count:
			// 23.9 hours is one day apart, and truncation reported it as zero.
			a := time.Date(h.Date.Year(), h.Date.Month(), h.Date.Day(), 0, 0, 0, 0, time.UTC)
			b := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
			diff := int(a.Sub(b).Hours()+0.5) / 24
			if diff < 0 {
				diff = -diff
			}
			if diff < best {
				best, closest = diff, h.Date
			}
		}
		if best > 3 {
			t.Fatalf("%d: nearest computed Idul Fitri is %s, %d days from the announced %s — "+
				"the tabular arithmetic is wrong, not merely imprecise",
				year, closest.Format("2006-01-02"), best, iso)
		}
		t.Logf("%d: computed %s vs announced %s (%d day(s) out — provisional, as marked)",
			year, closest.Format("2006-01-02"), iso, best)
	}
}

// Five years must all produce a usable set.
func TestFiveYearsAllGenerate(t *testing.T) {
	for year := 2026; year <= 2030; year++ {
		got := Generate(year)
		if len(got) < 10 {
			t.Fatalf("%d produced only %d holidays", year, len(got))
		}
		seen := map[string]bool{}
		for _, h := range got {
			if h.Date.Year() != year {
				t.Fatalf("%d produced a date in %d: %s", year, h.Date.Year(), h.Name)
			}
			key := h.Date.Format("2006-01-02") + h.Name
			if seen[key] {
				t.Fatalf("%d produced %s twice on %s", year, h.Name, h.Date.Format("2006-01-02"))
			}
			seen[key] = true
		}
	}
}
