package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type FactRepo struct{ db *gorm.DB }

func NewFactRepo(db *gorm.DB) *FactRepo { return &FactRepo{db: db} }

// SeenChecksum answers BR-6.3 before the file is parsed at all. Identity is
// the checksum, not the filename, so renaming a file does not let it in twice.
func (r *FactRepo) SeenChecksum(ctx context.Context, sum string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT count(*) FROM import_run
		 WHERE file_checksum = ? AND outcome IN ('OK','PARTIAL')`, sum).Scan(&n).Error
	return n > 0, err
}

// LoadFile writes the run, replaces the site-days the file covers, and records
// the rejections — all in ONE transaction (BR-6.3, BR-6.5). A partial failure
// must not leave half a file loaded.
func (r *FactRepo) LoadFile(ctx context.Context, run app.ImportRun, rows []app.TxnRow, rejects []app.Rejection, actor *uuid.UUID) (app.ImportRun, error) {
	run.ImportRunID = id.New()
	run.StartedAt = time.Now().UTC()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Replace this file's site-days. BR-6.3: re-importing a CORRECTED file
		// for the same day replaces that day's rows for that site, and the
		// delete has to happen inside the same transaction as the insert or a
		// crash between them loses a day of trading.
		seen := map[string]bool{}
		for _, row := range rows {
			key := row.SiteID.String() + row.BusinessDate.Format("2006-01-02")
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := tx.Exec(`DELETE FROM history_txn WHERE site_id = ? AND business_date = ?`,
				row.SiteID, row.BusinessDate).Error; err != nil {
				return err
			}
		}

		var inserted int
		for _, row := range rows {
			res := tx.Exec(`
				INSERT INTO history_txn (txn_id, company_id, site_id, business_date,
				    pos_receipt_no, sales_type, promo_id, order_mode, gross_amount_idr, import_run_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (site_id, business_date, pos_receipt_no) DO NOTHING`,
				id.New(), row.CompanyID, row.SiteID, row.BusinessDate, row.ReceiptNo,
				row.SalesType, nullUUID(row.PromoID), row.OrderMode,
				int64(row.GrossIDR), run.ImportRunID)
			if res.Error != nil {
				return res.Error
			}
			inserted += int(res.RowsAffected)
		}
		run.RowsInserted = inserted
		run.RowsSkipped = len(rows) - inserted
		run.RowsRejected = len(rejects)

		fin := time.Now().UTC()
		run.FinishedAt = &fin
		if err := insertRun(tx, run, actor, fin); err != nil {
			return err
		}
		for _, rj := range rejects {
			if err := tx.Exec(`
				INSERT INTO import_rejection (rejection_id, import_run_id, line_no, reason, original_line)
				VALUES (?, ?, ?, ?, ?)`,
				id.New(), run.ImportRunID, rj.LineNo, rj.Reason, rj.Original).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return run, err
}

// LoadTargets writes a target import: the rows, the rejections and the run row
// in ONE transaction, the run row last so its counts are the real ones.
//
// Targets are UPSERTED by the (site, period_kind, year, month, sales_type)
// unique index, so re-importing a corrected file overwrites rather than
// duplicating — the same idempotency the transaction path gets from its
// receipt key, and the reason a target import needs no delete-then-insert.
//
// It deliberately performs no arithmetic across rows. BR-2.3 says the twelve
// months need not sum to the year, and a bulk loader is the most natural place
// in the product to break that rule by being helpful.
func (r *FactRepo) LoadTargets(ctx context.Context, run app.ImportRun, rows []app.TargetRow, rejects []app.Rejection, actor *uuid.UUID) (app.ImportRun, error) {
	run.ImportRunID = id.New()
	run.StartedAt = time.Now().UTC()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var written int
		for _, row := range rows {
			var month any
			if row.PeriodKind == "MONTH" {
				month = row.Month
			}
			res := tx.Exec(`
				INSERT INTO sales_target (target_id, company_id, site_id, period_kind,
				                          period_year, period_month, sales_type,
				                          target_amount_idr, updated_by)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (site_id, period_kind, period_year, COALESCE(period_month, 0), sales_type)
				DO UPDATE SET target_amount_idr = EXCLUDED.target_amount_idr,
				              updated_by = EXCLUDED.updated_by,
				              updated_at = now()`,
				id.New(), row.CompanyID, row.SiteID, row.PeriodKind,
				row.Year, month, row.SalesType, int64(row.AmountIDR), nullUUID(actor))
			if res.Error != nil {
				return res.Error
			}
			written += int(res.RowsAffected)
		}
		run.RowsInserted = written
		run.RowsSkipped = len(rows) - written
		run.RowsRejected = len(rejects)

		fin := time.Now().UTC()
		run.FinishedAt = &fin
		if err := insertRun(tx, run, actor, fin); err != nil {
			return err
		}
		for _, rj := range rejects {
			if err := tx.Exec(`
				INSERT INTO import_rejection (rejection_id, import_run_id, line_no, reason, original_line)
				VALUES (?, ?, ?, ?, ?)`,
				id.New(), run.ImportRunID, rj.LineNo, rj.Reason, rj.Original).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return run, err
}

// insertRun is the one place a run row is written, so the two loaders cannot
// drift in what they record.
func insertRun(tx *gorm.DB, run app.ImportRun, actor *uuid.UUID, fin time.Time) error {
	var trailerRows, trailerTotal any
	if run.TrailerRows != nil {
		trailerRows = *run.TrailerRows
	}
	if run.TrailerTotal != nil {
		trailerTotal = int64(*run.TrailerTotal)
	}
	kind := run.Kind
	if kind == "" {
		kind = "transactions"
	}
	return tx.Exec(`
		INSERT INTO import_run (import_run_id, file_name, file_checksum, kind, rows_read,
		    rows_inserted, rows_skipped, rows_rejected, trailer_rows, trailer_total_idr,
		    outcome, message, actor_id, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ImportRunID, run.FileName, run.Checksum, kind, run.RowsRead,
		run.RowsInserted, run.RowsSkipped, run.RowsRejected,
		trailerRows, trailerTotal, run.Outcome, run.Message,
		nullUUID(actor), run.StartedAt, fin).Error
}

func (r *FactRepo) Runs(ctx context.Context, limit, offset int) ([]app.ImportRun, int, error) {
	var total int64
	if err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM import_run`).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT import_run_id, file_name, file_checksum, kind, rows_read, rows_inserted,
		       rows_skipped, rows_rejected, trailer_rows, trailer_total_idr,
		       outcome, COALESCE(message, ''), started_at, finished_at
		  FROM import_run ORDER BY started_at DESC LIMIT ? OFFSET ?`, limit, offset).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.ImportRun
	for rows.Next() {
		var r app.ImportRun
		var trailerTotal *int64
		if err := rows.Scan(&r.ImportRunID, &r.FileName, &r.Checksum, &r.Kind, &r.RowsRead,
			&r.RowsInserted, &r.RowsSkipped, &r.RowsRejected, &r.TrailerRows,
			&trailerTotal, &r.Outcome, &r.Message, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, 0, err
		}
		if trailerTotal != nil {
			m := money.IDR(*trailerTotal)
			r.TrailerTotal = &m
		}
		out = append(out, r)
	}
	return out, int(total), nil
}

func (r *FactRepo) Rejections(ctx context.Context, runID uuid.UUID) ([]app.Rejection, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT line_no, reason, original_line FROM import_rejection
		 WHERE import_run_id = ? ORDER BY line_no`, runID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Rejection
	for rows.Next() {
		var rj app.Rejection
		if err := rows.Scan(&rj.LineNo, &rj.Reason, &rj.Original); err != nil {
			return nil, err
		}
		out = append(out, rj)
	}
	return out, nil
}

// PromoActuals attributes actuals BY promo_id, never by date range. A
// transaction inside a promotion's dates but not tagged with its id is normal
// sales, and counting it would flatter every report (BR-7.6).
func (r *FactRepo) PromoActuals(ctx context.Context, planIDs []uuid.UUID) (map[uuid.UUID]app.Actual, error) {
	out := map[uuid.UUID]app.Actual{}
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT promo_id, COALESCE(SUM(gross_amount_idr), 0)::bigint, count(*)
		  FROM history_txn
		 WHERE promo_id = ANY(?::uuid[]) AND sales_type = 'promo'
		 GROUP BY promo_id`, uuidList(planIDs)).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid uuid.UUID
		var gross int64
		var count int
		if err := rows.Scan(&pid, &gross, &count); err != nil {
			return nil, err
		}
		out[pid] = app.Actual{GrossIDR: money.IDR(gross), ReceiptCount: count}
	}
	return out, nil
}

// TargetVsActual joins targets to actuals per site and month. The join is a
// FULL OUTER JOIN in effect: a site with a target and no sales must appear
// (that is the interesting row), and so must sales against no target.
func (r *FactRepo) TargetVsActual(ctx context.Context, f app.TargetFilter) ([]app.TargetActualRow, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		WITH t AS (
		    SELECT company_id, site_id, period_year, COALESCE(period_month, 0) AS period_month,
		           sales_type, SUM(target_amount_idr)::bigint AS target_idr
		      FROM sales_target
		     WHERE company_id = ANY(?::uuid[])
		       AND (? = 0 OR period_year = ?)
		       AND (? = 0 OR COALESCE(period_month, 0) = ?)
		       AND period_kind = CASE WHEN ? = 0 THEN 'YEAR' ELSE 'MONTH' END
		       AND (? = '' OR sales_type = ?)
		     GROUP BY 1,2,3,4,5
		), a AS (
		    SELECT company_id, site_id,
		           EXTRACT(YEAR FROM business_date)::int AS period_year,
		           CASE WHEN ? = 0 THEN 0 ELSE EXTRACT(MONTH FROM business_date)::int END AS period_month,
		           sales_type,
		           SUM(gross_amount_idr)::bigint AS actual_idr,
		           count(*)::int AS receipts
		      FROM history_txn
		     WHERE company_id = ANY(?::uuid[])
		       AND (? = 0 OR EXTRACT(YEAR FROM business_date) = ?)
		       AND (? = 0 OR EXTRACT(MONTH FROM business_date) = ?)
		       AND (? = '' OR sales_type = ?)
		     GROUP BY 1,2,3,4,5
		)
		SELECT c.company_id, c.company_name, s.site_id, s.site_code, s.site_name,
		       COALESCE(t.period_year, a.period_year),
		       COALESCE(t.period_month, a.period_month),
		       COALESCE(t.sales_type, a.sales_type),
		       COALESCE(t.target_idr, 0), COALESCE(a.actual_idr, 0), COALESCE(a.receipts, 0)
		  FROM t FULL OUTER JOIN a
		    ON t.site_id = a.site_id AND t.period_year = a.period_year
		   AND t.period_month = a.period_month AND t.sales_type = a.sales_type
		  JOIN site s ON s.site_id = COALESCE(t.site_id, a.site_id)
		  JOIN company c ON c.company_id = s.company_id
		 ORDER BY c.company_name, s.site_code,
		          COALESCE(t.period_year, a.period_year),
		          COALESCE(t.period_month, a.period_month)`,
		uuidList(f.Companies), f.Year, f.Year, f.Month, f.Month, f.Month,
		f.SalesType, f.SalesType,
		f.Month,
		uuidList(f.Companies), f.Year, f.Year, f.Month, f.Month,
		f.SalesType, f.SalesType).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.TargetActualRow
	for rows.Next() {
		var r app.TargetActualRow
		var target, actual int64
		if err := rows.Scan(&r.CompanyID, &r.CompanyName, &r.SiteID, &r.SiteCode, &r.SiteName,
			&r.Year, &r.Month, &r.SalesType, &target, &actual, &r.ReceiptCount); err != nil {
			return nil, err
		}
		r.TargetIDR = money.IDR(target)
		r.ActualIDR = money.IDR(actual)
		out = append(out, r)
	}
	return out, nil
}

func (r *FactRepo) SiteCodeIndex(ctx context.Context) (map[string]app.Site, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT site_id, company_id, site_code, site_name, site_type, is_active FROM site`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]app.Site{}
	for rows.Next() {
		var s app.Site
		if err := rows.Scan(&s.SiteID, &s.CompanyID, &s.SiteCode, &s.SiteName, &s.SiteType, &s.IsActive); err != nil {
			return nil, err
		}
		out[s.SiteCode] = s
	}
	return out, nil
}

// PlanCodeIndex maps plan_code to id for the importer. Only RELEASED plans:
// attributing sales to a plan that was never approved is how a cancelled
// promotion acquires revenue (BR-6.6 UNKNOWN_PROMO).
func (r *FactRepo) PlanCodeIndex(ctx context.Context) (map[string]uuid.UUID, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT plan_code, plan_id FROM promotion_plan WHERE status = 'RELEASED'`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]uuid.UUID{}
	for rows.Next() {
		var code string
		var pid uuid.UUID
		if err := rows.Scan(&code, &pid); err != nil {
			return nil, err
		}
		out[code] = pid
	}
	return out, nil
}
