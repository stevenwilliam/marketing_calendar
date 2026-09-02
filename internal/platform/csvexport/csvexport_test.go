package csvexport

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

// BR-7.3: a CSV is an executable document in Excel.
func TestFormulaInjectionIsNeutralised(t *testing.T) {
	cases := map[string]string{
		`=1+1`:               `'=1+1`,
		`+1`:                 `'+1`,
		`-1`:                 `'-1`,
		`@SUM(A1)`:           `'@SUM(A1)`,
		"\tx":                "'\tx",
		"\rx":                "'\rx",
		`=cmd|' /C calc'!A0`: `'=cmd|' /C calc'!A0`,
		`Kopi Sore Hemat`:    `Kopi Sore Hemat`,
		``:                   ``,
		`1-2`:                `1-2`, // dangerous only in FIRST position
	}
	for in, want := range cases {
		if got := Guard(in); got != want {
			t.Fatalf("Guard(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every cell goes through Guard, not a remembered subset.
func TestWriterGuardsEveryColumn(t *testing.T) {
	var b bytes.Buffer
	w := New(&b)
	w.Write([]string{"aman", "=1+1", "juga aman", "@evil"})
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "'=1+1") || !strings.Contains(out, "'@evil") {
		t.Fatalf("a column was not guarded: %q", out)
	}
}

// BR-7.2: a real RFC 4180 file with | as the separator. A value containing a
// pipe, a quote or a newline must survive the round trip.
func TestPipeDelimitedRoundTrip(t *testing.T) {
	rows := [][]string{
		{"kode", "nama", "catatan"},
		{"MXX-001", "Maxx | Plaza", "baris satu\nbaris dua"},
		{"MXX-002", `Kutipan "ganda"`, "koma, tetap aman"},
	}
	var b bytes.Buffer
	w := New(&b)
	for _, r := range rows {
		w.Write(r)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	body := bytes.TrimPrefix(b.Bytes(), []byte{0xEF, 0xBB, 0xBF})
	r := csv.NewReader(bytes.NewReader(body))
	r.Comma = Delimiter
	got, err := r.ReadAll()
	if err != nil {
		t.Fatalf("the file we wrote does not parse as CSV: %v", err)
	}
	if len(got) != len(rows) {
		t.Fatalf("read %d rows, wrote %d", len(got), len(rows))
	}
	for i := range rows {
		for j := range rows[i] {
			if got[i][j] != rows[i][j] {
				t.Fatalf("row %d col %d: got %q, want %q", i, j, got[i][j], rows[i][j])
			}
		}
	}
}

// The separator is a pipe and a comma is just data.
func TestCommaIsNotASeparator(t *testing.T) {
	var b bytes.Buffer
	w := New(&b)
	w.Write([]string{"a,b", "c"})
	_ = w.Flush()
	line := strings.TrimSpace(strings.TrimPrefix(b.String(), "\xef\xbb\xbf"))
	if line != "a,b|c" {
		t.Fatalf("got %q, want %q", line, "a,b|c")
	}
}

// Money is plain digits so the cell sums in a spreadsheet.
func TestMoneyHasNoSeparators(t *testing.T) {
	if got := Money(1_234_567); got != "1234567" {
		t.Fatalf("Money = %q, want 1234567", got)
	}
	if got := Money(0); got != "0" {
		t.Fatalf("Money(0) = %q", got)
	}
	if got := Money(-5); got != "-5" {
		t.Fatalf("Money(-5) = %q", got)
	}
}
