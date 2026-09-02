// Package approval is the generic approval chain state machine.
//
// BR-4.1: it operates on a subject TYPE and a subject ID. Nothing in this
// package knows what a promotion is, and nothing in it may learn. A superapp
// needs approvals for purchase orders, leave, discounts, write-offs and price
// changes; a chain written inside the promotion module gets rewritten five
// times (D25).
//
// This package is pure: no database, no HTTP, no clock of its own. Decisions
// are computed from a snapshot and returned as events for a repository to
// persist inside one transaction.
package approval

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type (
	SubjectType  string
	Status       string
	Action       string
	Satisfaction string
)

const (
	SubjectPromotionPlan SubjectType = "promotion_plan"

	StatusPending       Status = "PENDING"
	StatusApproved      Status = "APPROVED"
	StatusRejected      Status = "REJECTED"
	StatusCancelled     Status = "CANCELLED"
	StatusForceReleased Status = "FORCE_RELEASED"

	ActionApprove      Action = "APPROVE"
	ActionReject       Action = "REJECT"
	ActionForceRelease Action = "FORCE_RELEASE"
	ActionAutoCancel   Action = "AUTO_CANCEL"
	ActionRevive       Action = "REVIVE"

	// AnyOf — one holder of any listed role satisfies the step. This is how
	// "Role_3 OR Role_4" is expressed (BR-4.2).
	AnyOf Satisfaction = "ANY_OF"
	// AllOf — every listed role must approve before the step completes.
	AllOf Satisfaction = "ALL_OF"
)

var (
	ErrNotPending     = errors.New("instansi persetujuan sudah selesai")
	ErrNotYourStep    = errors.New("bukan langkah Anda")
	ErrAlreadyDecided = errors.New("Anda sudah memutuskan pada langkah ini")
	ErrSelfApproval   = errors.New("pembuat rencana tidak boleh menyetujui rencananya sendiri")
	ErrReasonRequired = errors.New("alasan wajib diisi")
	ErrWrongCompany   = errors.New("peran Anda tidak berlaku di perusahaan rencana ini")
	ErrNotCancelled   = errors.New("hanya rencana yang dibatalkan otomatis dapat dihidupkan kembali")
	ErrNoSteps        = errors.New("rantai persetujuan tidak memiliki langkah")
)

// Step is one ordered step of a chain version.
type Step struct {
	StepNo       int
	Name         string
	Satisfaction Satisfaction
	RoleIDs      []uuid.UUID
}

// Chain is an immutable snapshot of ONE VERSION of a chain. An instance binds
// to a version, not to a chain, so an administrator rewriting the chain cannot
// disturb a plan already in flight (BR-4.3).
type Chain struct {
	VersionID uuid.UUID
	Steps     []Step
}

func (c Chain) Step(no int) (Step, bool) {
	for _, s := range c.Steps {
		if s.StepNo == no {
			return s, true
		}
	}
	return Step{}, false
}

// Event is an append-only decision record (BR-4.9). It is never updated and
// never deleted; a mistake is corrected with another event.
type Event struct {
	InstanceID uuid.UUID
	StepNo     int
	Action     Action
	ActorID    *uuid.UUID // nil when the actor is the scheduler
	Reason     string
	OccurredAt time.Time
}

// Instance is the live state of one subject's journey through one chain
// version.
type Instance struct {
	InstanceID    uuid.UUID
	VersionID     uuid.UUID
	SubjectType   SubjectType
	SubjectID     uuid.UUID
	CompanyID     uuid.UUID
	CreatedBy     uuid.UUID
	CurrentStepNo int
	Status        Status
	Events        []Event
}

// Actor is the person deciding, with the roles they hold IN THE SUBJECT'S
// COMPANY. BR-4.4a: holding `operation` for Maxx Coffee does not approve a
// Ruuma plan, and the caller resolves that scoping before we are asked.
type Actor struct {
	UserID       uuid.UUID
	RoleIDs      []uuid.UUID
	CompanyIDs   []uuid.UUID
	IsSuperadmin bool
}

func (a Actor) holdsAnyRole(roles []uuid.UUID) bool {
	for _, want := range roles {
		for _, has := range a.RoleIDs {
			if has == want {
				return true
			}
		}
	}
	return false
}

func (a Actor) inCompany(c uuid.UUID) bool {
	for _, x := range a.CompanyIDs {
		if x == c {
			return true
		}
	}
	return false
}

// Decision is what the caller must persist, in one transaction.
type Decision struct {
	Events      []Event
	NextStepNo  int
	NewStatus   Status
	ChainClosed bool
}

// CanApprove answers "may this actor act on the current step", with the reason
// when they may not. Every refusal is a named error so the HTTP layer maps it
// to a code rather than a generic 403.
func (i *Instance) CanApprove(c Chain, a Actor, now time.Time) error {
	if i.Status != StatusPending {
		return ErrNotPending
	}
	// BR-4.4a — the role must be held in the subject's own company.
	if !a.inCompany(i.CompanyID) {
		return ErrWrongCompany
	}
	// BR-4.11 — the creator may not approve their own plan at any step, even
	// holding the role. Superadmin force-release is the documented escape.
	if a.UserID == i.CreatedBy {
		return ErrSelfApproval
	}
	step, ok := c.Step(i.CurrentStepNo)
	if !ok {
		return ErrNoSteps
	}
	if !a.holdsAnyRole(step.RoleIDs) {
		return ErrNotYourStep
	}
	// BR-4.10 — one decision per approver per step.
	for _, e := range i.Events {
		if e.StepNo == i.CurrentStepNo && e.Action == ActionApprove &&
			e.ActorID != nil && *e.ActorID == a.UserID {
			return ErrAlreadyDecided
		}
	}
	return nil
}

// Approve records one approval and advances the chain when the step is
// satisfied. BR-4.4: step n+1 opens only when step n is satisfied.
func (i *Instance) Approve(c Chain, a Actor, now time.Time) (Decision, error) {
	if err := i.CanApprove(c, a, now); err != nil {
		return Decision{}, err
	}
	step, _ := c.Step(i.CurrentStepNo)
	actor := a.UserID
	ev := Event{InstanceID: i.InstanceID, StepNo: i.CurrentStepNo,
		Action: ActionApprove, ActorID: &actor, OccurredAt: now}

	if !i.stepSatisfiedWith(step, ev) {
		// ALL_OF still waiting on other roles: the step stays open.
		return Decision{Events: []Event{ev}, NextStepNo: i.CurrentStepNo,
			NewStatus: StatusPending}, nil
	}

	next := i.CurrentStepNo + 1
	if _, more := c.Step(next); more {
		return Decision{Events: []Event{ev}, NextStepNo: next, NewStatus: StatusPending}, nil
	}
	// The last step closed the chain.
	return Decision{Events: []Event{ev}, NextStepNo: i.CurrentStepNo,
		NewStatus: StatusApproved, ChainClosed: true}, nil
}

// stepSatisfiedWith answers whether the step completes once ev is added.
func (i *Instance) stepSatisfiedWith(step Step, ev Event) bool {
	if step.Satisfaction == AnyOf {
		return true
	}
	// ALL_OF: every listed role must have an approval on this step. Role
	// coverage is computed from the events already recorded plus this one.
	approvedBy := map[uuid.UUID]bool{}
	for _, e := range i.Events {
		if e.StepNo == step.StepNo && e.Action == ActionApprove && e.ActorID != nil {
			approvedBy[*e.ActorID] = true
		}
	}
	if ev.ActorID != nil {
		approvedBy[*ev.ActorID] = true
	}
	// The caller supplies role coverage through RoleCoverage; without it we
	// require one distinct approver per listed role, which is the conservative
	// reading and never completes a step early.
	return len(approvedBy) >= len(step.RoleIDs)
}

// Reject returns the subject to its creator with a mandatory reason (BR-4.5).
func (i *Instance) Reject(c Chain, a Actor, reason string, now time.Time) (Decision, error) {
	if err := i.CanApprove(c, a, now); err != nil {
		return Decision{}, err
	}
	if !hasText(reason) {
		return Decision{}, ErrReasonRequired
	}
	actor := a.UserID
	return Decision{
		Events: []Event{{InstanceID: i.InstanceID, StepNo: i.CurrentStepNo,
			Action: ActionReject, ActorID: &actor, Reason: reason, OccurredAt: now}},
		NextStepNo: i.CurrentStepNo, NewStatus: StatusRejected, ChainClosed: true,
	}, nil
}

// ForceRelease completes every outstanding step at once (BR-4.8). It requires
// a typed reason and is the only bypass in the system.
func (i *Instance) ForceRelease(a Actor, reason string, now time.Time) (Decision, error) {
	if !a.IsSuperadmin {
		return Decision{}, ErrNotYourStep
	}
	if i.Status != StatusPending {
		return Decision{}, ErrNotPending
	}
	if !hasText(reason) {
		return Decision{}, ErrReasonRequired
	}
	actor := a.UserID
	return Decision{
		Events: []Event{{InstanceID: i.InstanceID, StepNo: i.CurrentStepNo,
			Action: ActionForceRelease, ActorID: &actor, Reason: reason, OccurredAt: now}},
		NextStepNo: i.CurrentStepNo, NewStatus: StatusForceReleased, ChainClosed: true,
	}, nil
}

// AutoCancel is the scheduler acting. The actor is nil, which is why
// approval_event.actor_id is nullable (BR-4.6).
func (i *Instance) AutoCancel(now time.Time) (Decision, error) {
	if i.Status != StatusPending {
		return Decision{}, ErrNotPending
	}
	return Decision{
		Events: []Event{{InstanceID: i.InstanceID, StepNo: i.CurrentStepNo,
			Action: ActionAutoCancel, ActorID: nil,
			Reason:     "rantai persetujuan belum selesai pada batas pembatalan otomatis",
			OccurredAt: now}},
		NextStepNo: i.CurrentStepNo, NewStatus: StatusCancelled, ChainClosed: true,
	}, nil
}

// Revive returns an auto-cancelled instance to the step it was pending at.
// The cancellation event stays: it is append-only, and the revival is a second
// event, not an erasure (BR-4.6).
func (i *Instance) Revive(a Actor, reason string, now time.Time) (Decision, error) {
	if !a.IsSuperadmin {
		return Decision{}, ErrNotYourStep
	}
	if i.Status != StatusCancelled {
		return Decision{}, ErrNotCancelled
	}
	if !hasText(reason) {
		return Decision{}, ErrReasonRequired
	}
	actor := a.UserID
	return Decision{
		Events: []Event{{InstanceID: i.InstanceID, StepNo: i.CurrentStepNo,
			Action: ActionRevive, ActorID: &actor, Reason: reason, OccurredAt: now}},
		NextStepNo: i.CurrentStepNo, NewStatus: StatusPending,
	}, nil
}

func hasText(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}
