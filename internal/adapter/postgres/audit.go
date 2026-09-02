package postgres

import (
	"context"
	"encoding/json"

	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type AuditRepo struct{ db *gorm.DB }

func NewAuditRepo(db *gorm.DB) *AuditRepo { return &AuditRepo{db: db} }

func (r *AuditRepo) Write(ctx context.Context, e app.AuditEntry) error {
	before, _ := marshalOrNil(e.Before)
	after, _ := marshalOrNil(e.After)
	var ip any
	if e.IP != "" {
		ip = e.IP
	}
	var reason any
	if e.Reason != "" {
		reason = e.Reason
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO audit_log (audit_id, actor_id, action, subject_type, subject_id,
		    company_id, before_state, after_state, reason, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.New(), nullUUID(e.ActorID), e.Action, e.SubjectType, nullUUID(e.SubjectID),
		nullUUID(e.CompanyID), before, after, reason, ip).Error
}

func marshalOrNil(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (r *AuditRepo) List(ctx context.Context, q string, limit, offset int) ([]app.AuditRow, int, error) {
	like := "%" + q + "%"
	var total int64
	if err := r.db.WithContext(ctx).Raw(`
		SELECT count(*) FROM audit_log a LEFT JOIN app_user u ON u.user_id = a.actor_id
		 WHERE (? = '' OR a.action ILIKE ? OR a.subject_type ILIKE ? OR u.full_name ILIKE ?)`,
		q, like, like, like).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT a.audit_id, COALESCE(u.full_name, 'Sistem'), a.action, a.subject_type,
		       a.subject_id, COALESCE(a.reason, ''), COALESCE(host(a.ip_address), ''), a.occurred_at
		  FROM audit_log a LEFT JOIN app_user u ON u.user_id = a.actor_id
		 WHERE (? = '' OR a.action ILIKE ? OR a.subject_type ILIKE ? OR u.full_name ILIKE ?)
		 ORDER BY a.occurred_at DESC LIMIT ? OFFSET ?`,
		q, like, like, like, limit, offset).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.AuditRow
	for rows.Next() {
		var a app.AuditRow
		if err := rows.Scan(&a.AuditID, &a.ActorName, &a.Action, &a.SubjectType,
			&a.SubjectID, &a.Reason, &a.IP, &a.OccurredAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, int(total), nil
}
