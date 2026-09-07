package app

import (
	"log/slog"
	"time"

	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/ratelimit"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

// Deps is everything a handler needs, wired once at boot.
//
// The wiring itself lives in cmd/api. If this package constructed its own
// repositories it would have to import the postgres adapter, and the
// dependency arrow would point outward — exactly what the hexagonal rule
// forbids (CLAUDE.md §2: adapter -> app -> domain, inward only). The compiler
// enforced it: the first version of this file imported the adapter and would
// not build.
type Deps struct {
	Cfg    config.Config
	Log    *slog.Logger
	DB     *gorm.DB
	Tokens *security.TokenIssuer
	Now    func() time.Time

	Users    UserRepo
	Sessions SessionRepo
	Master   MasterRepo
	Params   ParamRepo
	Targets  TargetRepo
	Promos   PromoRepo
	Approval ApprovalRepo
	Facts    FactRepo
	Audit    AuditRepo
	Notify   Sender

	LoginLimiter *ratelimit.Limiter
	// ReadLimiter is deliberately generous: it exists to stop a runaway client,
	// not to police staff doing their jobs.
	ReadLimiter *ratelimit.Limiter
}

// Parameter keys. Every operational timing is one of these (BR-1.6, and
// CLAUDE.md §7: "operational timings are parameters too").
const (
	ParamLeadTimeDays      = "promo.lead_time_working_days"
	ParamAutoCancelDays    = "promo.auto_cancel_days_before"
	ParamOverlapWarning    = "promo.overlap_warning_enabled"
	ParamReleaseRecipients = "notify.release_recipients"
	ParamImportDropPath    = "import.drop_path"
	ParamMaxExportRows     = "report.max_export_rows"
	ParamLockoutThreshold  = "auth.lockout_threshold"
	ParamLockoutMinutes    = "auth.lockout_minutes"
	ParamPasswordMinLen    = "auth.password_min_length"
	ParamTOTPRequired      = "auth.totp_required"
)
