package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

// actorFor builds the approval Actor with the roles held IN THE SUBJECT'S
// COMPANY. Passing the union of a user's roles across companies would let a
// Maxx Coffee operations head approve a Ruuma plan (BR-4.4a, D37).
func actorFor(p Principal, companyID uuid.UUID) approval.Actor {
	return approval.Actor{
		UserID:       p.UserID,
		RoleIDs:      p.RolesInCompany(companyID),
		CompanyIDs:   p.CompanyIDs,
		IsSuperadmin: p.IsSuperadmin,
	}
}

// Approve advances the chain. Every refusal maps to a named code so the UI can
// say why rather than showing a generic 403.
func (d *Deps) Approve(ctx context.Context, p Principal, planID uuid.UUID, ip string) error {
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return err
	}
	if row.Plan.Status != promo.StatusPending {
		return apierror.New(apierror.CodeConflict, "rencana ini tidak sedang menunggu persetujuan")
	}
	instance, err := d.Approval.InstanceBySubject(ctx, approval.SubjectPromotionPlan, planID)
	if err != nil {
		return err
	}

	dec, err := d.Approval.Decide(ctx, instance.InstanceID, ip,
		func(in *approval.Instance, chain approval.Chain) (approval.Decision, error) {
			return in.Approve(chain, actorFor(p, in.CompanyID), d.Now())
		})
	if err != nil {
		return mapApprovalError(err)
	}

	if dec.ChainClosed && dec.NewStatus == approval.StatusApproved {
		if err := d.Promos.SetStatus(ctx, planID, promo.StatusReleased, nil, false); err != nil {
			return err
		}
		d.notifyRelease(ctx, planID)
	} else {
		d.notifyStep(ctx, planID)
	}

	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "approval.approve",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		After: map[string]any{"step": dec.Events[0].StepNo, "status": dec.NewStatus}, IP: ip})
	return nil
}

// Reject returns the plan to its creator with a mandatory reason (BR-4.5).
func (d *Deps) Reject(ctx context.Context, p Principal, planID uuid.UUID, reason, ip string) error {
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return err
	}
	clean, err := sanitize.Text(reason, 2000)
	if err != nil {
		return apierror.New(apierror.CodeReasonRequired, "alasan penolakan wajib diisi")
	}
	instance, err := d.Approval.InstanceBySubject(ctx, approval.SubjectPromotionPlan, planID)
	if err != nil {
		return err
	}
	_, err = d.Approval.Decide(ctx, instance.InstanceID, ip,
		func(in *approval.Instance, chain approval.Chain) (approval.Decision, error) {
			return in.Reject(chain, actorFor(p, in.CompanyID), clean, d.Now())
		})
	if err != nil {
		return mapApprovalError(err)
	}
	if err := d.Promos.SetStatus(ctx, planID, promo.StatusRejected, nil, false); err != nil {
		return err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "approval.reject",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		Reason: clean, IP: ip})
	d.notifyRejected(ctx, planID, clean)
	return nil
}

// ForceRelease is the only bypass in the system (BR-4.8, D27, D35).
func (d *Deps) ForceRelease(ctx context.Context, p Principal, planID uuid.UUID, reason, ip string) error {
	if !p.IsSuperadmin {
		return apierror.New(apierror.CodeForbidden, "hanya superadmin yang dapat merilis paksa")
	}
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return err
	}
	clean, err := sanitize.Text(reason, 2000)
	if err != nil {
		return apierror.New(apierror.CodeReasonRequired, "alasan rilis paksa wajib diisi")
	}
	instance, err := d.Approval.InstanceBySubject(ctx, approval.SubjectPromotionPlan, planID)
	if err != nil {
		return err
	}
	_, err = d.Approval.Decide(ctx, instance.InstanceID, ip,
		func(in *approval.Instance, chain approval.Chain) (approval.Decision, error) {
			return in.ForceRelease(actorFor(p, in.CompanyID), clean, d.Now())
		})
	if err != nil {
		return mapApprovalError(err)
	}
	// force_released carries onto the promotion report, which is the point:
	// a reviewer must be able to see which plans bypassed the chain (BR-7.5).
	if err := d.Promos.SetStatus(ctx, planID, promo.StatusReleased, nil, true); err != nil {
		return err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "approval.force_release",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		Reason: clean, IP: ip})
	d.notifyRelease(ctx, planID)
	return nil
}

// Revive returns an auto-cancelled plan to the step it was pending at. The
// cancellation event stays; the revival is a second event (BR-4.6).
func (d *Deps) Revive(ctx context.Context, p Principal, planID uuid.UUID, reason, ip string) error {
	if !p.IsSuperadmin {
		return apierror.New(apierror.CodeForbidden, "hanya superadmin yang dapat menghidupkan kembali rencana")
	}
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return err
	}
	clean, err := sanitize.Text(reason, 2000)
	if err != nil {
		return apierror.New(apierror.CodeReasonRequired, "alasan wajib diisi")
	}
	instance, err := d.Approval.InstanceBySubject(ctx, approval.SubjectPromotionPlan, planID)
	if err != nil {
		return err
	}
	_, err = d.Approval.Decide(ctx, instance.InstanceID, ip,
		func(in *approval.Instance, chain approval.Chain) (approval.Decision, error) {
			return in.Revive(actorFor(p, in.CompanyID), clean, d.Now())
		})
	if err != nil {
		return mapApprovalError(err)
	}
	if err := d.Promos.SetStatus(ctx, planID, promo.StatusPending, nil, false); err != nil {
		return err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "approval.revive",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		Reason: clean, IP: ip})
	d.notifyStep(ctx, planID)
	return nil
}

// mapApprovalError turns a domain refusal into an API code. A generic 403 for
// all of these is what makes an approval screen impossible to debug.
func mapApprovalError(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), approval.ErrSelfApproval.Error()):
		return apierror.New(apierror.CodeSelfApproval, approval.ErrSelfApproval.Error())
	case strings.Contains(err.Error(), approval.ErrAlreadyDecided.Error()):
		return apierror.New(apierror.CodeAlreadyDecided, approval.ErrAlreadyDecided.Error())
	case strings.Contains(err.Error(), approval.ErrNotYourStep.Error()):
		return apierror.New(apierror.CodeNotYourStep, approval.ErrNotYourStep.Error())
	case strings.Contains(err.Error(), approval.ErrWrongCompany.Error()):
		return apierror.New(apierror.CodeNotYourStep, approval.ErrWrongCompany.Error())
	case strings.Contains(err.Error(), approval.ErrReasonRequired.Error()):
		return apierror.New(apierror.CodeReasonRequired, approval.ErrReasonRequired.Error())
	case strings.Contains(err.Error(), approval.ErrNotPending.Error()):
		return apierror.New(apierror.CodeConflict, approval.ErrNotPending.Error())
	case strings.Contains(err.Error(), approval.ErrNotCancelled.Error()):
		return apierror.New(apierror.CodeConflict, approval.ErrNotCancelled.Error())
	default:
		return err
	}
}

// Notifications -------------------------------------------------------------
//
// A failed send must never block an approval: the decision has already
// committed when these run, and they queue rather than deliver.

func (d *Deps) notifyStep(ctx context.Context, planID uuid.UUID) {
	row, err := d.Promos.ByID(ctx, planID)
	if err != nil || d.Notify == nil {
		return
	}
	subject := fmt.Sprintf("[Marketing Calendar] %s menunggu persetujuan Anda", row.PlanCode)
	body := fmt.Sprintf(
		"Rencana promo %s (%s) menunggu pada langkah %d — %s.\n\nPeriode: %s s/d %s\nKelompok toko: %s\nTarget: Rp %s / %d struk\n",
		row.PlanCode, row.Version.PromoName, row.CurrentStepNo, row.CurrentStepName,
		row.Version.StartDate.Format("2006-01-02"), row.Version.EndDate.Format("2006-01-02"),
		row.SiteGroupName, row.Version.TargetSalesIDR.Format(), row.Version.TargetReceiptCount)
	// The pending approvers are addressed by role, so the recipient list comes
	// from the chain rather than from a stored list that goes stale.
	recipients := d.pendingApproverEmails(ctx, planID)
	if len(recipients) == 0 {
		return
	}
	_ = d.Notify.Queue(ctx, recipients, subject, body, "promotion_plan", &planID)
}

func (d *Deps) pendingApproverEmails(ctx context.Context, planID uuid.UUID) []string {
	var out []string
	rows, err := d.DB.WithContext(ctx).Raw(`
		SELECT DISTINCT u.email
		  FROM promotion_plan p
		  JOIN approval_instance ai ON ai.instance_id = p.approval_instance_id
		  JOIN approval_step s ON s.version_id = ai.version_id AND s.step_no = ai.current_step_no
		  JOIN approval_step_role sr ON sr.step_id = s.step_id
		  JOIN user_role ur ON ur.role_id = sr.role_id AND ur.company_id = ai.company_id
		  JOIN app_user u ON u.user_id = ur.user_id AND u.is_active
		 WHERE p.plan_id = ? AND ai.status = 'PENDING' AND u.user_id <> p.created_by`,
		planID).Rows()
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err == nil {
			out = append(out, e)
		}
	}
	return out
}

// notifyRelease goes to the FIXED maintained list and nobody else (D32).
// Chain actors are deliberately not appended: Steven chose the narrower rule,
// so the list is exactly the list.
func (d *Deps) notifyRelease(ctx context.Context, planID uuid.UUID) {
	row, err := d.Promos.ByID(ctx, planID)
	if err != nil || d.Notify == nil {
		return
	}
	recipients := d.Params.List(ctx, ParamReleaseRecipients)
	if len(recipients) == 0 {
		d.Log.Warn("release notification has no recipients",
			"param", ParamReleaseRecipients, "plan", row.PlanCode)
		return
	}
	tag := ""
	if row.ForceReleased {
		tag = " [RILIS PAKSA]"
	}
	subject := fmt.Sprintf("[Marketing Calendar] %s dirilis%s", row.PlanCode, tag)
	body := fmt.Sprintf(
		"%s — %s telah dirilis.\n\nMerek: %s\nPeriode: %s s/d %s\nKelompok toko: %s\nMode: %s\nTarget: Rp %s / %d struk\n\nAturan promo:\n%s\n",
		row.PlanCode, row.Version.PromoName, row.CompanyName,
		row.Version.StartDate.Format("2006-01-02"), row.Version.EndDate.Format("2006-01-02"),
		row.SiteGroupName, row.Version.OrderMode,
		row.Version.TargetSalesIDR.Format(), row.Version.TargetReceiptCount, row.Version.PromoRule)
	_ = d.Notify.Queue(ctx, recipients, subject, body, "promotion_plan", &planID)
}

func (d *Deps) notifyRejected(ctx context.Context, planID uuid.UUID, reason string) {
	row, err := d.Promos.ByID(ctx, planID)
	if err != nil || d.Notify == nil {
		return
	}
	var email string
	if err := d.DB.WithContext(ctx).Raw(
		`SELECT email FROM app_user WHERE user_id = ?`, row.Plan.CreatedBy).Scan(&email).Error; err != nil || email == "" {
		return
	}
	_ = d.Notify.Queue(ctx, []string{email},
		fmt.Sprintf("[Marketing Calendar] %s ditolak", row.PlanCode),
		fmt.Sprintf("%s — %s ditolak.\n\nAlasan:\n%s\n\nAnda dapat memperbaiki dan mengajukan ulang.\n",
			row.PlanCode, row.Version.PromoName, reason),
		"promotion_plan", &planID)
}
