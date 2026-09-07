package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

// ImportKind is what a file turns out to be. It is DETECTED FROM THE HEADER,
// never from the filename: a file renamed by hand must not change how it is
// parsed, and the drop directory is unattended (D47).
type ImportKind string

const (
	KindTransactions ImportKind = "transactions"
	KindTargetYear   ImportKind = "target_year"
	KindTargetMonth  ImportKind = "target_month"
	KindHoliday      ImportKind = "holiday"
)

// The CSV contracts of 06-domain-operations.md §3.0 (D30, extended by D47).
// Pipe-delimited, the same convention as every export the product produces.
var importColumns = map[ImportKind][]string{
	KindTransactions: {
		"site_code", "business_date", "pos_receipt_no",
		"sales_type", "promo_code", "order_mode", "gross_amount_idr",
	},
	KindTargetYear:  {"site_code", "year", "sales_type", "target_amount_idr"},
	KindTargetMonth: {"site_code", "year", "month", "sales_type", "target_amount_idr"},
	KindHoliday:     {"holiday_date", "holiday_name", "is_provisional"},
}

// KindLabel is what the reconciliation screen calls each kind.
var KindLabel = map[ImportKind]string{
	KindTransactions: "Transaksi",
	KindTargetYear:   "Target tahunan",
	KindTargetMonth:  "Target bulanan",
	KindHoliday:      "Hari libur",
}

// detectKind reads the header. The three contracts are distinguishable by the
// columns alone, which is what lets one drop directory carry all of them.
//
// The order of these tests matters: a monthly target has `year` too, so the
// narrower shape has to be checked first. Getting that backwards would load
// every monthly target as a yearly one and silently overwrite twelve rows with
// one.
func detectKind(header map[string]int) (ImportKind, error) {
	has := func(names ...string) bool {
		for _, n := range names {
			if _, ok := header[n]; !ok {
				return false
			}
		}
		return true
	}
	switch {
	case has("holiday_date", "holiday_name"):
		return KindHoliday, nil
	case has("pos_receipt_no", "business_date"):
		return KindTransactions, nil
	case has("month", "year", "target_amount_idr"):
		return KindTargetMonth, nil
	case has("year", "target_amount_idr"):
		return KindTargetYear, nil
	default:
		return "", errors.New(
			"berkas tidak cocok dengan kontrak mana pun: butuh pos_receipt_no (transaksi), " +
				"year + target_amount_idr (target tahunan), year + month + target_amount_idr " +
				"(target bulanan), atau holiday_date + holiday_name (hari libur)")
	}
}

// Template returns a ready-to-fill example for a kind: the header, one row of
// plausible data, and the trailer. Handing somebody a template is cheaper than
// handing them a specification and hoping.
func Template(kind ImportKind) (filename, body string) {
	switch kind {
	case KindTargetYear:
		return "template_target_tahunan.csv",
			"site_code|year|sales_type|target_amount_idr\n" +
				"MXX-001|2026|normal|9690000000\n" +
				"MXX-001|2026|promo|969000000\n" +
				"#TOTAL|2|10659000000\n"
	case KindTargetMonth:
		return "template_target_bulanan.csv",
			"site_code|year|month|sales_type|target_amount_idr\n" +
				"MXX-001|2026|1|normal|850000000\n" +
				"MXX-001|2026|1|promo|85000000\n" +
				"MXX-001|2026|2|normal|850000000\n" +
				"#TOTAL|3|1785000000\n"
	case KindHoliday:
		return "template_hari_libur.csv",
			"holiday_date|holiday_name|is_provisional\n" +
				"2028-01-01|Tahun Baru Masehi|false\n" +
				"2028-08-17|Hari Kemerdekaan Republik Indonesia|false\n" +
				"2028-12-25|Hari Raya Natal|false\n" +
				"#TOTAL|3\n"
	default:
		return "template_transaksi.csv",
			"site_code|business_date|pos_receipt_no|sales_type|promo_code|order_mode|gross_amount_idr\n" +
				"MXX-001|2026-09-01|R-000198231|promo|PRM-7QK2|dine_in|185000\n" +
				"MXX-001|2026-09-01|R-000198232|normal||take_away|42000\n" +
				"#TOTAL|2|227000\n"
	}
}

// Rejection reasons, as documented in 06 §3.3.
const (
	reasonUnknownSite       = "UNKNOWN_SITE"
	reasonUnknownPromo      = "UNKNOWN_PROMO"
	reasonPromoInconsistent = "PROMO_INCONSISTENT"
	reasonNegativeAmount    = "NEGATIVE_AMOUNT"
	reasonBadDate           = "BAD_DATE"
	reasonBadAmount         = "BAD_AMOUNT"
	reasonBadEnum           = "BAD_ENUM"
	reasonShortRow          = "SHORT_ROW"
	reasonBlankReceipt      = "BLANK_RECEIPT"
	reasonBadYear           = "BAD_YEAR"
	reasonBadMonth          = "BAD_MONTH"
	reasonDuplicateTarget   = "DUPLICATE_IN_FILE"
	reasonBlankName         = "BLANK_NAME"
	reasonBadBool           = "BAD_BOOLEAN"
)

type ImportResult struct {
	Run        ImportRun   `json:"run"`
	Rejections []Rejection `json:"rejections,omitempty"`
	Skipped    bool        `json:"skipped"`
}

// ImportFile parses and loads one file.
//
// Order matters: the checksum is computed and checked BEFORE parsing, so a
// file already loaded costs one query rather than a full parse (BR-6.3).
func (d *Deps) ImportFile(ctx context.Context, path string, actor *uuid.UUID) (*ImportResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	checksum := hex.EncodeToString(sum[:])

	seen, err := d.Facts.SeenChecksum(ctx, checksum)
	if err != nil {
		return nil, err
	}
	if seen {
		// BR-6.3: re-importing an identical file reports zero and is NOT an
		// error. Treating it as one makes the nightly job alert every time a
		// file is left in the drop directory.
		return &ImportResult{Skipped: true, Run: ImportRun{
			FileName: filepath.Base(path), Checksum: checksum, Kind: string(KindTransactions),
			Outcome: "SKIPPED", Message: "berkas dengan checksum ini sudah pernah dimuat"}}, nil
	}

	sites, err := d.Facts.SiteCodeIndex(ctx)
	if err != nil {
		return nil, err
	}
	promos, err := d.Facts.PlanCodeIndex(ctx)
	if err != nil {
		return nil, err
	}

	parsed, parseErr := parseFile(strings.NewReader(string(raw)), sites, promos)
	if parseErr != nil {
		// The message IS the cause here; wrapping with both would print it
		// twice in the operator's log line.
		return nil, apierror.New(apierror.CodeValidation, parseErr.Error())
	}
	rejects := parsed.Rejections

	run := ImportRun{FileName: filepath.Base(path), Checksum: checksum,
		Kind: string(parsed.Kind), RowsRead: parsed.RowsRead(),
		TrailerRows: parsed.TrailerRows, TrailerTotal: parsed.TrailerTotal}

	// The row count is what catches a truncated upload, which otherwise
	// arrives looking exactly like a quiet day of trading.
	if parsed.TrailerRows != nil && *parsed.TrailerRows != run.RowsRead {
		run.Outcome = "FAILED"
		run.Message = fmt.Sprintf("trailer menyatakan %d baris, berkas berisi %d — berkas terpotong?",
			*parsed.TrailerRows, run.RowsRead)
		saved, err := d.Facts.LoadFile(ctx, run, nil, rejects, actor)
		return &ImportResult{Run: saved, Rejections: rejects}, err
	}
	// The TOTAL is only enforced when nothing was rejected.
	//
	// The row count above is what catches truncation. The total is a second
	// opinion, meaningful only when every row loaded: a file with one bad line
	// has a legitimately smaller sum than its trailer states, and failing the
	// whole file for it would lose the load to guard something already
	// guarded.
	loadedIDR := parsed.Sum()
	if parsed.TrailerTotal != nil && len(rejects) == 0 && loadedIDR != *parsed.TrailerTotal {
		run.Outcome = "FAILED"
		run.Message = fmt.Sprintf("trailer menyatakan total Rp %s, berkas berjumlah Rp %s",
			parsed.TrailerTotal.Format(), loadedIDR.Format())
		saved, err := d.Facts.LoadFile(ctx, run, nil, rejects, actor)
		return &ImportResult{Run: saved, Rejections: rejects}, err
	}

	run.Outcome = "OK"
	if len(rejects) > 0 {
		run.Outcome = "PARTIAL"
		run.Message = fmt.Sprintf("%d baris ditolak", len(rejects))
		if parsed.TrailerTotal != nil && loadedIDR != *parsed.TrailerTotal {
			// Said plainly rather than hidden: the reconciliation screen must
			// show why the loaded total is below the file's own.
			run.Message += fmt.Sprintf("; total dimuat Rp %s dari Rp %s pada trailer — selisihnya adalah baris yang ditolak",
				loadedIDR.Format(), parsed.TrailerTotal.Format())
		}
	}

	var saved ImportRun
	switch parsed.Kind {
	case KindTransactions:
		saved, err = d.Facts.LoadFile(ctx, run, parsed.Txns, rejects, actor)
	case KindHoliday:
		// Holidays upsert on (country, date). Re-importing a corrected decree
		// overwrites the estimate it replaces, which is the whole point.
		saved, err = d.Facts.LoadHolidays(ctx, run, parsed.Holidays, rejects, actor)
	default:
		// Targets are upserted by the (site, period, sales_type) unique index,
		// so re-importing a corrected target file overwrites rather than
		// duplicating — the same idempotency the transaction path gets from
		// its receipt key.
		saved, err = d.Facts.LoadTargets(ctx, run, parsed.Targets, rejects, actor)
	}
	if err != nil {
		return nil, err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: actor, Action: "import.run",
		SubjectType: "import_run", SubjectID: &saved.ImportRunID,
		After: map[string]any{"file": saved.FileName, "kind": saved.Kind,
			"inserted": saved.RowsInserted, "skipped": saved.RowsSkipped,
			"rejected": saved.RowsRejected}})
	return &ImportResult{Run: saved, Rejections: rejects}, nil
}

// parseImport reads the contract. The header is matched BY NAME, not by
// position: a POS that reorders its columns must not silently shift every
// amount into the wrong field.
type parsedFile struct {
	Kind         ImportKind
	Txns         []TxnRow
	Targets      []TargetRow
	Holidays     []Holiday
	Rejections   []Rejection
	TrailerRows  *int
	TrailerTotal *money.IDR
}

// RowsRead is every line the file offered, valid or not. It is what the
// trailer's row count is compared against, because a truncated file is short
// regardless of how many of its rows were any good.
func (p parsedFile) RowsRead() int {
	return len(p.Txns) + len(p.Targets) + len(p.Holidays) + len(p.Rejections)
}

// Sum is the rupiah total of the rows that parsed, for the trailer's second
// check.
func (p parsedFile) Sum() money.IDR {
	var s money.IDR
	for _, r := range p.Txns {
		s += r.GrossIDR
	}
	for _, r := range p.Targets {
		s += r.AmountIDR
	}
	return s
}

// parseFile reads any of the three contracts. The header decides which, and
// the header is matched BY NAME, not by position: a source that reorders its
// columns must not silently shift every value into the wrong field.
func parseFile(r io.Reader, sites map[string]Site, promos map[string]uuid.UUID) (parsedFile, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	var out parsedFile
	var header []string
	var index map[string]int
	var kind ImportKind
	// seen catches the same target addressed twice in ONE file. The database
	// would upsert it and the last line would silently win; a file that
	// contradicts itself is a mistake worth naming.
	seen := map[string]int{}

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "#TOTAL") {
			// `#TOTAL|<rows>` or `#TOTAL|<rows>|<rupiah>`. The rupiah half is
			// optional because a holiday file has no money in it, and
			// demanding a "0" there would be a column that means nothing.
			parts := strings.Split(line, "|")
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					out.TrailerRows = &n
				}
			}
			if len(parts) >= 3 {
				if v, err := money.Parse(strings.TrimSpace(parts[2])); err == nil {
					out.TrailerTotal = &v
				}
			}
			continue
		}
		fields := splitPipe(line)

		if header == nil {
			header = fields
			index = map[string]int{}
			for i, h := range fields {
				index[strings.ToLower(strings.TrimSpace(h))] = i
			}
			k, err := detectKind(index)
			if err != nil {
				return out, err
			}
			kind = k
			out.Kind = k
			var missing []string
			for _, want := range importColumns[kind] {
				if _, ok := index[want]; !ok {
					missing = append(missing, want)
				}
			}
			if len(missing) > 0 {
				return out, fmt.Errorf("berkas %s tidak sesuai kontrak: kolom hilang %s",
					KindLabel[kind], strings.Join(missing, ", "))
			}
			// An unknown column rejects the FILE rather than being ignored: it
			// means the source changed its export and nobody said so.
			if len(fields) != len(importColumns[kind]) {
				return out, fmt.Errorf("berkas %s berisi %d kolom, kontrak menetapkan %d",
					KindLabel[kind], len(fields), len(importColumns[kind]))
			}
			continue
		}

		if kind == KindTransactions {
			row, reason := parseRow(fields, index, sites, promos)
			if reason != "" {
				out.Rejections = append(out.Rejections, Rejection{LineNo: lineNo, Reason: reason, Original: line})
				continue
			}
			out.Txns = append(out.Txns, row)
			continue
		}

		if kind == KindHoliday {
			h, reason := parseHolidayRow(fields, index)
			if reason == "" {
				key := calendar.Key(h.Date)
				if first, dup := seen[key]; dup {
					reason = fmt.Sprintf("%s (baris %d)", reasonDuplicateTarget, first)
				} else {
					seen[key] = lineNo
				}
			}
			if reason != "" {
				out.Rejections = append(out.Rejections, Rejection{LineNo: lineNo, Reason: reason, Original: line})
				continue
			}
			out.Holidays = append(out.Holidays, h)
			continue
		}

		row, reason := parseTargetRow(fields, index, sites, kind)
		if reason == "" {
			key := row.SiteID.String() + row.PeriodKind + strconv.Itoa(row.Year) +
				strconv.Itoa(row.Month) + row.SalesType
			if first, dup := seen[key]; dup {
				reason = fmt.Sprintf("%s (baris %d)", reasonDuplicateTarget, first)
			} else {
				seen[key] = lineNo
			}
		}
		if reason != "" {
			out.Rejections = append(out.Rejections, Rejection{LineNo: lineNo, Reason: reason, Original: line})
			continue
		}
		out.Targets = append(out.Targets, row)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if header == nil {
		return out, errors.New("berkas kosong atau tanpa baris header")
	}
	return out, nil
}

// parseHolidayRow reads a holiday line.
//
// is_provisional is REQUIRED, not optional with a default. A date that drives
// the promotion lead time must say out loud whether it has been confirmed
// against the official decree; letting the column be omitted would make
// "certain" the silent default for exactly the dates most likely to be guesses.
func parseHolidayRow(f []string, index map[string]int) (Holiday, string) {
	get := func(name string) string {
		i, ok := index[name]
		if !ok || i >= len(f) {
			return ""
		}
		return strings.TrimSpace(f[i])
	}
	if len(f) < len(importColumns[KindHoliday]) {
		return Holiday{}, reasonShortRow
	}
	date, err := calendar.ParseDate(get("holiday_date"))
	if err != nil {
		return Holiday{}, reasonBadDate
	}
	name, err := sanitize.Text(get("holiday_name"), 200)
	if err != nil {
		return Holiday{}, reasonBlankName
	}
	prov, err := strconv.ParseBool(strings.ToLower(get("is_provisional")))
	if err != nil {
		return Holiday{}, reasonBadBool
	}
	return Holiday{Date: date, Name: name, Country: "ID", IsActive: true, IsProvisional: prov}, ""
}

// parseTargetRow reads a yearly or monthly target line.
//
// It deliberately performs NO cross-row arithmetic: BR-2.3 says the twelve
// months need not sum to the year, and an importer that checked would be the
// most natural place in the whole product to break that rule by accident.
func parseTargetRow(f []string, index map[string]int, sites map[string]Site, kind ImportKind) (TargetRow, string) {
	get := func(name string) string {
		i, ok := index[name]
		if !ok || i >= len(f) {
			return ""
		}
		return strings.TrimSpace(f[i])
	}
	if len(f) < len(importColumns[kind]) {
		return TargetRow{}, reasonShortRow
	}
	site, ok := sites[get("site_code")]
	if !ok {
		return TargetRow{}, reasonUnknownSite
	}
	year, err := strconv.Atoi(get("year"))
	if err != nil || year < 2000 || year > 2999 {
		return TargetRow{}, reasonBadYear
	}
	salesType, err := sanitize.Enum(get("sales_type"), "normal", "promo")
	if err != nil {
		return TargetRow{}, reasonBadEnum
	}
	amount, err := money.Parse(get("target_amount_idr"))
	if err != nil {
		if err == money.ErrNegative {
			return TargetRow{}, reasonNegativeAmount
		}
		return TargetRow{}, reasonBadAmount
	}

	row := TargetRow{
		CompanyID: site.CompanyID, SiteID: site.SiteID, SiteCode: site.SiteCode,
		Year: year, SalesType: salesType, AmountIDR: amount,
	}
	if kind == KindTargetMonth {
		month, err := strconv.Atoi(get("month"))
		if err != nil || month < 1 || month > 12 {
			return TargetRow{}, reasonBadMonth
		}
		row.PeriodKind, row.Month = "MONTH", month
	} else {
		row.PeriodKind = "YEAR"
	}
	return row, ""
}

func parseRow(f []string, index map[string]int, sites map[string]Site, promos map[string]uuid.UUID) (TxnRow, string) {
	get := func(name string) string {
		i, ok := index[name]
		if !ok || i >= len(f) {
			return ""
		}
		return strings.TrimSpace(f[i])
	}
	if len(f) < len(importColumns) {
		return TxnRow{}, reasonShortRow
	}

	site, ok := sites[get("site_code")]
	if !ok {
		return TxnRow{}, reasonUnknownSite
	}
	date, err := calendar.ParseDate(get("business_date"))
	if err != nil {
		return TxnRow{}, reasonBadDate
	}
	receipt, err := sanitize.Text(get("pos_receipt_no"), 100)
	if err != nil {
		return TxnRow{}, reasonBlankReceipt
	}
	salesType, err := sanitize.Enum(get("sales_type"), "normal", "promo")
	if err != nil {
		return TxnRow{}, reasonBadEnum
	}
	orderMode, err := sanitize.Enum(get("order_mode"), "dine_in", "take_away")
	if err != nil {
		return TxnRow{}, reasonBadEnum
	}
	amount, err := money.Parse(get("gross_amount_idr"))
	if err != nil {
		if err == money.ErrNegative {
			return TxnRow{}, reasonNegativeAmount
		}
		return TxnRow{}, reasonBadAmount
	}

	row := TxnRow{CompanyID: site.CompanyID, SiteID: site.SiteID, BusinessDate: date,
		ReceiptNo: receipt, SalesType: salesType, OrderMode: orderMode, GrossIDR: amount}

	promoCode := get("promo_code")
	switch salesType {
	case "promo":
		if promoCode == "" {
			return TxnRow{}, reasonPromoInconsistent
		}
		pid, ok := promos[promoCode]
		if !ok {
			return TxnRow{}, reasonUnknownPromo
		}
		row.PromoID = &pid
	case "normal":
		// BR-6.6: reject, never silently repair. A normal row carrying a promo
		// code is a POS defect, and dropping the code quietly would hide it.
		if promoCode != "" {
			return TxnRow{}, reasonPromoInconsistent
		}
	}
	return row, ""
}

// splitPipe honours RFC 4180 quoting with | as the separator, so a value
// containing a pipe survives.
func splitPipe(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			if inQuote && i+1 < len(line) && line[i+1] == '"' {
				cur.WriteByte('"')
				i++
			} else {
				inQuote = !inQuote
			}
		case c == '|' && !inQuote:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, cur.String())
	return out
}

// ImportDropPath processes every file in the drop directory, oldest first, and
// then MOVES each one out of it.
//
// The move is the point. Without it the drop directory is not a queue, it is a
// pile: a file that fails is retried every single night forever, because the
// checksum index only remembers runs that succeeded. Observed on the live
// server — one truncated file had failed on six consecutive nights, filling
// the run log and telling nobody. A processed file belongs in `processed/`, a
// failed one in `failed/` where somebody has to look at it (D48).
func (d *Deps) ImportDropPath(ctx context.Context, actor *uuid.UUID) ([]ImportResult, error) {
	dir := d.Params.String(ctx, ParamImportDropPath, d.Cfg.ImportDropPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []ImportResult
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
			continue
		}
		src := filepath.Join(dir, e.Name())
		res, err := d.ImportFile(ctx, src, actor)
		if err != nil {
			// One bad file must not stop the run: the rest of the night's
			// files still need to load. It is moved aside so tomorrow's run
			// does not trip over it again.
			d.Log.Error("import failed", "file", e.Name(), "err", err.Error())
			d.archive(src, dir, "failed")
			continue
		}
		bucket := "processed"
		if res.Run.Outcome == "FAILED" {
			bucket = "failed"
		}
		d.archive(src, dir, bucket)
		out = append(out, *res)
	}
	return out, nil
}

// archive moves a processed file out of the queue. A name collision is
// resolved rather than overwriting: two nights can legitimately produce
// `txn_20260901_01.csv`, and losing the first one to the second would destroy
// the only copy of what was actually loaded.
func (d *Deps) archive(src, dir, bucket string) {
	destDir := filepath.Join(dir, bucket)
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		d.Log.Error("cannot create archive directory", "dir", destDir, "err", err.Error())
		return
	}
	base := filepath.Base(src)
	dest := filepath.Join(destDir, base)
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(base)
		dest = filepath.Join(destDir, fmt.Sprintf("%s_%s%s",
			strings.TrimSuffix(base, ext), time.Now().UTC().Format("20060102-150405"), ext))
	}
	if err := os.Rename(src, dest); err != nil {
		// Rename fails across filesystems. Falling back to copy-then-delete
		// would risk losing the file if the delete failed, so it is left in
		// place and said out loud instead.
		d.Log.Error("cannot archive imported file; it will be retried next run",
			"file", base, "err", err.Error())
	}
}
