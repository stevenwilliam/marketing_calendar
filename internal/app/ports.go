// Package app is the use-case layer. It orchestrates the domain and the ports;
// it owns no SQL and no HTTP.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
)

// Identity ------------------------------------------------------------------

type User struct {
	UserID        uuid.UUID
	Email         string
	FullName      string
	PasswordHash  string
	IsActive      bool
	LockedUntil   *time.Time
	FailedLogins  int
	TOTPSecret    string
	TOTPConfirmed bool
}

// Grant is one role held in one company (BR-5.4, D37).
type Grant struct {
	RoleID    uuid.UUID
	RoleCode  string
	CompanyID uuid.UUID
}

// Principal is the authenticated caller, resolved fresh on every request so
// revoking a role takes effect immediately rather than in fifteen minutes.
type Principal struct {
	UserID       uuid.UUID
	Email        string
	FullName     string
	Grants       []Grant
	Permissions  map[string]bool
	CompanyIDs   []uuid.UUID
	SiteIDs      []uuid.UUID // empty means no site restriction (BR-5.5)
	IsSuperadmin bool
}

func (p Principal) Can(permission string) bool { return p.Permissions[permission] }

func (p Principal) InCompany(c uuid.UUID) bool {
	for _, x := range p.CompanyIDs {
		if x == c {
			return true
		}
	}
	return false
}

// RolesInCompany is what the approval engine needs: the roles held IN THE
// SUBJECT'S company, never the union across companies (BR-4.4a).
func (p Principal) RolesInCompany(c uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, g := range p.Grants {
		if g.CompanyID == c {
			out = append(out, g.RoleID)
		}
	}
	return out
}

type UserRepo interface {
	ByEmail(ctx context.Context, email string) (*User, error)
	ByID(ctx context.Context, id uuid.UUID) (*User, error)
	Principal(ctx context.Context, id uuid.UUID) (*Principal, error)
	RecordLoginFailure(ctx context.Context, id uuid.UUID, lockFor time.Duration, threshold int) error
	ClearLoginFailures(ctx context.Context, id uuid.UUID) error
	SetTOTP(ctx context.Context, id uuid.UUID, secret string) error
	ConfirmTOTP(ctx context.Context, id uuid.UUID) error
	Create(ctx context.Context, u User, grants []Grant, actor *uuid.UUID) (uuid.UUID, error)
	List(ctx context.Context, q string, limit, offset int) ([]UserRow, int, error)
	SetGrants(ctx context.Context, userID uuid.UUID, grants []Grant, actor uuid.UUID) error
	SetActive(ctx context.Context, userID uuid.UUID, active bool) error
}

type UserRow struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	IsActive  bool      `json:"is_active"`
	TOTPReady bool      `json:"totp_ready"`
	Roles     []string  `json:"roles"`
	Companies []string  `json:"companies"`
}

type SessionRepo interface {
	IssueRefresh(ctx context.Context, userID, familyID uuid.UUID, hash string, expires time.Time, ua, ip string) error
	// RotateRefresh returns the user for a valid token and revokes the family
	// when a rotated token is presented again (BR-5.6).
	RotateRefresh(ctx context.Context, hash string, now time.Time) (userID uuid.UUID, familyID uuid.UUID, reuse bool, err error)
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error
	DenyJTI(ctx context.Context, jti string, expires time.Time) error
	IsJTIDenied(ctx context.Context, jti string) (bool, error)
	PurgeExpired(ctx context.Context, now time.Time) (int, error)
}

// Master data ---------------------------------------------------------------

type Company struct {
	CompanyID   uuid.UUID `json:"company_id"`
	CompanyCode string    `json:"company_code"`
	CompanyName string    `json:"company_name"`
	IsActive    bool      `json:"is_active"`
}

type Site struct {
	SiteID    uuid.UUID `json:"site_id"`
	CompanyID uuid.UUID `json:"company_id"`
	SiteCode  string    `json:"site_code"`
	SiteName  string    `json:"site_name"`
	SiteType  string    `json:"site_type"`
	IsActive  bool      `json:"is_active"`
}

type SiteGroup struct {
	SiteGroupID uuid.UUID   `json:"site_group_id"`
	CompanyID   uuid.UUID   `json:"company_id"`
	Name        string      `json:"site_group_name"`
	IsSystem    bool        `json:"is_system"`
	IsActive    bool        `json:"is_active"`
	MemberCount int         `json:"member_count"`
	SiteIDs     []uuid.UUID `json:"site_ids"`
}

type Holiday struct {
	HolidayID uuid.UUID `json:"holiday_id"`
	Date      time.Time `json:"holiday_date"`
	Name      string    `json:"holiday_name"`
	Country   string    `json:"country"`
	IsActive  bool      `json:"is_active"`
}

type MasterRepo interface {
	Companies(ctx context.Context) ([]Company, error)
	Sites(ctx context.Context, companies []uuid.UUID, q string) ([]Site, error)
	SiteByID(ctx context.Context, id uuid.UUID) (*Site, error)
	CreateSite(ctx context.Context, s Site, actor uuid.UUID) (uuid.UUID, error)
	UpdateSite(ctx context.Context, s Site, actor uuid.UUID) error
	SiteGroups(ctx context.Context, companies []uuid.UUID, q string) ([]SiteGroup, error)
	SiteGroupByID(ctx context.Context, id uuid.UUID) (*SiteGroup, error)
	CreateSiteGroup(ctx context.Context, g SiteGroup, siteIDs []uuid.UUID, actor uuid.UUID) (uuid.UUID, error)
	SetSiteGroupMembers(ctx context.Context, groupID uuid.UUID, siteIDs []uuid.UUID, actor uuid.UUID) error
	Holidays(ctx context.Context, year int) ([]Holiday, error)
	HolidaySet(ctx context.Context) (calendar.HolidaySet, error)
	UpsertHoliday(ctx context.Context, h Holiday, actor uuid.UUID) error
	DeleteHoliday(ctx context.Context, id uuid.UUID, actor uuid.UUID) error
}

// Parameters ----------------------------------------------------------------

type Param struct {
	Key         string     `json:"param_key"`
	Value       string     `json:"param_value"`
	ValueType   string     `json:"value_type"`
	Description string     `json:"description"`
	IsSecret    bool       `json:"is_secret"`
	UpdatedBy   *uuid.UUID `json:"updated_by"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ParamRepo interface {
	All(ctx context.Context) ([]Param, error)
	Int(ctx context.Context, key string, def int) int
	Bool(ctx context.Context, key string, def bool) bool
	String(ctx context.Context, key, def string) string
	List(ctx context.Context, key string) []string
	Set(ctx context.Context, key, value string, actor uuid.UUID) error
}

// Targets -------------------------------------------------------------------

type TargetRow struct {
	TargetID   uuid.UUID
	CompanyID  uuid.UUID
	SiteID     uuid.UUID
	SiteCode   string
	SiteName   string
	PeriodKind string
	Year       int
	Month      int
	SalesType  string
	AmountIDR  money.IDR
	UpdatedAt  time.Time
}

type TargetRepo interface {
	List(ctx context.Context, f TargetFilter) ([]TargetRow, error)
	Upsert(ctx context.Context, r TargetRow, actor uuid.UUID) error
	SumBySite(ctx context.Context, f TargetFilter) (map[uuid.UUID]money.IDR, error)
}

type TargetFilter struct {
	Companies  []uuid.UUID
	SiteIDs    []uuid.UUID
	Year       int
	Month      int
	PeriodKind string
	SalesType  string
	Query      string
}

// Promotions ----------------------------------------------------------------

type PlanRow struct {
	promo.Plan
	Version         promo.Version
	CompanyName     string
	CompanyCode     string
	SiteGroupName   string
	CreatedByName   string
	CurrentStepNo   int
	CurrentStepName string
	ApprovalStatus  string
}

type PlanFilter struct {
	Companies []uuid.UUID
	Status    string
	From, To  time.Time
	OrderMode string
	Query     string
	Limit     int
	Offset    int
}

type PromoRepo interface {
	List(ctx context.Context, f PlanFilter) ([]PlanRow, int, error)
	ByID(ctx context.Context, id uuid.UUID) (*PlanRow, error)
	Versions(ctx context.Context, planID uuid.UUID) ([]promo.Version, error)
	Create(ctx context.Context, p promo.Plan, v promo.Version) (uuid.UUID, error)
	SaveDraftVersion(ctx context.Context, v promo.Version) error
	NewVersion(ctx context.Context, planID uuid.UUID, v promo.Version) error
	SetStatus(ctx context.Context, planID uuid.UUID, status promo.Status, instanceID *uuid.UUID, forceReleased bool) error
	// Overlaps finds promotions sharing ANY SITE with the group, not merely the
	// same group id (BR-3.6 — the case a naive implementation misses).
	Overlaps(ctx context.Context, companyID, siteGroupID, excludePlanID uuid.UUID, from, to time.Time) ([]promo.Overlap, error)
	PendingBefore(ctx context.Context, cancelOnOrBefore time.Time) ([]PlanRow, error)
	NextPlanCode(ctx context.Context, year int) (string, error)
}

// Approval ------------------------------------------------------------------

type ApprovalRepo interface {
	ActiveChain(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType) (approval.Chain, uuid.UUID, error)
	ChainVersion(ctx context.Context, versionID uuid.UUID) (approval.Chain, error)
	OpenInstance(ctx context.Context, in approval.Instance) (uuid.UUID, error)
	InstanceByID(ctx context.Context, id uuid.UUID) (*approval.Instance, error)
	InstanceBySubject(ctx context.Context, t approval.SubjectType, id uuid.UUID) (*approval.Instance, error)
	// Decide applies a decision inside ONE transaction, taking SELECT ... FOR
	// UPDATE on the instance first (BR-4.10). Two approvers acting at once
	// must not both succeed.
	Decide(ctx context.Context, instanceID uuid.UUID, ip string, fn func(*approval.Instance, approval.Chain) (approval.Decision, error)) (approval.Decision, error)
	Events(ctx context.Context, instanceID uuid.UUID) ([]ApprovalEventRow, error)
	Inbox(ctx context.Context, p Principal, limit, offset int) ([]PlanRow, int, error)
	ChainConfig(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType) ([]ChainStepRow, int, error)
	SaveChain(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType, steps []ChainStepRow, actor uuid.UUID) (int, error)
}

type ApprovalEventRow struct {
	StepNo     int             `json:"step_no"`
	StepName   string          `json:"step_name"`
	Action     approval.Action `json:"action"`
	ActorName  string          `json:"actor_name"`
	Reason     string          `json:"reason"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type ChainStepRow struct {
	StepNo       int                   `json:"step_no"`
	StepName     string                `json:"step_name"`
	Satisfaction approval.Satisfaction `json:"satisfaction"`
	RoleIDs      []uuid.UUID           `json:"role_ids"`
	RoleLabels   []string              `json:"role_labels"`
}

// Facts and reporting -------------------------------------------------------

type TxnRow struct {
	CompanyID    uuid.UUID
	SiteID       uuid.UUID
	BusinessDate time.Time
	ReceiptNo    string
	SalesType    string
	PromoID      *uuid.UUID
	OrderMode    string
	GrossIDR     money.IDR
}

type ImportRun struct {
	ImportRunID  uuid.UUID  `json:"import_run_id"`
	FileName     string     `json:"file_name"`
	Checksum     string     `json:"file_checksum"`
	RowsRead     int        `json:"rows_read"`
	RowsInserted int        `json:"rows_inserted"`
	RowsSkipped  int        `json:"rows_skipped"`
	RowsRejected int        `json:"rows_rejected"`
	TrailerRows  *int       `json:"trailer_rows"`
	TrailerTotal *money.IDR `json:"trailer_total_idr"`
	Outcome      string     `json:"outcome"`
	Message      string     `json:"message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

type Rejection struct {
	LineNo   int    `json:"line_no"`
	Reason   string `json:"reason"`
	Original string `json:"original_line"`
}

type FactRepo interface {
	// SeenChecksum answers BR-6.3 before a file is parsed at all.
	SeenChecksum(ctx context.Context, sum string) (bool, error)
	// LoadFile replaces the site-days it covers, inside one transaction.
	LoadFile(ctx context.Context, run ImportRun, rows []TxnRow, rejects []Rejection, actor *uuid.UUID) (ImportRun, error)
	Runs(ctx context.Context, limit, offset int) ([]ImportRun, int, error)
	Rejections(ctx context.Context, runID uuid.UUID) ([]Rejection, error)
	PromoActuals(ctx context.Context, planIDs []uuid.UUID) (map[uuid.UUID]Actual, error)
	TargetVsActual(ctx context.Context, f TargetFilter) ([]TargetActualRow, error)
	SiteCodeIndex(ctx context.Context) (map[string]Site, error)
	PlanCodeIndex(ctx context.Context) (map[string]uuid.UUID, error)
}

type Actual struct {
	GrossIDR     money.IDR
	ReceiptCount int
}

type TargetActualRow struct {
	CompanyID    uuid.UUID
	CompanyName  string
	SiteID       uuid.UUID
	SiteCode     string
	SiteName     string
	Year         int
	Month        int
	SalesType    string
	TargetIDR    money.IDR
	ActualIDR    money.IDR
	ReceiptCount int
}

// Audit ---------------------------------------------------------------------

type AuditEntry struct {
	ActorID     *uuid.UUID
	Action      string
	SubjectType string
	SubjectID   *uuid.UUID
	CompanyID   *uuid.UUID
	Before      any
	After       any
	Reason      string
	IP          string
}

type AuditRepo interface {
	Write(ctx context.Context, e AuditEntry) error
	List(ctx context.Context, q string, limit, offset int) ([]AuditRow, int, error)
}

type AuditRow struct {
	AuditID     uuid.UUID  `json:"audit_id"`
	ActorName   string     `json:"actor_name"`
	Action      string     `json:"action"`
	SubjectType string     `json:"subject_type"`
	SubjectID   *uuid.UUID `json:"subject_id"`
	Reason      string     `json:"reason"`
	IP          string     `json:"ip_address"`
	OccurredAt  time.Time  `json:"occurred_at"`
}

// Notification --------------------------------------------------------------

// Sender is the port WhatsApp will also implement (D16). A failed send must
// never block an approval: the decision commits, the message queues.
type Sender interface {
	Queue(ctx context.Context, recipients []string, subject, body string, subjectType string, subjectID *uuid.UUID) error
	Flush(ctx context.Context, limit int) (sent int, failed int, err error)
}
