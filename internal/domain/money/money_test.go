package money

import "testing"

// BR-1.2: floor((amount * bps + 5000) / 10000), rounded half-up.
func TestApplyBPSRoundsHalfUp(t *testing.T) {
	cases := []struct {
		name   string
		amount IDR
		bps    int64
		want   IDR
	}{
		{"11 percent of 185000", 185_000, 1100, 20_350},
		{"exact half rounds up", 5, 1000, 1}, // 5*1000+5000 = 10000 -> 1
		{"just under half rounds down", 4, 1000, 0},
		{"zero rate", 185_000, 0, 0},
		{"zero amount", 0, 1100, 0},
		{"whole rate", 185_000, 10000, 185_000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ApplyBPS(c.amount, c.bps)
			if err != nil {
				t.Fatalf("ApplyBPS(%d,%d): %v", c.amount, c.bps, err)
			}
			if got != c.want {
				t.Fatalf("ApplyBPS(%d,%d) = %d, want %d", c.amount, c.bps, got, c.want)
			}
		})
	}
}

func TestApplyBPSRefusesNegativeRate(t *testing.T) {
	if _, err := ApplyBPS(1000, -1); err != ErrNegative {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

func TestApplyBPSRefusesOverflow(t *testing.T) {
	if _, err := ApplyBPS(MaxIDR, 10000); err != ErrOverflow {
		t.Fatalf("want ErrOverflow, got %v", err)
	}
}

// A wrapped int64 is a silent corruption of a ledger, so Add refuses.
func TestAddRefusesOverflow(t *testing.T) {
	if _, err := Add(MaxIDR, MaxIDR); err != ErrOverflow {
		t.Fatalf("want ErrOverflow, got %v", err)
	}
	if _, err := Add(MaxIDR, 1); err != ErrOverflow {
		t.Fatalf("MaxIDR+1 must overflow, got %v", err)
	}
	// The boundary itself is legal: an off-by-one here rejects a valid amount.
	if got, err := Add(MaxIDR-1, 1); err != nil || got != MaxIDR {
		t.Fatalf("MaxIDR-1+1 = %d, %v; want MaxIDR, nil", got, err)
	}
}

func TestSubRefusesOverflow(t *testing.T) {
	if _, err := Sub(-MaxIDR, 1); err != ErrOverflow {
		t.Fatalf("want ErrOverflow, got %v", err)
	}
	if got, err := Sub(1_000, 400); err != nil || got != 600 {
		t.Fatalf("Sub = %d, %v", got, err)
	}
}

func TestSum(t *testing.T) {
	got, err := Sum(1_000, 2_500, 300)
	if err != nil || got != 3_800 {
		t.Fatalf("Sum = %d, %v; want 3800, nil", got, err)
	}
}

// BR-6.6 reject, never silently repair. "185.000" is ambiguous between
// Indonesian thousands and a decimal point; guessing divides revenue by 1000.
func TestParseRejectsSeparatorsAndDecimals(t *testing.T) {
	for _, s := range []string{"185.000", "185,000", "185000.00", "Rp 185000", "185 000", "1e5", ""} {
		if _, err := Parse(s); err == nil {
			t.Fatalf("Parse(%q) succeeded; it must be rejected", s)
		}
	}
}

func TestParseAcceptsWholeRupiah(t *testing.T) {
	got, err := Parse("185000")
	if err != nil || got != 185_000 {
		t.Fatalf("Parse = %d, %v", got, err)
	}
}

func TestParseRejectsNegative(t *testing.T) {
	if _, err := Parse("-1"); err != ErrNegative {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

func TestFormatUsesIndonesianGrouping(t *testing.T) {
	cases := map[IDR]string{0: "0", 1: "1", 999: "999", 1_000: "1.000",
		1_234_567: "1.234.567", 1_000_000_000: "1.000.000.000"}
	for in, want := range cases {
		if got := in.Format(); got != want {
			t.Fatalf("IDR(%d).Format() = %q, want %q", in, got, want)
		}
	}
}

// A percentage of a zero target is undefined, not zero — reporting 0% would
// read as "on target" for a site with no target set.
func TestPercentOfZeroTargetIsUndefined(t *testing.T) {
	if _, ok := PercentOfBPS(500, 0); ok {
		t.Fatal("a variance against a zero target must be reported as undefined")
	}
	bps, ok := PercentOfBPS(1_500, 1_000)
	if !ok || bps != 15000 {
		t.Fatalf("PercentOfBPS(1500,1000) = %d, %v; want 15000, true", bps, ok)
	}
}
