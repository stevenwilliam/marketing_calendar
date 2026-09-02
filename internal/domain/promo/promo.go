// Package promo is the promotion plan lifecycle (BR-3, BR-4.7).
//
// The load-bearing decision: the substantive fields live on a VERSION, not on
// the plan. Approval locks the plan, and an edit creates a new version that
// re-enters the chain at step 1 (BR-4.7, D12). An approved version is simply
// never written to again — the immutability is structural, not a guard someone
// has to remember to write.
package promo

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
)

type (
	Status    string
	OrderMode string
)

const (
	StatusDraft     Status = "DRAFT"
	StatusPending   Status = "PENDING"
	StatusReleased  Status = "RELEASED"
	StatusRejected  Status = "REJECTED"
	StatusCancelled Status = "CANCELLED"

	// BR-3.4: a promotion applies to exactly one mode. A promotion for both is
	// two plans, which keeps measurement unambiguous.
	DineIn   OrderMode = "dine_in"
	TakeAway OrderMode = "take_away"
)

var (
	ErrNameRequired   = errors.New("nama promo wajib diisi")
	ErrRuleRequired   = errors.New("aturan promo wajib diisi")
	ErrSiteGroup      = errors.New("kelompok toko wajib dipilih")
	ErrDateOrder      = errors.New("tanggal selesai harus sama atau setelah tanggal mulai")
	ErrNegativeTarget = errors.New("target tidak boleh negatif")
	ErrOrderMode      = errors.New("mode pesanan harus dine_in atau take_away")
	ErrLocked         = errors.New("rencana yang sudah disetujui tidak dapat diubah")
	ErrNotSubmittable = errors.New("hanya draf atau rencana ditolak yang dapat diajukan")
	ErrOverlapUnacked = errors.New("ada promo yang tumpang tindih dan belum diakui")
)

// Version carries every substantive field. Nothing here is editable once the
// version has entered an approved chain.
type Version struct {
	VersionID              uuid.UUID
	PlanID                 uuid.UUID
	VersionNo              int
	PromoName              string
	StartDate              time.Time
	EndDate                time.Time
	TargetSalesIDR         money.IDR
	TargetReceiptCount     int
	OrderMode              OrderMode
	PromoRule              string
	OverlapAcknowledged    bool
	LeadTimeOverridden     bool
	LeadTimeOverrideReason string
	CreatedBy              uuid.UUID
	CreatedAt              time.Time
}

// Plan is the stable identity; status and the pointer to the current version.
type Plan struct {
	PlanID           uuid.UUID
	CompanyID        uuid.UUID
	PlanCode         string
	SiteGroupID      uuid.UUID
	CurrentVersionID uuid.UUID
	Status           Status
	ForceReleased    bool
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
}

// ValidateForSubmit enforces BR-3.1, BR-3.2 and BR-3.5. A draft may be
// incomplete; this is the gate at submit.
func (v Version) ValidateForSubmit() map[string]string {
	f := map[string]string{}
	if !hasText(v.PromoName) {
		f["promo_name"] = ErrNameRequired.Error()
	}
	if !hasText(v.PromoRule) {
		f["promo_rule"] = ErrRuleRequired.Error()
	}
	if v.StartDate.IsZero() || v.EndDate.IsZero() {
		f["period"] = ErrDateOrder.Error()
	} else if v.EndDate.Before(v.StartDate) {
		f["end_date"] = ErrDateOrder.Error() // BR-3.2
	}
	if v.TargetSalesIDR < 0 {
		f["target_sales_idr"] = ErrNegativeTarget.Error() // BR-3.5
	}
	if v.TargetReceiptCount < 0 {
		f["target_receipt_count"] = ErrNegativeTarget.Error()
	}
	switch v.OrderMode {
	case DineIn, TakeAway:
	default:
		f["order_mode"] = ErrOrderMode.Error() // BR-3.4, allow-list
	}
	if len(f) == 0 {
		return nil
	}
	return f
}

// Overlap is one conflicting promotion, for the acknowledgement warning.
type Overlap struct {
	PlanID        uuid.UUID
	PlanCode      string
	PromoName     string
	StartDate     time.Time
	EndDate       time.Time
	SharedSiteIDs []uuid.UUID
}

// RangesOverlap is inclusive on both ends: a promotion ending on the 10th and
// one starting on the 10th DO overlap, because both run on the 10th. Adjacent
// ranges (end = start - 1 day) do not.
func RangesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}

// SubmitCheck is everything that must hold at submit time (BR-3.3, BR-3.6).
// The lead time is evaluated HERE and not at draft-save, so a draft left open
// overnight does not silently become invalid.
type SubmitCheck struct {
	Now                 time.Time
	Holidays            calendar.HolidaySet
	LeadTimeWorkingDays int
	Overlaps            []Overlap
	// IsSuperadmin permits the lead-time override, which is audited and needs
	// a typed reason (BR-4.8).
	IsSuperadmin bool
}

// SubmitResult says what the caller must do next.
type SubmitResult struct {
	Fields         map[string]string
	EarliestStart  time.Time
	LeadTimeFailed bool
	Overlaps       []Overlap
}

// CanSubmit returns nil when the version may enter the chain.
func (v Version) CanSubmit(plan Plan, c SubmitCheck) (SubmitResult, error) {
	var r SubmitResult

	if plan.Status != StatusDraft && plan.Status != StatusRejected {
		return r, ErrNotSubmittable
	}
	if f := v.ValidateForSubmit(); f != nil {
		r.Fields = f
		return r, errors.New("validasi gagal")
	}

	// BR-3.3 — the lead time, against the holiday calendar.
	r.EarliestStart = c.Holidays.EarliestStart(c.Now, c.LeadTimeWorkingDays)
	if calendar.Today(v.StartDate).Before(r.EarliestStart) {
		overridden := v.LeadTimeOverridden && c.IsSuperadmin && hasText(v.LeadTimeOverrideReason)
		if !overridden {
			r.LeadTimeFailed = true
			return r, calendar.ErrLeadTime
		}
	}

	// BR-3.6 — overlaps are allowed but must be acknowledged.
	if len(c.Overlaps) > 0 && !v.OverlapAcknowledged {
		r.Overlaps = c.Overlaps
		return r, ErrOverlapUnacked
	}
	return r, nil
}

// CanEdit expresses BR-4.7. Once step 1 has been approved the plan's
// substantive fields are immutable; an edit creates a new version.
func (p Plan) CanEdit() error {
	switch p.Status {
	case StatusDraft, StatusRejected:
		return nil
	default:
		// PENDING, RELEASED and CANCELLED all refuse. A PENDING plan is
		// mid-chain: editing it would mean approvers approved different text
		// from the text that releases.
		return ErrLocked
	}
}

// NextVersion derives the successor of an approved or rejected version. The
// previous version is retained and remains readable (BR-4.7, D12); the new one
// re-enters the chain at step 1 with its acknowledgements cleared, because an
// overlap acknowledged against the old dates says nothing about the new ones.
func (v Version) NextVersion(now time.Time, by uuid.UUID) Version {
	n := v
	n.VersionID = uuid.New()
	n.VersionNo = v.VersionNo + 1
	n.CreatedBy = by
	n.CreatedAt = now
	n.OverlapAcknowledged = false
	n.LeadTimeOverridden = false
	n.LeadTimeOverrideReason = ""
	return n
}

// DaysUntilAutoCancel is negative once the plan is past its cancellation date.
func (v Version) AutoCancelDate(m int) time.Time {
	return calendar.AutoCancelDate(v.StartDate, m)
}

// ShouldAutoCancel is the daily job's predicate (BR-4.6): a plan not yet fully
// approved on the Mth day before its start date.
func ShouldAutoCancel(p Plan, v Version, today time.Time, m int) bool {
	if p.Status != StatusPending {
		return false
	}
	return !calendar.Today(today).Before(v.AutoCancelDate(m))
}

func hasText(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}
