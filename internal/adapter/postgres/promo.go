package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

type PromoRepo struct{ db *gorm.DB }

func NewPromoRepo(db *gorm.DB) *PromoRepo { return &PromoRepo{db: db} }

const planSelect = `
SELECT p.plan_id, p.company_id, p.plan_code, p.site_group_id, p.current_version_id,
       p.status, p.force_released, p.created_by, p.created_at,
       v.version_id, v.version_no, v.promo_name, v.start_date, v.end_date,
       v.target_sales_idr, v.target_receipt_count, v.order_mode, v.promo_rule,
       v.overlap_acknowledged, v.lead_time_overridden,
       COALESCE(v.lead_time_override_reason, ''),
       c.company_name, c.company_code, g.site_group_name, u.full_name,
       COALESCE(ai.current_step_no, 0), COALESCE(st.step_name, ''),
       COALESCE(ai.status, '')
  FROM promotion_plan p
  JOIN promotion_plan_version v ON v.version_id = p.current_version_id
  JOIN company c ON c.company_id = p.company_id
  JOIN site_group g ON g.site_group_id = p.site_group_id
  JOIN app_user u ON u.user_id = p.created_by
  LEFT JOIN approval_instance ai ON ai.instance_id = p.approval_instance_id
  LEFT JOIN approval_step st ON st.version_id = ai.version_id AND st.step_no = ai.current_step_no`

func scanPlan(rows interface{ Scan(...any) error }) (app.PlanRow, error) {
	var p app.PlanRow
	var sales int64
	err := rows.Scan(&p.PlanID, &p.CompanyID, &p.PlanCode, &p.SiteGroupID, &p.CurrentVersionID,
		&p.Status, &p.ForceReleased, &p.Plan.CreatedBy, &p.Plan.CreatedAt,
		&p.Version.VersionID, &p.Version.VersionNo, &p.Version.PromoName,
		&p.Version.StartDate, &p.Version.EndDate, &sales, &p.Version.TargetReceiptCount,
		&p.Version.OrderMode, &p.Version.PromoRule, &p.Version.OverlapAcknowledged,
		&p.Version.LeadTimeOverridden, &p.Version.LeadTimeOverrideReason,
		&p.CompanyName, &p.CompanyCode, &p.SiteGroupName, &p.CreatedByName,
		&p.CurrentStepNo, &p.CurrentStepName, &p.ApprovalStatus)
	p.Version.TargetSalesIDR = money.IDR(sales)
	p.Version.PlanID = p.PlanID
	return p, err
}

func (r *PromoRepo) List(ctx context.Context, f app.PlanFilter) ([]app.PlanRow, int, error) {
	like := "%" + f.Query + "%"
	where := `
		 WHERE p.company_id = ANY(?::uuid[])
		   AND (? = '' OR p.status = ?)
		   AND (?::date IS NULL OR v.end_date >= ?)
		   AND (?::date IS NULL OR v.start_date <= ?)
		   AND (? = '' OR v.order_mode = ?)
		   AND (? = '' OR v.promo_name ILIKE ? OR p.plan_code ILIKE ? OR g.site_group_name ILIKE ?)`
	var from, to any
	if !f.From.IsZero() {
		from = f.From
	}
	if !f.To.IsZero() {
		to = f.To
	}
	args := []any{uuidList(f.Companies), f.Status, f.Status,
		from, from, to, to, f.OrderMode, f.OrderMode,
		f.Query, like, like, like}

	var total int64
	countSQL := `SELECT count(*) FROM promotion_plan p
	  JOIN promotion_plan_version v ON v.version_id = p.current_version_id
	  JOIN site_group g ON g.site_group_id = p.site_group_id` + where
	if err := r.db.WithContext(ctx).Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.db.WithContext(ctx).Raw(
		planSelect+where+` ORDER BY v.start_date DESC, p.plan_code DESC LIMIT ? OFFSET ?`,
		append(args, limit, f.Offset)...).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.PlanRow
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, int(total), nil
}

func (r *PromoRepo) ByID(ctx context.Context, planID uuid.UUID) (*app.PlanRow, error) {
	row := r.db.WithContext(ctx).Raw(planSelect+` WHERE p.plan_id = ?`, planID).Row()
	p, err := scanPlan(row)
	if isNoRows(err) {
		return nil, apierror.NotFound("rencana promo")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PromoRepo) Versions(ctx context.Context, planID uuid.UUID) ([]promo.Version, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT version_id, plan_id, version_no, promo_name, start_date, end_date,
		       target_sales_idr, target_receipt_count, order_mode, promo_rule,
		       overlap_acknowledged, lead_time_overridden,
		       COALESCE(lead_time_override_reason, ''), created_by, created_at
		  FROM promotion_plan_version WHERE plan_id = ? ORDER BY version_no DESC`, planID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []promo.Version
	for rows.Next() {
		var v promo.Version
		var sales int64
		if err := rows.Scan(&v.VersionID, &v.PlanID, &v.VersionNo, &v.PromoName,
			&v.StartDate, &v.EndDate, &sales, &v.TargetReceiptCount, &v.OrderMode,
			&v.PromoRule, &v.OverlapAcknowledged, &v.LeadTimeOverridden,
			&v.LeadTimeOverrideReason, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.TargetSalesIDR = money.IDR(sales)
		out = append(out, v)
	}
	return out, nil
}

func (r *PromoRepo) Create(ctx context.Context, p promo.Plan, v promo.Version) (uuid.UUID, error) {
	planID := id.New()
	versionID := id.New()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO promotion_plan (plan_id, company_id, plan_code, site_group_id, status, created_by)
			VALUES (?, ?, ?, ?, 'DRAFT', ?)`,
			planID, p.CompanyID, p.PlanCode, p.SiteGroupID, p.CreatedBy).Error; err != nil {
			if isForeignKeyViolation(err) {
				return apierror.New(apierror.CodeCrossBrand,
					"kelompok toko bukan milik perusahaan rencana ini")
			}
			return err
		}
		if err := insertVersion(tx, versionID, planID, 1, v); err != nil {
			return err
		}
		return tx.Exec(`UPDATE promotion_plan SET current_version_id = ? WHERE plan_id = ?`,
			versionID, planID).Error
	})
	return planID, err
}

func insertVersion(tx *gorm.DB, versionID, planID uuid.UUID, no int, v promo.Version) error {
	var reason any
	if v.LeadTimeOverrideReason != "" {
		reason = v.LeadTimeOverrideReason
	}
	return tx.Exec(`
		INSERT INTO promotion_plan_version (version_id, plan_id, version_no, promo_name,
		    start_date, end_date, target_sales_idr, target_receipt_count, order_mode,
		    promo_rule, overlap_acknowledged, lead_time_overridden,
		    lead_time_override_reason, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		versionID, planID, no, v.PromoName, v.StartDate, v.EndDate,
		int64(v.TargetSalesIDR), v.TargetReceiptCount, v.OrderMode, v.PromoRule,
		v.OverlapAcknowledged, v.LeadTimeOverridden, reason, v.CreatedBy).Error
}

// SaveDraftVersion updates a version IN PLACE. It is guarded by the plan's
// status: only DRAFT and REJECTED reach here, which is BR-4.7 enforced at the
// write rather than trusted from the caller.
func (r *PromoRepo) SaveDraftVersion(ctx context.Context, v promo.Version) error {
	var reason any
	if v.LeadTimeOverrideReason != "" {
		reason = v.LeadTimeOverrideReason
	}
	res := r.db.WithContext(ctx).Exec(`
		UPDATE promotion_plan_version pv
		   SET promo_name = ?, start_date = ?, end_date = ?, target_sales_idr = ?,
		       target_receipt_count = ?, order_mode = ?, promo_rule = ?,
		       overlap_acknowledged = ?, lead_time_overridden = ?,
		       lead_time_override_reason = ?
		  FROM promotion_plan p
		 WHERE pv.version_id = ? AND p.plan_id = pv.plan_id
		   AND p.status IN ('DRAFT','REJECTED')`,
		v.PromoName, v.StartDate, v.EndDate, int64(v.TargetSalesIDR),
		v.TargetReceiptCount, v.OrderMode, v.PromoRule, v.OverlapAcknowledged,
		v.LeadTimeOverridden, reason, v.VersionID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apierror.New(apierror.CodePlanLocked,
			"rencana yang sudah masuk rantai persetujuan tidak dapat diubah; buat versi baru")
	}
	return nil
}

// NewVersion is the BR-4.7 path: an edit after approval creates a successor
// and re-points the plan at it. The previous version is retained.
func (r *PromoRepo) NewVersion(ctx context.Context, planID uuid.UUID, v promo.Version) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var next int
		if err := tx.Raw(`
			SELECT COALESCE(max(version_no), 0) + 1 FROM promotion_plan_version
			 WHERE plan_id = ?`, planID).Scan(&next).Error; err != nil {
			return err
		}
		versionID := id.New()
		if err := insertVersion(tx, versionID, planID, next, v); err != nil {
			return err
		}
		return tx.Exec(`
			UPDATE promotion_plan
			   SET current_version_id = ?, status = 'DRAFT', approval_instance_id = NULL,
			       force_released = false, updated_at = now()
			 WHERE plan_id = ?`, versionID, planID).Error
	})
}

func (r *PromoRepo) SetStatus(ctx context.Context, planID uuid.UUID, status promo.Status, instanceID *uuid.UUID, forceReleased bool) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE promotion_plan
		   SET status = ?,
		       approval_instance_id = COALESCE(?, approval_instance_id),
		       force_released = ?,
		       updated_at = now()
		 WHERE plan_id = ?`,
		string(status), nullUUID(instanceID), forceReleased, planID).Error
}

// Overlaps finds promotions sharing ANY SITE with the target group, not merely
// the same group id. Two different groups containing the same store DO
// conflict, and that is the case a naive implementation misses (BR-3.6).
func (r *PromoRepo) Overlaps(ctx context.Context, companyID, siteGroupID, excludePlanID uuid.UUID, from, to time.Time) ([]promo.Overlap, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		WITH target_sites AS (
		    SELECT site_id FROM site_group_member WHERE site_group_id = ?
		)
		SELECT p.plan_id, p.plan_code, v.promo_name, v.start_date, v.end_date,
		       COALESCE(array_agg(DISTINCT m.site_id::text), '{}')
		  FROM promotion_plan p
		  JOIN promotion_plan_version v ON v.version_id = p.current_version_id
		  JOIN site_group_member m ON m.site_group_id = p.site_group_id
		 WHERE p.company_id = ?
		   AND p.plan_id <> ?
		   AND p.status IN ('PENDING','RELEASED')
		   AND m.site_id IN (SELECT site_id FROM target_sites)
		   AND v.start_date <= ? AND v.end_date >= ?
		 GROUP BY p.plan_id, p.plan_code, v.promo_name, v.start_date, v.end_date
		 ORDER BY v.start_date`,
		siteGroupID, companyID, excludePlanID, to, from).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []promo.Overlap
	for rows.Next() {
		var o promo.Overlap
		var siteStrs []string
		if err := rows.Scan(&o.PlanID, &o.PlanCode, &o.PromoName,
			&o.StartDate, &o.EndDate, pqArray(&siteStrs)); err != nil {
			return nil, err
		}
		for _, s := range siteStrs {
			if u, err := uuid.Parse(s); err == nil {
				o.SharedSiteIDs = append(o.SharedSiteIDs, u)
			}
		}
		out = append(out, o)
	}
	return out, nil
}

// PendingBefore is the auto-cancel job's query (BR-4.6).
func (r *PromoRepo) PendingBefore(ctx context.Context, cancelOnOrBefore time.Time) ([]app.PlanRow, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		planSelect+` WHERE p.status = 'PENDING' AND v.start_date <= ?
		  ORDER BY v.start_date`, cancelOnOrBefore).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.PlanRow
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// NextPlanCode is P-YYYY-NNNN with a CSPRNG suffix on collision. The sequence
// is human-facing, so it stays readable; it is not a security boundary.
func (r *PromoRepo) NextPlanCode(ctx context.Context, year int) (string, error) {
	var n int
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(max(substring(plan_code from 8)::int), 0) + 1
		  FROM promotion_plan WHERE plan_code LIKE ?`,
		fmt.Sprintf("P-%d-%%", year)).Scan(&n).Error
	if err != nil {
		suffix, cerr := security.NewCode(6)
		if cerr != nil {
			return "", err
		}
		return fmt.Sprintf("P-%d-%s", year, suffix), nil
	}
	return fmt.Sprintf("P-%d-%04d", year, n), nil
}
