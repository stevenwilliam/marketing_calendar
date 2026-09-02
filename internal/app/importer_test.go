package app

import (
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
	rows, rejects, tr, tt, err := parseImport(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rejects) != 0 {
		t.Fatalf("rows=%d rejects=%d", len(rows), len(rejects))
	}
	if tr == nil || *tr != 2 {
		t.Fatal("trailer row count not read")
	}
	if tt == nil || *tt != money.IDR(227000) {
		t.Fatal("trailer total not read")
	}
	if rows[0].PromoID == nil || rows[1].PromoID != nil {
		t.Fatal("promo attribution is wrong")
	}
}

// A POS that reorders its columns must not silently shift every amount into
// the wrong field. The header is matched BY NAME.
func TestHeaderIsMatchedByNameNotPosition(t *testing.T) {
	sites, promos := fixtures()
	in := "gross_amount_idr|site_code|order_mode|business_date|sales_type|promo_code|pos_receipt_no\n" +
		"185000|MXX-001|dine_in|2026-09-01|normal||R-1\n"
	rows, rejects, _, _, err := parseImport(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rejects) != 0 {
		t.Fatalf("rows=%d rejects=%d", len(rows), len(rejects))
	}
	if rows[0].GrossIDR != 185000 {
		t.Fatalf("amount landed in the wrong field: %d", rows[0].GrossIDR)
	}
	if rows[0].ReceiptNo != "R-1" {
		t.Fatalf("receipt = %q", rows[0].ReceiptNo)
	}
}

// An unknown column is a rejected FILE: it means the POS changed its export
// and nobody told us.
func TestUnknownColumnRejectsTheWholeFile(t *testing.T) {
	sites, promos := fixtures()
	in := header + "|extra_column\nMXX-001|2026-09-01|R-1|normal||dine_in|100|x\n"
	if _, _, _, _, err := parseImport(strings.NewReader(in), sites, promos); err == nil {
		t.Fatal("an extra column must reject the file")
	}
	missing := "site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode\n"
	if _, _, _, _, err := parseImport(strings.NewReader(missing), sites, promos); err == nil {
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
	rows, rejects, _, _, err := parseImport(strings.NewReader(in), sites, promos)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("no row in this file is valid, got %d", len(rows))
	}
	want := []string{reasonUnknownSite, reasonUnknownPromo, reasonPromoInconsistent,
		reasonPromoInconsistent, reasonNegativeAmount, reasonBadAmount,
		reasonBadDate, reasonBadEnum, reasonBadEnum}
	if len(rejects) != len(want) {
		t.Fatalf("got %d rejections, want %d", len(rejects), len(want))
	}
	for i, w := range want {
		if rejects[i].Reason != w {
			t.Fatalf("row %d: reason %q, want %q", i+1, rejects[i].Reason, w)
		}
		if rejects[i].Original == "" {
			t.Fatalf("row %d: the original line must be kept (BR-6.5)", i+1)
		}
	}
}

// "185.000" is ambiguous between Indonesian thousands and a decimal point.
// Guessing divides revenue by a thousand, so it is rejected.
func TestThousandsSeparatorIsRejectedNotGuessed(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\nMXX-001|2026-09-01|R-1|normal||dine_in|185.000\n"
	rows, rejects, _, _, _ := parseImport(strings.NewReader(in), sites, promos)
	if len(rows) != 0 || len(rejects) != 1 || rejects[0].Reason != reasonBadAmount {
		t.Fatalf("rows=%d rejects=%v", len(rows), rejects)
	}
}

// A value containing a pipe survives, because the file is real RFC 4180 with
// | as the separator.
func TestQuotedPipeSurvives(t *testing.T) {
	sites, promos := fixtures()
	in := header + "\n" + `MXX-001|2026-09-01|"R|1"|normal||dine_in|100` + "\n"
	rows, rejects, _, _, err := parseImport(strings.NewReader(in), sites, promos)
	if err != nil || len(rejects) != 0 || len(rows) != 1 {
		t.Fatalf("err=%v rejects=%v rows=%d", err, rejects, len(rows))
	}
	if rows[0].ReceiptNo != "R|1" {
		t.Fatalf("receipt = %q, want %q", rows[0].ReceiptNo, "R|1")
	}
}

func TestEmptyFileIsAnError(t *testing.T) {
	sites, promos := fixtures()
	if _, _, _, _, err := parseImport(strings.NewReader(""), sites, promos); err == nil {
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
