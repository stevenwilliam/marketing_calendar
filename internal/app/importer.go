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

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

// The CSV contract of 06-domain-operations.md §3.0 (D30). Pipe-delimited, the
// same convention as every export the product produces.
var importColumns = []string{
	"site_code", "business_date", "pos_receipt_no",
	"sales_type", "promo_code", "order_mode", "gross_amount_idr",
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
			FileName: filepath.Base(path), Checksum: checksum,
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

	rows, rejects, trailerRows, trailerTotal, parseErr := parseImport(strings.NewReader(string(raw)), sites, promos)
	if parseErr != nil {
		return nil, parseErr
	}

	run := ImportRun{FileName: filepath.Base(path), Checksum: checksum,
		RowsRead: len(rows) + len(rejects), TrailerRows: trailerRows, TrailerTotal: trailerTotal}

	// The trailer is the only thing that catches a truncated upload, which
	// otherwise arrives looking exactly like a quiet day of trading.
	if trailerRows != nil && *trailerRows != run.RowsRead {
		run.Outcome = "FAILED"
		run.Message = fmt.Sprintf("trailer menyatakan %d baris, berkas berisi %d — berkas terpotong?",
			*trailerRows, run.RowsRead)
		saved, err := d.Facts.LoadFile(ctx, run, nil, rejects, actor)
		return &ImportResult{Run: saved, Rejections: rejects}, err
	}
	// The TOTAL is only enforced when nothing was rejected.
	//
	// The trailer exists to catch a truncated upload, and the ROW COUNT above
	// is what catches that — a truncated file is short. The total is a second
	// opinion, and it is only meaningful when every row loaded: a file with
	// one bad line has a legitimately smaller sum than its trailer states.
	// Enforcing it regardless would fail the whole file for one bad row and
	// lose a night of trading, which is a far worse outcome than the one the
	// check is guarding against.
	if trailerTotal != nil && len(rejects) == 0 {
		var sum money.IDR
		for _, r := range rows {
			sum += r.GrossIDR
		}
		if sum != *trailerTotal {
			run.Outcome = "FAILED"
			run.Message = fmt.Sprintf("trailer menyatakan total Rp %s, berkas berjumlah Rp %s",
				trailerTotal.Format(), sum.Format())
			saved, err := d.Facts.LoadFile(ctx, run, nil, rejects, actor)
			return &ImportResult{Run: saved, Rejections: rejects}, err
		}
	}

	run.Outcome = "OK"
	if len(rejects) > 0 {
		run.Outcome = "PARTIAL"
		run.Message = fmt.Sprintf("%d baris ditolak", len(rejects))
		if trailerTotal != nil {
			var sum money.IDR
			for _, r := range rows {
				sum += r.GrossIDR
			}
			if sum != *trailerTotal {
				// Said plainly rather than hidden: the reconciliation screen
				// must show why the loaded total is below the file's own.
				run.Message += fmt.Sprintf("; total dimuat Rp %s dari Rp %s pada trailer — selisihnya adalah baris yang ditolak",
					sum.Format(), trailerTotal.Format())
			}
		}
	}
	saved, err := d.Facts.LoadFile(ctx, run, rows, rejects, actor)
	if err != nil {
		return nil, err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: actor, Action: "import.run",
		SubjectType: "import_run", SubjectID: &saved.ImportRunID,
		After: map[string]any{"file": saved.FileName, "inserted": saved.RowsInserted,
			"skipped": saved.RowsSkipped, "rejected": saved.RowsRejected}})
	return &ImportResult{Run: saved, Rejections: rejects}, nil
}

// parseImport reads the contract. The header is matched BY NAME, not by
// position: a POS that reorders its columns must not silently shift every
// amount into the wrong field.
func parseImport(r io.Reader, sites map[string]Site, promos map[string]uuid.UUID) ([]TxnRow, []Rejection, *int, *money.IDR, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	var header []string
	var index map[string]int
	var rows []TxnRow
	var rejects []Rejection
	var trailerRows *int
	var trailerTotal *money.IDR

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// The trailer: #TOTAL|<rows>|<sum>
		if strings.HasPrefix(line, "#TOTAL") {
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					trailerRows = &n
				}
				if t, err := money.Parse(strings.TrimSpace(parts[2])); err == nil {
					trailerTotal = &t
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
			var missing []string
			for _, want := range importColumns {
				if _, ok := index[want]; !ok {
					missing = append(missing, want)
				}
			}
			if len(missing) > 0 {
				return nil, nil, nil, nil, fmt.Errorf(
					"berkas tidak sesuai kontrak: kolom hilang %s", strings.Join(missing, ", "))
			}
			// An unknown column is a rejected FILE, not an ignored column: it
			// means the POS changed its export and nobody told us.
			if len(fields) != len(importColumns) {
				return nil, nil, nil, nil, fmt.Errorf(
					"berkas berisi %d kolom, kontrak menetapkan %d", len(fields), len(importColumns))
			}
			continue
		}

		row, reason := parseRow(fields, index, sites, promos)
		if reason != "" {
			rejects = append(rejects, Rejection{LineNo: lineNo, Reason: reason, Original: line})
			continue
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, nil, nil, err
	}
	if header == nil {
		return nil, nil, nil, nil, errors.New("berkas kosong atau tanpa baris header")
	}
	return rows, rejects, trailerRows, trailerTotal, nil
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

// ImportDropPath processes every file in the drop directory, oldest first.
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
		res, err := d.ImportFile(ctx, filepath.Join(dir, e.Name()), actor)
		if err != nil {
			// One bad file must not stop the run: the rest of the night's
			// files still need to load, and the failure is recorded.
			d.Log.Error("import failed", "file", e.Name(), "err", err.Error())
			continue
		}
		out = append(out, *res)
	}
	return out, nil
}
