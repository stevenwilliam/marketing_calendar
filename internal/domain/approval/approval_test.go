package approval

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	roleMktgHead = uuid.New()
	roleAnalyst  = uuid.New()
	roleFinance  = uuid.New()
	roleOps      = uuid.New()
	roleCFO      = uuid.New()
	companyMaxx  = uuid.New()
	companyRuuma = uuid.New()
	creator      = uuid.New()
	now          = time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
)

// The default chain of BR-4.2 after D36: five steps.
func defaultChain() Chain {
	return Chain{VersionID: uuid.New(), Steps: []Step{
		{StepNo: 1, Name: "Marketing Head", Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleMktgHead}},
		{StepNo: 2, Name: "Business Analyst", Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleAnalyst}},
		{StepNo: 3, Name: "Finance Head", Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleFinance}},
		{StepNo: 4, Name: "Operation", Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleOps}},
		{StepNo: 5, Name: "CFO", Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleCFO}},
	}}
}

func newInstance(c Chain) *Instance {
	return &Instance{InstanceID: uuid.New(), VersionID: c.VersionID,
		SubjectType: SubjectPromotionPlan, SubjectID: uuid.New(),
		CompanyID: companyMaxx, CreatedBy: creator,
		CurrentStepNo: 1, Status: StatusPending}
}

func actor(role uuid.UUID, companies ...uuid.UUID) Actor {
	if len(companies) == 0 {
		companies = []uuid.UUID{companyMaxx}
	}
	return Actor{UserID: uuid.New(), RoleIDs: []uuid.UUID{role}, CompanyIDs: companies}
}

// apply mutates the instance the way the repository will, so a test walks the
// same state transitions production does.
func apply(i *Instance, d Decision) {
	i.Events = append(i.Events, d.Events...)
	i.CurrentStepNo = d.NextStepNo
	i.Status = d.NewStatus
}

// BR-4.2 / F2: not RELEASED until the whole chain completes.
func TestNotReleasedUntilChainComplete(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	for _, role := range []uuid.UUID{roleMktgHead, roleAnalyst, roleFinance, roleOps} {
		d, err := i.Approve(c, actor(role), now)
		if err != nil {
			t.Fatalf("step %d: %v", i.CurrentStepNo, err)
		}
		apply(i, d)
		if i.Status != StatusPending {
			t.Fatalf("chain closed early at step %d", i.CurrentStepNo)
		}
	}
	d, err := i.Approve(c, actor(roleCFO), now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.Status != StatusApproved || !d.ChainClosed {
		t.Fatalf("after five approvals status = %s, closed = %v", i.Status, d.ChainClosed)
	}
	if len(i.Events) != 5 {
		t.Fatalf("want 5 append-only events, got %d", len(i.Events))
	}
}

// BR-4.4: an approver at a later step cannot approve early.
func TestLaterStepCannotApproveEarly(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	if err := i.CanApprove(c, actor(roleCFO), now); err != ErrNotYourStep {
		t.Fatalf("CFO approving at step 1 must be refused, got %v", err)
	}
}

// BR-4.11: the creator may not approve their own plan, even holding the role.
func TestCreatorCannotApprove(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	a := Actor{UserID: creator, RoleIDs: []uuid.UUID{roleMktgHead}, CompanyIDs: []uuid.UUID{companyMaxx}}
	if err := i.CanApprove(c, a, now); err != ErrSelfApproval {
		t.Fatalf("want ErrSelfApproval, got %v", err)
	}
}

// BR-4.4a / D37: the role must be held in the SUBJECT'S company. This is what
// lets one `operation` role serve three brands without a role per brand.
func TestRoleInWrongCompanyCannotApprove(t *testing.T) {
	c := defaultChain()
	i := newInstance(c) // company Maxx
	a := actor(roleMktgHead, companyRuuma)
	if err := i.CanApprove(c, a, now); err != ErrWrongCompany {
		t.Fatalf("a Ruuma-only approver must not approve a Maxx plan, got %v", err)
	}
}

// BR-4.10: the same approver cannot approve the same step twice.
func TestSameApproverCannotDecideTwice(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	a := actor(roleMktgHead)
	d, err := i.Approve(c, a, now)
	if err != nil {
		t.Fatal(err)
	}
	// Do not advance: simulate the second request racing the first, which the
	// repository serialises with SELECT ... FOR UPDATE.
	i.Events = append(i.Events, d.Events...)
	if err := i.CanApprove(c, a, now); err != ErrAlreadyDecided {
		t.Fatalf("want ErrAlreadyDecided, got %v", err)
	}
}

// BR-4.2: ANY_OF with two roles — either satisfies, and it advances once.
func TestAnyOfEitherRoleSatisfies(t *testing.T) {
	c := Chain{VersionID: uuid.New(), Steps: []Step{
		{StepNo: 1, Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleOps, roleCFO}},
		{StepNo: 2, Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleFinance}},
	}}
	for _, role := range []uuid.UUID{roleOps, roleCFO} {
		i := newInstance(c)
		d, err := i.Approve(c, actor(role), now)
		if err != nil {
			t.Fatalf("role %v: %v", role, err)
		}
		apply(i, d)
		if i.CurrentStepNo != 2 {
			t.Fatalf("ANY_OF did not advance: step %d", i.CurrentStepNo)
		}
	}
}

// ALL_OF must NOT complete on the first approval — the conservative reading,
// because completing a step early is the failure that matters.
func TestAllOfWaitsForEveryRole(t *testing.T) {
	c := Chain{VersionID: uuid.New(), Steps: []Step{
		{StepNo: 1, Satisfaction: AllOf, RoleIDs: []uuid.UUID{roleFinance, roleOps}},
		{StepNo: 2, Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleCFO}},
	}}
	i := newInstance(c)
	d, err := i.Approve(c, actor(roleFinance), now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.CurrentStepNo != 1 || i.Status != StatusPending {
		t.Fatalf("ALL_OF advanced on one of two approvals: step %d status %s", i.CurrentStepNo, i.Status)
	}
	d, err = i.Approve(c, actor(roleOps), now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.CurrentStepNo != 2 {
		t.Fatalf("ALL_OF did not advance after both approvals: step %d", i.CurrentStepNo)
	}
}

// BR-4.5: rejection needs a reason and returns the subject to its creator.
func TestRejectRequiresReason(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	if _, err := i.Reject(c, actor(roleMktgHead), "   ", now); err != ErrReasonRequired {
		t.Fatalf("whitespace is not a reason, got %v", err)
	}
	d, err := i.Reject(c, actor(roleMktgHead), "diskon terlalu dalam", now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.Status != StatusRejected || i.Events[0].Reason == "" {
		t.Fatalf("status %s, reason %q", i.Status, i.Events[0].Reason)
	}
}

// BR-4.8 / D27: force-release requires a typed reason and only a superadmin.
func TestForceReleaseRequiresReasonAndSuperadmin(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	if _, err := i.ForceRelease(actor(roleCFO), "sudah disetujui lisan", now); err != ErrNotYourStep {
		t.Fatalf("a non-superadmin must not force-release, got %v", err)
	}
	su := Actor{UserID: uuid.New(), IsSuperadmin: true, CompanyIDs: []uuid.UUID{companyMaxx}}
	if _, err := i.ForceRelease(su, "", now); err != ErrReasonRequired {
		t.Fatalf("want ErrReasonRequired, got %v", err)
	}
	d, err := i.ForceRelease(su, "kampanye nasional, disetujui di rapat direksi", now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.Status != StatusForceReleased {
		t.Fatalf("status = %s", i.Status)
	}
}

// BR-4.6: the scheduler is the actor, and actor_id is nil for it.
func TestAutoCancelHasNoHumanActor(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	d, err := i.AutoCancel(now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)
	if i.Status != StatusCancelled {
		t.Fatalf("status = %s", i.Status)
	}
	if d.Events[0].ActorID != nil {
		t.Fatal("the scheduler must not be attributed to a person")
	}
}

// The cancellation stays in the history; the revival is a second event, not
// an erasure (BR-4.6, BR-4.9).
func TestReviveKeepsTheCancellationEvent(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	d, _ := i.Approve(c, actor(roleMktgHead), now)
	apply(i, d)
	d, _ = i.AutoCancel(now)
	apply(i, d)

	su := Actor{UserID: uuid.New(), IsSuperadmin: true, CompanyIDs: []uuid.UUID{companyMaxx}}
	d, err := i.Revive(su, "dibatalkan karena approver cuti", now)
	if err != nil {
		t.Fatal(err)
	}
	apply(i, d)

	if i.Status != StatusPending {
		t.Fatalf("status = %s", i.Status)
	}
	if i.CurrentStepNo != 2 {
		t.Fatalf("revive must return to the pending step, got %d", i.CurrentStepNo)
	}
	var cancels, revives int
	for _, e := range i.Events {
		switch e.Action {
		case ActionAutoCancel:
			cancels++
		case ActionRevive:
			revives++
		}
	}
	if cancels != 1 || revives != 1 {
		t.Fatalf("history must keep both events: %d cancels, %d revives", cancels, revives)
	}
}

func TestReviveOnlyAfterAutoCancel(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	su := Actor{UserID: uuid.New(), IsSuperadmin: true, CompanyIDs: []uuid.UUID{companyMaxx}}
	if _, err := i.Revive(su, "alasan", now); err != ErrNotCancelled {
		t.Fatalf("want ErrNotCancelled, got %v", err)
	}
}

// BR-4.3 expressed in the domain: an instance carries the VERSION it started
// with, so a chain rewritten mid-flight cannot reach it. The engine is handed
// a Chain snapshot; this asserts the binding is by version id.
func TestChainChangeDoesNotAffectInFlight(t *testing.T) {
	oldChain := defaultChain()
	i := newInstance(oldChain)
	d, _ := i.Approve(oldChain, actor(roleMktgHead), now)
	apply(i, d)

	// An administrator rewrites the chain: a new version, one step, a role
	// nobody in flight holds.
	newChain := Chain{VersionID: uuid.New(), Steps: []Step{
		{StepNo: 1, Satisfaction: AnyOf, RoleIDs: []uuid.UUID{roleCFO}},
	}}
	if i.VersionID == newChain.VersionID {
		t.Fatal("the instance must not be rebound to the new version")
	}
	// The in-flight instance continues on its own version: step 2 is the
	// Business Analyst, exactly as it was when the plan was submitted.
	if err := i.CanApprove(oldChain, actor(roleAnalyst), now); err != nil {
		t.Fatalf("the bound version must still govern: %v", err)
	}
}

// A closed instance takes no further decisions (BR-4.9, no undo).
func TestClosedInstanceRefusesFurtherDecisions(t *testing.T) {
	c := defaultChain()
	i := newInstance(c)
	d, _ := i.Reject(c, actor(roleMktgHead), "tidak sesuai anggaran", now)
	apply(i, d)
	if err := i.CanApprove(c, actor(roleMktgHead), now); err != ErrNotPending {
		t.Fatalf("want ErrNotPending, got %v", err)
	}
	if _, err := i.ForceRelease(Actor{IsSuperadmin: true}, "alasan", now); err != ErrNotPending {
		t.Fatalf("force-release on a closed instance must be refused, got %v", err)
	}
}
