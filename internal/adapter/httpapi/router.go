package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/web"
)

// NewRouter mounts every route. Each one declares its permission; a route
// without a require() is either public by design (health, login) or a defect,
// and there is nowhere else for a handler to be registered.
func NewRouter(d *app.Deps) http.Handler {
	if d.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), traceMiddleware(d), securityHeaders())
	if len(d.Cfg.TrustedProxies) > 0 {
		_ = r.SetTrustedProxies(d.Cfg.TrustedProxies)
	} else {
		_ = r.SetTrustedProxies(nil)
	}
	r.MaxMultipartMemory = 16 << 20

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := d.DB.DB()
		if err == nil {
			err = sqlDB.Ping()
		}
		if err != nil {
			c.JSON(503, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})
	if d.Cfg.MetricsEnabled {
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	api := r.Group("/api/v1")

	// Public: authentication only.
	auth := api.Group("/auth")
	auth.POST("/login", handleLogin(d))
	auth.POST("/totp", handleVerifyTOTP(d))
	auth.POST("/refresh", handleRefresh(d))

	// Everything below requires a session.
	in := api.Group("")
	in.Use(authenticate(d))

	in.POST("/auth/logout", handleLogout(d))
	in.GET("/auth/me", handleMe(d))

	in.GET("/companies", require("promo.view"), handleCompanies(d))
	in.GET("/sites", require("promo.view"), handleSites(d))
	in.POST("/sites", require("site.manage"), handleCreateSite(d))
	in.PUT("/sites/:id", require("site.manage"), handleUpdateSite(d))
	in.GET("/site-groups", require("promo.view"), handleSiteGroups(d))
	in.POST("/site-groups", require("site.manage"), handleCreateSiteGroup(d))
	in.PUT("/site-groups/:id/members", require("site.manage"), handleSetGroupMembers(d))

	in.GET("/holidays", require("promo.view"), handleHolidays(d))
	in.POST("/holidays", require("settings.manage"), handleUpsertHoliday(d))
	in.DELETE("/holidays/:id", require("settings.manage"), handleDeleteHoliday(d))

	in.GET("/parameters", require("settings.manage"), handleParameters(d))
	in.PUT("/parameters/:key", require("settings.manage"), handleSetParameter(d))

	in.GET("/roles", require("user.manage"), handleRoles(d))
	in.GET("/users", require("user.manage"), handleUsers(d))
	in.POST("/users", require("user.manage"), handleCreateUser(d))
	in.PUT("/users/:id/roles", require("user.manage"), handleSetUserRoles(d))

	in.GET("/targets", require("target.view"), handleTargets(d))
	in.PUT("/targets", require("target.manage"), handleUpsertTarget(d))
	in.GET("/targets/summary", require("target.view"), handleTargetSummary(d))
	in.GET("/targets/export", require("report.export"), handleExportTargets(d))

	in.GET("/promotions", require("promo.view"), handlePlans(d))
	in.GET("/promotions/calendar", require("promo.view"), handleCalendar(d))
	in.POST("/promotions", require("promo.create"), handleCreatePlan(d))
	in.GET("/promotions/:id", require("promo.view"), handlePlan(d))
	in.PUT("/promotions/:id", require("promo.view"), handleUpdatePlan(d))
	in.POST("/promotions/:id/submit", require("promo.view"), handleSubmit(d))
	in.GET("/promotions/export", require("report.export"), handleExportPlans(d))

	in.GET("/approvals/inbox", require("promo.view"), handleInbox(d))
	in.POST("/promotions/:id/approve", require("promo.view"), handleApprove(d))
	in.POST("/promotions/:id/reject", require("promo.view"), handleReject(d))
	in.POST("/promotions/:id/force-release", require("force_release"), handleForceRelease(d))
	in.POST("/promotions/:id/revive", require("force_release"), handleRevive(d))
	in.GET("/approvals/chain", require("settings.manage"), handleGetChain(d))
	in.PUT("/approvals/chain", require("settings.manage"), handleSaveChain(d))

	in.GET("/imports", require("import.view"), handleImports(d))
	in.GET("/imports/:id/rejections", require("import.view"), handleRejections(d))
	in.POST("/imports/run", require("import.run"), handleRunImport(d))
	in.POST("/imports/upload", require("import.run"), handleUploadImport(d))
	// Anyone who can see the reconciliation screen can take a template; it is
	// a blank form, not data.
	in.GET("/imports/templates/:kind", require("import.view"), handleImportTemplate(d))

	in.GET("/reports/promotions", require("report.view"), handlePromoReport(d))
	in.GET("/reports/promotions/export", require("report.export"), handleExportPromoReport(d))
	in.GET("/reports/targets", require("report.view"), handleTargetReport(d))
	in.GET("/reports/targets/export", require("report.export"), handleExportTargetReport(d))

	in.GET("/audit", require("audit.view"), handleAudit(d))

	// The built SPA, mounted last. Its NoRoute handler returns a JSON 404 for
	// anything under /api/ and index.html for everything else, so client-side
	// routing survives a refresh without swallowing an unknown endpoint.
	web.Mount(r)
	return r
}
