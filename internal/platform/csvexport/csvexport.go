// Package csvexport writes RFC 4180 CSV with a PIPE separator.
//
// CLAUDE.md §7: the delimiter is a pipe, never a comma. It is still a real
// RFC 4180 file — a value containing a pipe, a quote or a newline survives the
// round trip — and every cell is guarded against formula injection, because a
// CSV is an executable document in Excel (BR-7.2, BR-7.3).
package csvexport

import (
	"encoding/csv"
	"io"
	"strings"
	"time"

	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
)

// Delimiter is the pipe. It is not configurable: one delimiter across the
// product is the point.
const Delimiter = '|'

// dangerous are the leading characters Excel and LibreOffice treat as the
// start of a formula.
const dangerous = "=+-@\t\r"

// Guard neutralises formula injection by prefixing an apostrophe. It is
// applied to EVERY cell on the way out, not to a list of fields someone
// remembered — the one field nobody thought about is the one that carries the
// payload.
func Guard(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune(dangerous, rune(s[0])) {
		return "'" + s
	}
	return s
}

// Writer emits a pipe-delimited CSV. Every value passes through Guard.
type Writer struct {
	w   *csv.Writer
	err error
}

func New(w io.Writer) *Writer {
	// A UTF-8 BOM so Excel on Windows opens Indonesian text correctly rather
	// than as mojibake. Without it "Kopi Sore Hemat" survives but a rupiah
	// sign or an accented site name does not.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	c := csv.NewWriter(w)
	c.Comma = Delimiter
	return &Writer{w: c}
}

func (x *Writer) Write(record []string) {
	if x.err != nil {
		return
	}
	out := make([]string, len(record))
	for i, v := range record {
		out[i] = Guard(v)
	}
	x.err = x.w.Write(out)
}

func (x *Writer) Flush() error {
	x.w.Flush()
	if x.err != nil {
		return x.err
	}
	return x.w.Error()
}

// Helpers so a caller never formats money or dates by hand at the call site.

// Money renders whole rupiah as plain digits — no separators, so the cell is
// a number in the spreadsheet rather than text that cannot be summed.
func Money(a money.IDR) string { return itoa(int64(a)) }

func Int(n int) string { return itoa(int64(n)) }

func Date(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
}

func Timestamp(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// Filename builds a download name: mc_<report>_<yyyymmdd-hhmmss>.csv
func Filename(report string, now time.Time, loc *time.Location) string {
	return "mc_" + report + "_" + now.In(loc).Format("20060102-150405") + ".csv"
}
