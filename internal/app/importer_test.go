package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
)

func fixtures() (map[string]Site, map[string]uuid.UUID) {
	company := uuid.New()
	sites := map[string]Site{
		"MXX-001": {SiteID: uuid.New(), CompanyID: company, SiteCode: "MXX-001", SiteType: "coffee_shop"},
	}
	promos := map[string]uuid.UUID{"PRM-7QK2": uuid.New()}
	return sites, promos
}

const header = "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr"

func TestParseHappyPath(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\n" +
		"MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000\n" +
		"MXX-001|2026-09-01|R-000198232|normal||take_away|42000\n" +
		"#TOTAL|2|227000\n"
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Txns) != 2 || len(got.Rejections) != 0 {
		t.Fatalf("rows=%d got.Rejections=%d", len(got.Txns), len(got.Rejections))
	}
	if got.TrailerRows == nil || *got.TrailerRows != 2 {
		t.Fatal("trailer row count not read")
	}
	if got.TrailerTotal == nil || *got.TrailerTotal != money.IDR(227000) {
		t.Fatal("trailer total not read")
	}
	if got.Txns[0].PromoID == nil || got.Txns[1].PromoID != nil {
		t.Fatal("promo attribution is wrong")
	}
}

// A POS that reorders its columns must not silently shift every amount into
// the wrong field. The header is matched BY NAME.
func TestHeaderIsMatchedByNameNotPosition(t *testing.T) {
	sites, promos := fixtures()
	in := "gross_amount_idr|site_code|order_mode|business_date|sales_type|promo_code|pos_receipt_no\n" +
		"185000|MXX-001|dine_in|2026-09-01|normal||R-1\n"
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Txns) != 1 || len(got.Rejections) != 0 {
		t.Fatalf("rows=%d got.Rejections=%d", len(got.Txns), len(got.Rejections))
	}
	if got.Txns[0].GrossIDR != 185000 {
		t.Fatalf("amount landed in the wrong field: %d", got.Txns[0].GrossIDR)
	}
	if got.Txns[0].ReceiptNo != "R-1" {
		t.Fatalf("receipt = %q", got.Txns[0].ReceiptNo)
	}
}

// An unknown column is a rejected FILE: it means the POS changed its export
// and nobody told us.
func TestUnknownColumnRejectsTheWholeFile(t *testing.T) {
	sites, promos := fixtures()
	in := header + "|extra_column\nMXX-001|2026-09-01|R-1|normal||dine_in|100|x\n"
	if _, err := parseFile(strings.NewReader(in), sites, promos); err == nil {
		t.Fatal("an extra column must reject the file")
	}
	missing := "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode\n"
	if _, err := parseFile(strings.NewReader(missing), sites, promos); err == nil {
		t.Fatal("a missing column must reject the file")
	}
}

// BR-6.5/6.6: every bad row is rejected with its reason, never coerced.
func TestRejectionReasons(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\n" +
		"NOPE-001|2026-09-01|R-1|normal||dine_in|100\n" + // UNKNOWN_SITE
		"MXX-001|2026-09-01|R-2|promo|NOT-A-PROMO|dine_in|100\n" + // UNKNOWN_PROMO
		"MXX-001|2026-09-01|R-3|promo||dine_in|100\n" + // PROMO_INCONSISTENT
		"MXX-001|2026-09-01|R-4|normal|PRM-7QK2|dine_in|100\n" + // PROMO_INCONSISTENT
		"MXX-001|2026-09-01|R-5|normal||dine_in|-5\n" + // NEGATIVE_AMOUNT
		"MXX-001|2026-09-01|R-6|normal||dine_in|185.000\n" + // BAD_AMOUNT
		"MXX-001|not-a-date|R-7|normal||dine_in|100\n" + // BAD_DATE
		"MXX-001|2026-09-01|R-8|both||dine_in|100\n" + // BAD_ENUM
		"MXX-001|2026-09-01|R-9|normal||delivery|100\n" // BAD_ENUM
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Txns) != 0 {
		t.Fatalf("no row in this file is valid, got %d", len(got.Txns))
	}
	want := []string{reasonUnknownSite, reasonUnknownPromo, reasonPromoInconsistent,
		reasonPromoInconsistent, reasonNegativeAmount, reasonBadAmount,
		reasonBadDate, reasonBadEnum, reasonBadEnum}
	if len(got.Rejections) != len(want) {
		t.Fatalf("got %d rejections, want %d", len(got.Rejections), len(want))
	}
	for i, w := range want {
		if got.Rejections[i].Reason != w {
			t.Fatalf("row %d: reason %q, want %q", i+1, got.Rejections[i].Reason, w)
		}
		if got.Rejections[i].Original == "" {
			t.Fatalf("row %d: the original line must be kept (BR-6.5)", i+1)
		}
	}
}

// "185.000" is ambiguous between Indonesian thousands and a decimal point.
// Guessing divides revenue by a thousand, so it is rejected.
func TestThousandsSeparatorIsRejectedNotGuessed(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\nMXX-001|2026-09-01|R-1|normal||dine_in|185.000\n"
	got, _ := parseFile(strings.NewReader(in), sites, promos)
	if len(got.Txns) != 0 || len(got.Rejections) != 1 || got.Rejections[0].Reason != reasonBadAmount {
		t.Fatalf("rows=%d got.Rejections=%v", len(got.Txns), got.Rejections)
	}
}

// A value containing a pipe survives, because the file is real RFC 4180 with
// | as the separator.
func TestQuotedPipeSurvives(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\n" + `MXX-001|2026-09-01|"R|1"|normal||dine_in|100` + "\n"
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil || len(got.Rejections) != 0 || len(got.Txns) != 1 {
		t.Fatalf("err=%v got.Rejections=%v rows=%d", err, got.Rejections, len(got.Txns))
	}
	if got.Txns[0].ReceiptNo != "R|1" {
		t.Fatalf("receipt = %q, want %q", got.Txns[0].ReceiptNo, "R|1")
	}
}

func TestEmptyFileIsAnError(t *testing.T) {
	sites, promos := fixtures()
	if _, err := parseFile(strings.NewReader(""), sites, promos); err == nil {
		t.Fatal("an empty file must be an error, not a silent zero-row success")
	}
}

func TestSplitPipeHandlesEscapedQuotes(t *testing.T) {
	got := splitPipe(`a|"b|c"|"d""e"`)
	want := []string{"a", "b|c", `d"e`}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- the target contracts (D47) -------------------------------------------

// The kind is detected from the HEADER, never the filename, because the drop
// directory is unattended and a file renamed by hand must not change how it is
// parsed.
func TestKindIsDetectedFromTheHeader(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   ImportKind
	}{
		{"transactions", "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr", KindTransactions},
		{"yearly target", "site_code|year|sales_type|target_amount_idr", KindTargetYear},
		{"monthly target", "site_code|year|month|sales_type|target_amount_idr", KindTargetMonth},
		{"reordered monthly", "target_amount_idr|month|sales_type|year|site_code", KindTargetMonth},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			index := map[string]int{}
			for i, h := range splitPipe(c.header) {
				index[h] = i
			}
			got, err := detectKind(index)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("detected %q, want %q", got, c.want)
			}
		})
	}
}

// A monthly file carries `year` as well, so the narrower shape has to be
// tested first. Getting this backwards would load every monthly target as a
// yearly one and silently overwrite twelve rows with one.
func TestMonthlyIsNotMistakenForYearly(t *testing.T) {
	index := map[string]int{"site_code": 0, "year": 1, "month": 2, "sales_type": 3, "target_amount_idr": 4}
	got, err := detectKind(index)
	if err != nil {
		t.Fatal(err)
	}
	if got != KindTargetMonth {
		t.Fatalf("a file with a month column was detected as %q", got)
	}
}

func TestUnknownShapeIsRejected(t *testing.T) {
	if _, err := detectKind(map[string]int{"kolom_a": 0, "kolom_b": 1}); err == nil {
		t.Fatal("a file matching no contract must be rejected, not guessed at")
	}
}

func TestParseYearlyTargets(t *testing.T) {
	sites, promos := fixtures()
	in := "site_code|year|sales_type|target_amount_idr\n" +
		"MXX-001|2026|normal|9690000000\n" +
		"MXX-001|2026|promo|969000000\n" +
		"#TOTAL|2|10659000000\n"
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindTargetYear {
		t.Fatalf("kind = %s", got.Kind)
	}
	if len(got.Targets) != 2 || len(got.Rejections) != 0 {
		t.Fatalf("targets=%d got.Rejections=%v", len(got.Targets), got.Rejections)
	}
	if got.Targets[0].PeriodKind != "YEAR" || got.Targets[0].Month != 0 {
		t.Fatalf("a yearly row must carry no month: %+v", got.Targets[0])
	}
	if got.Sum() != money.IDR(10_659_000_000) {
		t.Fatalf("sum = %d", got.Sum())
	}
}

func TestParseMonthlyTargetsAndTheirRejections(t *testing.T) {
	sites, promos := fixtures()
	in := "site_code|year|month|sales_type|target_amount_idr\n" +
		"MXX-001|2026|1|normal|850000000\n" +
		"MXX-001|2026|13|normal|850000000\n" + // BAD_MONTH
		"MXX-001|1999|2|normal|850000000\n" + // BAD_YEAR
		"MXX-001|2026|3|normal|850.000.000\n" + // BAD_AMOUNT, not silently repaired
		"MXX-001|2026|4|both|850000000\n" + // BAD_ENUM
		"NOPE-1|2026|5|normal|850000000\n" // UNKNOWN_SITE
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("only the first row is valid, got %d", len(got.Targets))
	}
	want := []string{reasonBadMonth, reasonBadYear, reasonBadAmount, reasonBadEnum, reasonUnknownSite}
	if len(got.Rejections) != len(want) {
		t.Fatalf("got %d rejections, want %d: %+v", len(got.Rejections), len(want), got.Rejections)
	}
	for i, w := range want {
		if got.Rejections[i].Reason != w {
			t.Fatalf("rejection %d: %q, want %q", i, got.Rejections[i].Reason, w)
		}
		if got.Rejections[i].Original == "" {
			t.Fatalf("rejection %d lost its original line", i)
		}
	}
}

// A file that contradicts itself is a mistake worth naming. The database would
// upsert it and the last line would silently win.
func TestSameTargetTwiceInOneFileIsRejected(t *testing.T) {
	sites, promos := fixtures()
	in := "site_code|year|sales_type|target_amount_idr\n" +
		"MXX-001|2026|normal|100000000\n" +
		"MXX-001|2026|normal|200000000\n"
	got, err := parseFile(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("the first occurrence should load, got %d", len(got.Targets))
	}
	if len(got.Rejections) != 1 || !strings.HasPrefix(got.Rejections[0].Reason, reasonDuplicateTarget) {
		t.Fatalf("the second occurrence must be rejected: %+v", got.Rejections)
	}
	// The same target in a DIFFERENT file is fine — that is a correction, and
	// the upsert is exactly how a corrected file is meant to work.
	again, err := parseFile(strings.NewReader(
		"site_code|year|sales_type|target_amount_idr\nMXX-001|2026|normal|300000000\n"), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Rejections) != 0 {
		t.Fatal("a correction in a separate file must not be treated as a duplicate")
	}
}

// BR-2.3 must survive the bulk path. This is the most natural place in the
// whole product to break it by being helpful.
func TestImporterDoesNotEnforceMonthsSummingToYear(t *testing.T) {
	sites, promos := fixtures()
	// A year of 1.2bn and twelve months of 112.5m each, summing to 1.35bn.
	var b strings.Builder
	b.WriteString("site_code|year|month|sales_type|target_amount_idr\n")
	for m := 1; m <= 12; m++ {
		fmt.Fprintf(&b, "MXX-001|2026|%d|normal|112500000\n", m)
	}
	got, err := parseFile(strings.NewReader(b.String()), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 12 || len(got.Rejections) != 0 {
		t.Fatalf("all twelve months must load: %d targets, %v", len(got.Targets), got.Rejections)
	}
}

// Every template must parse back through the parser that will read the file
// the user produces from it. A template that does not round-trip is a trap.
func TestEveryTemplateParsesAsItsOwnKind(t *testing.T) {
	sites := map[string]Site{
		"MXX-001": {SiteID: uuid.New(), CompanyID: uuid.New(), SiteCode: "MXX-001"},
	}
	promos := map[string]uuid.UUID{"PRM-7QK2": uuid.New()}
	for _, kind := range []ImportKind{KindTransactions, KindTargetYear, KindTargetMonth} {
		t.Run(string(kind), func(t *testing.T) {
			name, body := Template(kind)
			if name == "" || body == "" {
				t.Fatal("empty template")
			}
			got, err := parseFile(strings.NewReader(body), sites, promos)
			if err != nil {
				t.Fatalf("the template does not parse: %v", err)
			}
			if got.Kind != kind {
				t.Fatalf("the %s template is detected as %s", kind, got.Kind)
			}
			if len(got.Rejections) != 0 {
				t.Fatalf("the template's own example rows were rejected: %+v", got.Rejections)
			}
			// And its trailer must agree with its got.Txns, or it teaches the
			// wrong lesson the first time somebody edits it.
			if got.TrailerRows == nil || *got.TrailerRows != got.RowsRead() {
				t.Fatalf("trailer row count %v does not match %d rows", got.TrailerRows, got.RowsRead())
			}
			if got.TrailerTotal == nil || *got.TrailerTotal != got.Sum() {
				t.Fatalf("trailer total %v does not match the rows' sum %d", got.TrailerTotal, got.Sum())
			}
		})
	}
}
