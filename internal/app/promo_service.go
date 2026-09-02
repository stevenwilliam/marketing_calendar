package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

type PromoInput struct {
	CompanyID          uuid.UUID
	SiteGroupID        uuid.UUID
	PromoName          string
	StartDate          time.Time
	EndDate            time.Time
	TargetSalesIDR     money.IDR
	TargetReceiptCount int
	OrderMode          string
	PromoRule          string
	AcknowledgeOverlap bool
	OverrideLeadTime   bool
	OverrideReason     string
}

func (in *PromoInput) normalise() (map[string]string, error) {
	fields := map[string]string{}
	var err error
	if in.PromoName, err = sanitize.Text(in.PromoName, 200); err != nil {
		fields["promo_name"] = err.Error()
	}
	if in.PromoRule, err = sanitize.Text(in.PromoRule, 5000); err != nil {
		fields["promo_rule"] = err.Error()
	}
	if in.OrderMode, err = sanitize.Enum(in.OrderMode, "dine_in", "take_away"); err != nil {
		fields["order_mode"] = "mode pesanan harus dine_in atau take_away"
	}
	if in.OverrideReason != "" {
		if in.OverrideReason, err = sanitize.Text(in.OverrideReason, 1000); err != nil {
			fields["override_reason"] = err.Error()
		}
	}
	if len(fields) > 0 {
		return fields, apierror.Validation("periksa kembali isian", fields)
	}
	return nil, nil
}

func (d *Deps) toVersion(in PromoInput, by uuid.UUID) promo.Version {
	return promo.Version{
		PromoName: in.PromoName, StartDate: calendar.Today(in.StartDate),
		EndDate: calendar.Today(in.EndDate), TargetSalesIDR: in.TargetSalesIDR,
		TargetReceiptCount: in.TargetReceiptCount, OrderMode: promo.OrderMode(in.OrderMode),
		PromoRule: in.PromoRule, OverlapAcknowledged: in.AcknowledgeOverlap,
		LeadTimeOverridden: in.OverrideLeadTime, LeadTimeOverrideReason: in.OverrideReason,
		CreatedBy: by,
	}
}

// CreatePlan writes a DRAFT. Validation here is for feedback; the gate that
// matters is Submit (BR-3.1: a draft may be incomplete).
func (d *Deps) CreatePlan(ctx context.Context, p Principal, in PromoInput, ip string) (uuid.UUID, error) {
	if !p.Can("promo.create") {
		return uuid.Nil, apierror.New(apierror.CodeForbidden, "tidak memiliki izin promo.create")
	}
	if !p.InCompany(in.CompanyID) {
		return uuid.Nil, apierror.NotFound("perusahaan")
	}
	if _, err := in.normalise(); err != nil {
		return uuid.Nil, err
	}
	// The group must belong to the same company. The database refuses it too
	// (D31); this names it so the user gets a sentence, not a constraint.
	g, err := d.Master.SiteGroupByID(ctx, in.SiteGroupID)
	if err != nil {
		return uuid.Nil, err
	}
	if g.CompanyID != in.CompanyID {
		return uuid.Nil, apierror.New(apierror.CodeCrossBrand,
			"kelompok toko bukan milik perusahaan ini")
	}

	code, err := d.Promos.NextPlanCode(ctx, calendar.Today(d.Now()).Year())
	if err != nil {
		return uuid.Nil, err
	}
	plan := promo.Plan{CompanyID: in.CompanyID, PlanCode: code,
		SiteGroupID: in.SiteGroupID, Status: promo.StatusDraft, CreatedBy: p.UserID}
	planID, err := d.Promos.Create(ctx, plan, d.toVersion(in, p.UserID))
	if err != nil {
		return uuid.Nil, err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "promo.create",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &in.CompanyID,
		After: in, IP: ip})
	return planID, nil
}

// UpdatePlan edits a DRAFT or REJECTED plan in place, or creates a NEW VERSION
// when the plan is locked (BR-4.7). The caller does not choose which; the
// plan's status does.
func (d *Deps) UpdatePlan(ctx context.Context, p Principal, planID uuid.UUID, in PromoInput, ip string) error {
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return err
	}
	if !p.Can("promo.create") && !p.Can("promo.manage") {
		return apierror.New(apierror.CodeForbidden, "tidak memiliki izin mengubah promo")
	}
	if _, err := in.normalise(); err != nil {
		return err
	}

	v := d.toVersion(in, p.UserID)
	if err := row.Plan.CanEdit(); err != nil {
		// Locked: a new version, re-entering the chain from step 1.
		next := row.Version.NextVersion(d.Now(), p.UserID)
		next.PromoName, next.StartDate, next.EndDate = v.PromoName, v.StartDate, v.EndDate
		next.TargetSalesIDR, next.TargetReceiptCount = v.TargetSalesIDR, v.TargetReceiptCount
		next.OrderMode, next.PromoRule = v.OrderMode, v.PromoRule
		if err := d.Promos.NewVersion(ctx, planID, next); err != nil {
			return err
		}
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "promo.new_version",
			SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
			Before: row.Version, After: next, IP: ip,
			Reason: "rencana terkunci; perubahan membuat versi baru (BR-4.7)"})
		return nil
	}

	v.VersionID = row.Version.VersionID
	if err := d.Promos.SaveDraftVersion(ctx, v); err != nil {
		return err
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "promo.update",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		Before: row.Version, After: v, IP: ip})
	return nil
}

// SubmitResult carries what the UI must say when a submit is refused.
type SubmitOutcome struct {
	Overlaps      []promo.Overlap
	EarliestStart time.Time
	Fields        map[string]string
}

// Submit runs the BR-3.3 and BR-3.6 gates, opens an approval instance and
// moves the plan to PENDING.
func (d *Deps) Submit(ctx context.Context, p Principal, planID uuid.UUID, ack bool, ip string) (*SubmitOutcome, error) {
	row, err := d.loadPlanScoped(ctx, p, planID)
	if err != nil {
		return nil, err
	}
	if row.Plan.CreatedBy != p.UserID && !p.Can("promo.manage") {
		return nil, apierror.New(apierror.CodeForbidden, "hanya pembuat atau marketing head yang dapat mengajukan")
	}

	holidays, err := d.Master.HolidaySet(ctx)
	if err != nil {
		return nil, err
	}
	leadDays := d.Params.Int(ctx, ParamLeadTimeDays, 7)

	var overlaps []promo.Overlap
	if d.Params.Bool(ctx, ParamOverlapWarning, true) {
		overlaps, err = d.Promos.Overlaps(ctx, row.CompanyID, row.SiteGroupID, planID,
			row.Version.StartDate, row.Version.EndDate)
		if err != nil {
			return nil, err
		}
	}

	v := row.Version
	if ack {
		v.OverlapAcknowledged = true
	}
	check := promo.SubmitCheck{Now: d.Now(), Holidays: holidays,
		LeadTimeWorkingDays: leadDays, Overlaps: overlaps, IsSuperadmin: p.IsSuperadmin}

	res, err := v.CanSubmit(row.Plan, check)
	out := &SubmitOutcome{Overlaps: res.Overlaps, EarliestStart: res.EarliestStart, Fields: res.Fields}
	if err != nil {
		switch {
		case res.LeadTimeFailed:
			return out, &apierror.Error{Code: apierror.CodeLeadTime,
				Message: "tanggal mulai lebih awal dari masa tenggang",
				Fields: map[string]string{"start_date": "paling cepat " +
					res.EarliestStart.Format("2006-01-02") + " (" + itoa(leadDays) + " hari kerja)"}}
		case len(res.Overlaps) > 0:
			return out, apierror.New(apierror.CodeOverlap,
				"ada promo lain yang tumpang tindih pada toko yang sama; konfirmasi untuk melanjutkan")
		case res.Fields != nil:
			return out, apierror.Validation("lengkapi isian sebelum mengajukan", res.Fields)
		default:
			return out, apierror.New(apierror.CodeConflict, err.Error())
		}
	}

	if ack && !row.Version.OverlapAcknowledged {
		if err := d.Promos.SaveDraftVersion(ctx, v); err != nil {
			return out, err
		}
	}

	chain, _, err := d.Approval.ActiveChain(ctx, row.CompanyID, approval.SubjectPromotionPlan)
	if err != nil {
		return out, err
	}
	instanceID, err := d.Approval.OpenInstance(ctx, approval.Instance{
		VersionID: chain.VersionID, CompanyID: row.CompanyID,
		SubjectType: approval.SubjectPromotionPlan, SubjectID: planID,
		CreatedBy: p.UserID})
	if err != nil {
		return out, err
	}
	if err := d.Promos.SetStatus(ctx, planID, promo.StatusPending, &instanceID, false); err != nil {
		return out, err
	}

	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &p.UserID, Action: "promo.submit",
		SubjectType: "promotion_plan", SubjectID: &planID, CompanyID: &row.CompanyID,
		After: map[string]any{"instance_id": instanceID, "chain_version": chain.VersionID}, IP: ip})
	d.notifyStep(ctx, planID)
	return out, nil
}

// loadPlanScoped is the one place a plan is fetched. Asking for another
// company's plan returns 404, not 403: the existence of the row is not
// disclosed (12-security.md §4).
func (d *Deps) loadPlanScoped(ctx context.Context, p Principal, planID uuid.UUID) (*PlanRow, error) {
	row, err := d.Promos.ByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if !p.InCompany(row.CompanyID) {
		return nil, apierror.NotFound("rencana promo")
	}
	return row, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
