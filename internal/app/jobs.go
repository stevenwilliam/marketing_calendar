package app

import (
	"context"
	"fmt"

	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
)

// AutoCancel is the daily job of BR-4.6.
//
// It is idempotent by construction: it only touches PENDING plans, and a plan
// it cancels is no longer PENDING. Running it twice in a day, or catching up
// after a missed day, both do the right thing.
func (d *Deps) AutoCancel(ctx context.Context) (int, error) {
	today := calendar.Today(d.Now())
	m := d.Params.Int(ctx, ParamAutoCancelDays, 5)

	// A plan is due when today >= start - M, i.e. start <= today + M.
	cutoff := today.AddDate(0, 0, m)
	candidates, err := d.Promos.PendingBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}

	runID := id.New()
	_ = d.DB.WithContext(ctx).Exec(`
		INSERT INTO job_run (job_run_id, job_name, business_date, outcome)
		VALUES (?, 'auto-cancel', ?, 'RUNNING')`, runID, today).Error

	cancelled := 0
	for _, row := range candidates {
		// The domain decides, not the query. The SQL narrows the candidates;
		// ShouldAutoCancel is the rule, and it is the thing under test.
		if !promo.ShouldAutoCancel(row.Plan, row.Version, today, m) {
			continue
		}
		instance, err := d.Approval.InstanceBySubject(ctx, approval.SubjectPromotionPlan, row.PlanID)
		if err != nil {
			d.Log.Warn("auto-cancel: no approval instance", "plan", row.PlanCode, "err", err.Error())
			continue
		}
		_, err = d.Approval.Decide(ctx, instance.InstanceID, "",
			func(in *approval.Instance, _ approval.Chain) (approval.Decision, error) {
				return in.AutoCancel(d.Now())
			})
		if err != nil {
			d.Log.Warn("auto-cancel: decision refused", "plan", row.PlanCode, "err", err.Error())
			continue
		}
		if err := d.Promos.SetStatus(ctx, row.PlanID, promo.StatusCancelled, nil, false); err != nil {
			d.Log.Error("auto-cancel: status not written", "plan", row.PlanCode, "err", err.Error())
			continue
		}
		planID := row.PlanID
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: nil, Action: "promo.auto_cancel",
			SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
			Reason: fmt.Sprintf("rantai persetujuan belum selesai %d hari sebelum mulai (%s)",
				m, row.Version.StartDate.Format("2006-01-02"))})
		d.notifyAutoCancelled(ctx, row, m)
		cancelled++
	}

	_ = d.DB.WithContext(ctx).Exec(`
		UPDATE job_run SET outcome = 'OK', affected = ?, finished_at = now(),
		       message = ? WHERE job_run_id = ?`,
		cancelled, fmt.Sprintf("%d dari %d kandidat dibatalkan", cancelled, len(candidates)), runID).Error
	return cancelled, nil
}

// notifyAutoCancelled tells the creator AND every pending approver (BR-4.6).
// These are workflow notifications, not the release announcement, so D32's
// fixed list does not apply.
func (d *Deps) notifyAutoCancelled(ctx context.Context, row PlanRow, m int) {
	if d.Notify == nil {
		return
	}
	recipients := d.pendingApproverEmails(ctx, row.PlanID)
	var creator string
	_ = d.DB.WithContext(ctx).Raw(`SELECT email FROM app_user WHERE user_id = ?`,
		row.Plan.CreatedBy).Scan(&creator).Error
	if creator != "" {
		recipients = append(recipients, creator)
	}
	if len(recipients) == 0 {
		return
	}
	planID := row.PlanID
	_ = d.Notify.Queue(ctx, recipients,
		fmt.Sprintf("[Marketing Calendar] %s dibatalkan otomatis", row.PlanCode),
		fmt.Sprintf("%s — %s dibatalkan otomatis karena rantai persetujuan belum selesai "+
			"%d hari sebelum tanggal mulai (%s).\n\n"+
			"Rencana ini dapat dihidupkan kembali oleh superadmin dengan alasan tertulis.\n",
			row.PlanCode, row.Version.PromoName, m, row.Version.StartDate.Format("2006-01-02")),
		"promotion_plan", &planID)
}

// FlushNotifications drains the queue. Called by `mc job notify` and after a
// release, so a send failure retries rather than losing the message.
func (d *Deps) FlushNotifications(ctx context.Context, limit int) (int, int, error) {
	if d.Notify == nil {
		return 0, 0, nil
	}
	return d.Notify.Flush(ctx, limit)
}

// PurgeSessions keeps the session tables bounded. Explicitly NOT the audit or
// approval tables: BR-8.5 says nothing there is ever purged.
func (d *Deps) PurgeSessions(ctx context.Context) (int, error) {
	return d.Sessions.PurgeExpired(ctx, d.Now())
}
