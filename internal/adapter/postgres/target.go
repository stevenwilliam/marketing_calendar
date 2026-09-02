package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type TargetRepo struct{ db *gorm.DB }

func NewTargetRepo(db *gorm.DB) *TargetRepo { return &TargetRepo{db: db} }

// List reads targets. Money comes back as BIGINT into int64 — no ORM
// arithmetic anywhere on this path (99 §6).
func (r *TargetRepo) List(ctx context.Context, f app.TargetFilter) ([]app.TargetRow, error) {
	like := "%" + f.Query + "%"
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT t.target_id, t.company_id, t.site_id, s.site_code, s.site_name,
		       t.period_kind, t.period_year, COALESCE(t.period_month, 0),
		       t.sales_type, t.target_amount_idr, t.updated_at
		  FROM sales_target t
		  JOIN site s ON s.site_id = t.site_id
		 WHERE t.company_id = ANY(?::uuid[])
		   AND (? = 0 OR t.period_year = ?)
		   AND (? = 0 OR COALESCE(t.period_month, 0) = ?)
		   AND (? = '' OR t.period_kind = ?)
		   AND (? = '' OR t.sales_type = ?)
		   AND (cardinality(?::uuid[]) = 0 OR t.site_id = ANY(?::uuid[]))
		   AND (? = '' OR s.site_code ILIKE ? OR s.site_name ILIKE ?)
		 ORDER BY s.site_code, t.period_year, COALESCE(t.period_month, 0), t.sales_type`,
		uuidList(f.Companies),
		f.Year, f.Year,
		f.Month, f.Month,
		f.PeriodKind, f.PeriodKind,
		f.SalesType, f.SalesType,
		uuidList(f.SiteIDs), uuidList(f.SiteIDs),
		f.Query, like, like).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.TargetRow
	for rows.Next() {
		var t app.TargetRow
		var amount int64
		if err := rows.Scan(&t.TargetID, &t.CompanyID, &t.SiteID, &t.SiteCode, &t.SiteName,
			&t.PeriodKind, &t.Year, &t.Month, &t.SalesType, &amount, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.AmountIDR = money.IDR(amount)
		out = append(out, t)
	}
	return out, nil
}

// Upsert writes one target. The unique index on
// (site, period_kind, year, month, sales_type) is what makes this idempotent;
// the ON CONFLICT clause names it by its columns rather than by index name so
// a rename cannot silently turn this into a duplicate insert.
func (r *TargetRepo) Upsert(ctx context.Context, t app.TargetRow, actor uuid.UUID) error {
	var month any
	if t.PeriodKind == "MONTH" {
		month = t.Month
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO sales_target (target_id, company_id, site_id, period_kind,
		                          period_year, period_month, sales_type,
		                          target_amount_idr, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (site_id, period_kind, period_year, COALESCE(period_month, 0), sales_type)
		DO UPDATE SET target_amount_idr = EXCLUDED.target_amount_idr,
		              updated_by = EXCLUDED.updated_by,
		              updated_at = now()`,
		id.New(), t.CompanyID, t.SiteID, t.PeriodKind,
		t.Year, month, t.SalesType, int64(t.AmountIDR), actor).Error
}

// SumBySite rolls up in SQL. BR-2.6 says a roll-up is arithmetic computed on
// read; SUM over BIGINT is integer arithmetic, so this stays on the integer
// path end to end.
func (r *TargetRepo) SumBySite(ctx context.Context, f app.TargetFilter) (map[uuid.UUID]money.IDR, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT site_id, SUM(target_amount_idr)::bigint
		  FROM sales_target
		 WHERE company_id = ANY(?::uuid[])
		   AND (? = 0 OR period_year = ?)
		   AND (? = '' OR period_kind = ?)
		   AND (? = '' OR sales_type = ?)
		 GROUP BY site_id`,
		uuidList(f.Companies), f.Year, f.Year,
		f.PeriodKind, f.PeriodKind, f.SalesType, f.SalesType).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]money.IDR{}
	for rows.Next() {
		var sid uuid.UUID
		var sum int64
		if err := rows.Scan(&sid, &sum); err != nil {
			return nil, err
		}
		out[sid] = money.IDR(sum)
	}
	return out, nil
}
