package main

import (
	"log/slog"
	"time"

	"github.com/stevenwilliam/marketing_calendar/internal/adapter/postgres"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/ratelimit"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

// newDeps is the composition root. It is the ONLY place that knows both the
// app layer and the postgres adapter exist, which is what keeps the dependency
// arrow pointing inward.
func newDeps(db *gorm.DB, cfg config.Config, log *slog.Logger, sender app.Sender) (*app.Deps, error) {
	return &app.Deps{
		Cfg:    cfg,
		Log:    log,
		DB:     db,
		Tokens: security.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL),
		Now:    func() time.Time { return time.Now().UTC() },

		Users:    postgres.NewUserRepo(db),
		Sessions: postgres.NewSessionRepo(db),
		Master:   postgres.NewMasterRepo(db),
		Params:   postgres.NewParamRepo(db),
		Targets:  postgres.NewTargetRepo(db),
		Promos:   postgres.NewPromoRepo(db),
		Approval: postgres.NewApprovalRepo(db),
		Facts:    postgres.NewFactRepo(db),
		Audit:    postgres.NewAuditRepo(db),
		Notify:   sender,

		LoginLimiter: ratelimit.New(5, 10),
		ReadLimiter:  ratelimit.New(120, 600),
	}, nil
}
