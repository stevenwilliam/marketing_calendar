// Package money is whole Indonesian rupiah as int64, and nothing else.
//
// BR-1.1: floating point is prohibited in any code path touching money. This
// package contains no float64 at all, so the prohibition is structural rather
// than a rule someone has to remember.
package money

import (
	"errors"
	"strconv"
	"strings"
)

// IDR is a whole rupiah amount. Sen is not represented (D7).
type IDR int64

var (
	ErrNegative = errors.New("jumlah tidak boleh negatif")
	ErrOverflow = errors.New("jumlah di luar jangkauan")
	ErrSyntax   = errors.New("jumlah tidak valid")
)

func (a IDR) Int64() int64 { return int64(a) }

// MaxIDR bounds every amount in the system. It is deliberately below
// math.MaxInt64 so that arithmetic has headroom to DETECT an overflow instead
// of wrapping into it: a check written as "did the sign flip?" cannot fire
// when both operands are already near the int64 ceiling, and a wrapped int64
// is a silent corruption of a ledger. The bound is checked, not inferred.
const MaxIDR = IDR(1<<62 - 1)

// Add returns a+b, refusing any result outside [-MaxIDR, MaxIDR].
func Add(a, b IDR) (IDR, error) {
	if b > 0 && a > MaxIDR-b {
		return 0, ErrOverflow
	}
	if b < 0 && a < -MaxIDR-b {
		return 0, ErrOverflow
	}
	return a + b, nil
}

func Sub(a, b IDR) (IDR, error) {
	if b < 0 && a > MaxIDR+b {
		return 0, ErrOverflow
	}
	if b > 0 && a < -MaxIDR+b {
		return 0, ErrOverflow
	}
	return a - b, nil
}

// Sum adds many amounts, refusing overflow at every step.
func Sum(xs ...IDR) (IDR, error) {
	var total IDR
	var err error
	for _, x := range xs {
		if total, err = Add(total, x); err != nil {
			return 0, err
		}
	}
	return total, nil
}

// ApplyBPS applies a rate in basis points, rounded half-up:
//
//	floor((amount * bps + 5000) / 10000)      (BR-1.2)
//
// 11% is bps=1100. Integer arithmetic throughout; no float appears.
func ApplyBPS(amount IDR, bps int64) (IDR, error) {
	if bps < 0 {
		return 0, ErrNegative
	}
	if amount != 0 && bps != 0 {
		// Overflow guard before multiplying.
		if int64(amount) > (int64(MaxIDR)-5000)/bps {
			return 0, ErrOverflow
		}
	}
	n := int64(amount)*bps + 5000
	return IDR(n / 10000), nil
}

// PercentOfBPS is the variance of actual against target in basis points.
// Returns ok=false when the target is zero: a percentage of nothing is not
// zero percent, it is undefined, and reporting 0% would read as "on target".
func PercentOfBPS(actual, target IDR) (bps int64, ok bool) {
	if target == 0 {
		return 0, false
	}
	return (int64(actual)*10000 + int64(target)/2) / int64(target), true
}

// Parse reads a whole-rupiah string. It rejects decimals, separators and
// currency symbols outright (BR-6.6, reject never repair): "185.000" is
// ambiguous between Indonesian thousands and a decimal point, and guessing is
// how an import silently divides revenue by a thousand.
func Parse(s string) (IDR, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrSyntax
	}
	for _, r := range s {
		if r == '-' || (r >= '0' && r <= '9') {
			continue
		}
		return 0, ErrSyntax
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, ErrSyntax
	}
	if n < 0 {
		return 0, ErrNegative
	}
	return IDR(n), nil
}

// Format renders 1234567 as "1.234.567" — Indonesian grouping, no decimals.
func (a IDR) Format() string {
	n := int64(a)
	neg := n < 0
	if neg {
		n = -n
	}
	d := strconv.FormatInt(n, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, c := range d {
		if i > 0 && (len(d)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (a IDR) String() string { return "Rp " + a.Format() }
