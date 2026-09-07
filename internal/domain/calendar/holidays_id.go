package calendar

import "time"

// Indonesian public holidays, generated.
//
// They fall into three groups, and only the first two are knowable in advance:
//
//	fixed    1 Jan, 1 May, 1 Jun, 17 Aug, 25 Dec. Certain, every year.
//	derived  Wafat Isa Almasih (Good Friday) and Kenaikan Isa Almasih
//	         (Ascension) follow Easter, which is computable exactly.
//	decreed  Idul Fitri, Idul Adha, Tahun Baru Islam, Maulid, Isra Mikraj,
//	         Nyepi, Waisak, Imlek, and every cuti bersama. These are set by a
//	         joint ministerial decree, normally the year before, and are NOT
//	         reliably computable — the Islamic ones depend on a sighting, and
//	         Nyepi and Waisak on calendars this package does not implement.
//
// Everything in the third group is emitted as PROVISIONAL. It still drives the
// lead time, so the administrator is shown that it is an estimate rather than
// being left to assume otherwise. Getting this wrong is not a cosmetic error:
// BR-3.3 exists because a lead time computed on the wrong holidays yields a
// date the rule never legitimately allowed, and the number still looks like
// seven.

// Generated is one holiday produced by Generate.
type Generated struct {
	Date        time.Time
	Name        string
	Provisional bool
}

// Generate returns the Indonesian public holidays for a year.
//
// Fixed and Easter-derived dates come back with Provisional false. Every
// lunar-calendar date comes back Provisional TRUE, because this package
// approximates them and the decree is what counts.
func Generate(year int) []Generated {
	d := func(m time.Month, day int) time.Time {
		return time.Date(year, m, day, 0, 0, 0, 0, Jakarta)
	}
	out := []Generated{
		{d(time.January, 1), "Tahun Baru Masehi", false},
		{d(time.May, 1), "Hari Buruh Internasional", false},
		{d(time.June, 1), "Hari Lahir Pancasila", false},
		{d(time.August, 17), "Hari Kemerdekaan Republik Indonesia", false},
		{d(time.December, 25), "Hari Raya Natal", false},
	}

	// Easter is exact, so the two Christian movable feasts are too.
	easter := gregorianEaster(year)
	out = append(out,
		Generated{easter.AddDate(0, 0, -2), "Wafat Isa Almasih", false},
		Generated{easter.AddDate(0, 0, 39), "Kenaikan Isa Almasih", false},
	)

	// The Islamic dates are approximated from the tabular civil calendar,
	// which normally lands within a day of the sighting. A day is exactly the
	// margin that matters to a lead time, hence Provisional.
	for _, h := range []struct {
		month int // Hijri month
		day   int
		name  string
	}{
		{7, 27, "Isra Mikraj Nabi Muhammad SAW"},
		{10, 1, "Idul Fitri"},
		{10, 2, "Idul Fitri"},
		{12, 10, "Idul Adha"},
		{1, 1, "Tahun Baru Islam"},
		{3, 12, "Maulid Nabi Muhammad SAW"},
	} {
		for _, g := range hijriInGregorianYear(year, h.month, h.day) {
			out = append(out, Generated{g, h.name, true})
		}
	}
	return out
}

// gregorianEaster is the anonymous Gregorian computus. Exact for any year in
// the Gregorian calendar.
func gregorianEaster(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	dd := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - dd - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, Jakarta)
}

// hijriInGregorianYear returns every Gregorian date in `year` on which the
// given Hijri month/day falls. It returns a slice because a Hijri date can
// occur twice in one Gregorian year (the Hijri year is ~11 days shorter) or
// not at all.
func hijriInGregorianYear(year, hMonth, hDay int) []time.Time {
	var out []time.Time
	// The Hijri year that overlaps this Gregorian one, plus its neighbours, so
	// a date near either boundary is not missed.
	approx := int(float64(year-622)*1.030684) + 1
	for hy := approx - 1; hy <= approx+1; hy++ {
		g := hijriToGregorian(hy, hMonth, hDay)
		if g.Year() == year {
			out = append(out, g)
		}
	}
	return out
}

// hijriToGregorian converts a tabular Islamic civil date. The tabular calendar
// is arithmetic rather than observational, so it can differ from the announced
// date by a day — which is why every caller marks the result provisional.
func hijriToGregorian(hy, hm, hd int) time.Time {
	// Julian Day Number of the Hijri date, tabular civil calendar, epoch
	// Julian 16 July 622 CE.
	//
	//	JDN = (11y+3)/30 + 354y + 30m - (m-1)/2 + d + 1948440 - 385
	//
	// The month term is 30m - (m-1)/2, NOT 29(m-1) + m/2. The two look
	// interchangeable and differ by about thirty days — a whole lunar month,
	// which put Idul Fitri 2026 in February. Caught by a test that compared
	// against the announced dates rather than trusting the arithmetic.
	jdn := (11*hy+3)/30 + 354*hy + 30*hm - (hm-1)/2 + hd + 1948440 - 385
	return julianDayToTime(jdn)
}

func julianDayToTime(jdn int) time.Time {
	a := jdn + 32044
	b := (4*a + 3) / 146097
	c := a - (146097*b)/4
	dd := (4*c + 3) / 1461
	e := c - (1461*dd)/4
	m := (5*e + 2) / 153
	day := e - (153*m+2)/5 + 1
	month := m + 3 - 12*(m/10)
	year := 100*b + dd - 4800 + m/10
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, Jakarta)
}
