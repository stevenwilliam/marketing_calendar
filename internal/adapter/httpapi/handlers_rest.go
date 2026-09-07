package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/target"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
)

// Approval ------------------------------------------------------------------

func handleInbox(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		limit, _ := strconv.Atoi(c.Query("limit"))
		offset, _ := strconv.Atoi(c.Query("offset"))
		rows, total, err := d.Approval.Inbox(c.Request.Context(), p, limit, offset)
		if err != nil {
			fail(c, err)
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, planJSON(r))
		}
		c.JSON(http.StatusOK, gin.H{"data": out, "total": total})
	}
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func handleApprove(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		if err := d.Approve(c.Request.Context(), p, pid, clientIP(c)); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "approved"})
	}
}

func handleReject(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req reasonRequest
		_ = c.ShouldBindJSON(&req)
		if err := d.Reject(c.Request.Context(), p, pid, req.Reason, clientIP(c)); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "rejected"})
	}
}

func handleForceRelease(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req reasonRequest
		_ = c.ShouldBindJSON(&req)
		if err := d.ForceRelease(c.Request.Context(), p, pid, req.Reason, clientIP(c)); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "force_released"})
	}
}

func handleRevive(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req reasonRequest
		_ = c.ShouldBindJSON(&req)
		if err := d.Revive(c.Request.Context(), p, pid, req.Reason, clientIP(c)); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "revived"})
	}
}

func handleGetChain(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		companyID, err := uuid.Parse(c.Query("company_id"))
		if err != nil || !p.InCompany(companyID) {
			fail(c, apierror.NotFound("perusahaan"))
			return
		}
		steps, version, err := d.Approval.ChainConfig(c.Request.Context(), companyID,
			approval.SubjectPromotionPlan)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": steps, "version_no": version})
	}
}

type chainRequest struct {
	CompanyID uuid.UUID `json:"company_id"`
	Steps     []struct {
		StepName     string      `json:"step_name"`
		Satisfaction string      `json:"satisfaction"`
		RoleIDs      []uuid.UUID `json:"role_ids"`
	} `json:"steps"`
}

func handleSaveChain(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req chainRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		if !p.InCompany(req.CompanyID) {
			fail(c, apierror.NotFound("perusahaan"))
			return
		}
		steps := make([]app.ChainStepRow, 0, len(req.Steps))
		for i, s := range req.Steps {
			name, err := sanitize.Text(s.StepName, 120)
			if err != nil {
				fail(c, apierror.Validation("nama langkah wajib diisi", nil))
				return
			}
			sat, err := sanitize.Enum(s.Satisfaction, "ANY_OF", "ALL_OF")
			if err != nil {
				sat = "ANY_OF"
			}
			steps = append(steps, app.ChainStepRow{StepNo: i + 1, StepName: name,
				Satisfaction: approval.Satisfaction(sat), RoleIDs: s.RoleIDs})
		}
		version, err := d.Approval.SaveChain(c.Request.Context(), req.CompanyID,
			approval.SubjectPromotionPlan, steps, p.UserID)
		if err != nil {
			fail(c, err)
			return
		}
		// BR-4.3: this created a NEW VERSION. Plans in flight keep theirs, and
		// the response says so rather than leaving the administrator to wonder.
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "approval_chain.save", SubjectType: "approval_chain",
			CompanyID: &req.CompanyID, After: steps, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"version_no": version,
			"note": "rencana yang sedang berjalan tetap memakai versi lama (BR-4.3)"})
	}
}

// Targets -------------------------------------------------------------------

func targetFilter(c *gin.Context, p app.Principal) app.TargetFilter {
	f := app.TargetFilter{Companies: p.CompanyIDs, Query: c.Query("q"),
		PeriodKind: c.Query("period_kind"), SalesType: c.Query("sales_type")}
	if v := c.Query("company_id"); v != "" {
		if u, err := uuid.Parse(v); err == nil && p.InCompany(u) {
			f.Companies = []uuid.UUID{u}
		}
	}
	f.Year, _ = strconv.Atoi(c.Query("year"))
	f.Month, _ = strconv.Atoi(c.Query("month"))
	if v := c.Query("site_id"); v != "" {
		if u, err := uuid.Parse(v); err == nil {
			f.SiteIDs = []uuid.UUID{u}
		}
	}
	// BR-5.5: a caller may narrow to a subset of their sites; they can never
	// widen past their scope. Applied here so every target path inherits it.
	f.SiteIDs = app.IntersectSites(p, f.SiteIDs)
	return f
}

func handleTargets(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, err := d.Targets.List(c.Request.Context(), targetFilter(c, p))
		if err != nil {
			fail(c, err)
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, gin.H{"target_id": r.TargetID, "site_id": r.SiteID,
				"site_code": r.SiteCode, "site_name": r.SiteName,
				"period_kind": r.PeriodKind, "year": r.Year, "month": r.Month,
				"sales_type": r.SalesType, "target_amount_idr": int64(r.AmountIDR)})
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	}
}

type targetRequest struct {
	CompanyID  uuid.UUID `json:"company_id"`
	SiteID     uuid.UUID `json:"site_id"`
	PeriodKind string    `json:"period_kind"`
	Year       int       `json:"year"`
	Month      int       `json:"month"`
	SalesType  string    `json:"sales_type"`
	AmountIDR  int64     `json:"target_amount_idr"`
}

func handleUpsertTarget(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req targetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		if !p.InCompany(req.CompanyID) {
			fail(c, apierror.NotFound("perusahaan"))
			return
		}
		// The domain validates. BR-2.3's "months need not sum to the year" is
		// enforced by there being no code here that checks it.
		t := target.Target{PeriodKind: target.PeriodKind(req.PeriodKind), Year: req.Year,
			Month: req.Month, SalesType: target.SalesType(req.SalesType),
			AmountIDR: money.IDR(req.AmountIDR)}
		if err := t.Validate(); err != nil {
			fail(c, apierror.Validation(err.Error(), map[string]string{"target_amount_idr": err.Error()}))
			return
		}
		row := app.TargetRow{CompanyID: req.CompanyID, SiteID: req.SiteID,
			PeriodKind: req.PeriodKind, Year: req.Year, Month: req.Month,
			SalesType: req.SalesType, AmountIDR: money.IDR(req.AmountIDR)}
		if err := d.Targets.Upsert(c.Request.Context(), row, p.UserID); err != nil {
			fail(c, err)
			return
		}
		// BR-2.7: a target edit is always audited, and an edit after the
		// period closed is the one a reviewer will ask about.
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "target.upsert", SubjectType: "sales_target",
			CompanyID: &req.CompanyID, After: row, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// handleTargetSummary is the year-versus-months variance of BR-2.3. It is a
// DISPLAY value and never an error.
func handleTargetSummary(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		f := targetFilter(c, p)
		rows, err := d.Targets.List(c.Request.Context(), f)
		if err != nil {
			fail(c, err)
			return
		}
		type key struct {
			site      uuid.UUID
			salesType string
		}
		months := map[key][]money.IDR{}
		years := map[key]money.IDR{}
		names := map[key]app.TargetRow{}
		for _, r := range rows {
			k := key{r.SiteID, r.SalesType}
			names[k] = r
			if r.PeriodKind == "YEAR" {
				years[k] = r.AmountIDR
			} else {
				months[k] = append(months[k], r.AmountIDR)
			}
		}
		out := make([]gin.H, 0, len(names))
		for k, meta := range names {
			var yt *money.IDR
			if v, ok := years[k]; ok {
				yv := v
				yt = &yv
			}
			v, err := target.YearVersusMonths(yt, months[k])
			if err != nil {
				fail(c, err)
				return
			}
			out = append(out, gin.H{
				"site_id": k.site, "site_code": meta.SiteCode, "site_name": meta.SiteName,
				"sales_type": k.salesType, "has_year_target": v.HasYear,
				"year_target_idr": int64(v.YearTarget), "months_sum_idr": int64(v.MonthsSum),
				"delta_idr": int64(v.DeltaIDR), "delta_bps": v.DeltaBPS,
				"month_count": v.MonthCount})
		}
		c.JSON(http.StatusOK, gin.H{"data": out,
			"note": "selisih antara target tahun dan jumlah bulan adalah tampilan, bukan kesalahan (BR-2.3)"})
	}
}

func handleExportTargets(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		f := targetFilter(c, p)
		writeCSV(c, d, "target", func() (int, error) {
			return d.ExportTargets(c.Request.Context(), p, f, c.Writer)
		})
	}
}

// Reports -------------------------------------------------------------------

func handlePromoReport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, err := d.PromoReport(c.Request.Context(), p, planFilter(c, p))
		if err != nil {
			fail(c, err)
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			var achieved any
			if r.SalesDefined {
				achieved = r.SalesBPS
			}
			out = append(out, gin.H{
				"plan_id": r.PlanID, "plan_code": r.PlanCode, "promo_name": r.PromoName,
				"company_name": r.CompanyName, "site_group_name": r.SiteGroupName,
				"start_date": r.StartDate.Format("2006-01-02"),
				"end_date":   r.EndDate.Format("2006-01-02"),
				"order_mode": r.OrderMode, "status": r.Status, "force_released": r.ForceReleased,
				"target_sales_idr": int64(r.TargetSalesIDR),
				"actual_sales_idr": int64(r.ActualSalesIDR),
				"sales_delta_idr":  int64(r.SalesDeltaIDR),
				"achieved_bps":     achieved,
				"target_receipts":  r.TargetReceipts, "actual_receipts": r.ActualReceipts,
				"receipt_delta": r.ReceiptDelta})
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	}
}

func handleExportPromoReport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		f := planFilter(c, p)
		writeCSV(c, d, "laporan-promo", func() (int, error) {
			return d.ExportPromoReport(c.Request.Context(), p, f, c.Writer)
		})
	}
}

func handleTargetReport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, err := d.TargetReport(c.Request.Context(), p, targetFilter(c, p))
		if err != nil {
			fail(c, err)
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			ach, _ := target.Achieved(r.TargetIDR, r.ActualIDR)
			var achieved any
			if ach.Defined {
				achieved = ach.AchievedBPS
			}
			out = append(out, gin.H{"company_name": r.CompanyName, "site_id": r.SiteID,
				"site_code": r.SiteCode, "site_name": r.SiteName, "year": r.Year,
				"month": r.Month, "sales_type": r.SalesType,
				"target_idr": int64(r.TargetIDR), "actual_idr": int64(r.ActualIDR),
				"delta_idr": int64(ach.DeltaIDR), "achieved_bps": achieved,
				"receipt_count": r.ReceiptCount})
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	}
}

func handleExportTargetReport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		f := targetFilter(c, p)
		writeCSV(c, d, "target-vs-aktual", func() (int, error) {
			return d.ExportTargetReport(c.Request.Context(), p, f, c.Writer)
		})
	}
}

// Import --------------------------------------------------------------------

func handleImports(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		offset, _ := strconv.Atoi(c.Query("offset"))
		rows, total, err := d.Facts.Runs(c.Request.Context(), limit, offset)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	}
}

func handleRejections(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		rows, err := d.Facts.Rejections(c.Request.Context(), rid)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

// handleImportTemplate hands back a ready-to-fill example for a kind.
//
// A template is cheaper than a specification: the header is exactly what the
// parser matches on, the example row shows the shape of every value, and the
// trailer is already correct for the rows above it. Somebody filling this in
// cannot get the column names wrong.
func handleImportTemplate(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := app.ImportKind(c.Param("kind"))
		switch kind {
		case app.KindTransactions, app.KindTargetYear, app.KindTargetMonth:
		default:
			fail(c, apierror.NotFound("template"))
			return
		}
		name, body := app.Template(kind)
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		// The same UTF-8 BOM every export carries, so Excel on Windows opens
		// the template the same way it will open the file the user produces.
		c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		c.Writer.WriteString(body)
	}
}

func handleRunImport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		results, err := d.ImportDropPath(c.Request.Context(), &p.UserID)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": results})
	}
}

func handleUploadImport(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		fh, err := c.FormFile("file")
		if err != nil {
			fail(c, apierror.Validation("berkas tidak disertakan", nil))
			return
		}
		if fh.Size > 64<<20 {
			fail(c, apierror.Validation("berkas melebihi 64 MB", nil))
			return
		}
		// The filename from a client is a string, never a path.
		name := sanitize.Filename(fh.Filename)
		dir := d.Params.String(c.Request.Context(), app.ParamImportDropPath, d.Cfg.ImportDropPath)
		dest := filepath.Join(dir, name)
		if err := c.SaveUploadedFile(fh, dest); err != nil {
			fail(c, apierror.Wrap(apierror.CodeInternal, "tidak dapat menyimpan berkas", err))
			return
		}
		res, err := d.ImportFile(c.Request.Context(), dest, &p.UserID)
		if err != nil {
			_ = os.Remove(dest)
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": res})
	}
}

// Users and audit -----------------------------------------------------------

func handleRoles(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := d.DB.WithContext(c.Request.Context()).Raw(
			`SELECT role_id, role_code, label_id, label_en FROM role ORDER BY role_code`).Rows()
		if err != nil {
			fail(c, err)
			return
		}
		defer rows.Close()
		out := []gin.H{}
		for rows.Next() {
			var rid uuid.UUID
			var code, labelID, labelEN string
			if err := rows.Scan(&rid, &code, &labelID, &labelEN); err != nil {
				fail(c, err)
				return
			}
			out = append(out, gin.H{"role_id": rid, "role_code": code,
				"label_id": labelID, "label_en": labelEN})
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	}
}

func handleUsers(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		offset, _ := strconv.Atoi(c.Query("offset"))
		rows, total, err := d.Users.List(c.Request.Context(), c.Query("q"), limit, offset)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	}
}

type userRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
	Grants   []struct {
		RoleID    uuid.UUID `json:"role_id"`
		CompanyID uuid.UUID `json:"company_id"`
	} `json:"grants"`
}

func handleCreateUser(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req userRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		email, err := sanitize.Email(req.Email)
		if err != nil {
			fail(c, apierror.Validation("alamat surel tidak sah",
				map[string]string{"email": err.Error()}))
			return
		}
		name, err := sanitize.Text(req.FullName, 200)
		if err != nil {
			fail(c, apierror.Validation("nama lengkap wajib diisi",
				map[string]string{"full_name": err.Error()}))
			return
		}
		minLen := d.Params.Int(c.Request.Context(), app.ParamPasswordMinLen,
			security.DefaultPasswordMinLength)
		if err := security.CheckPasswordStrength(req.Password, minLen); err != nil {
			msg := fmt.Sprintf("kata sandi minimal %d karakter", minLen)
			fail(c, apierror.Validation(msg, map[string]string{"password": msg}))
			return
		}
		hash, err := security.HashPassword(req.Password)
		if err != nil {
			fail(c, err)
			return
		}
		// D37: at least one company, always explicit. An empty list is a
		// validation error naming the field, not a user who can log in and
		// see nothing.
		grants := make([]app.Grant, 0, len(req.Grants))
		for _, g := range req.Grants {
			if !p.InCompany(g.CompanyID) {
				fail(c, apierror.NotFound("perusahaan"))
				return
			}
			grants = append(grants, app.Grant{RoleID: g.RoleID, CompanyID: g.CompanyID})
		}
		if len(grants) == 0 {
			fail(c, apierror.Validation("pilih minimal satu peran dan perusahaan",
				map[string]string{"grants": "minimal satu peran di satu perusahaan (BR-5.4)"}))
			return
		}
		uid, err := d.Users.Create(c.Request.Context(),
			app.User{Email: email, FullName: name, PasswordHash: hash}, grants, &p.UserID)
		if err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "user.create", SubjectType: "app_user", SubjectID: &uid,
			After: map[string]any{"email": email, "grants": len(grants)}, IP: clientIP(c)})
		c.JSON(http.StatusCreated, gin.H{"user_id": uid})
	}
}

func handleSetUserRoles(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		uid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req userRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		grants := make([]app.Grant, 0, len(req.Grants))
		for _, g := range req.Grants {
			if !p.InCompany(g.CompanyID) {
				fail(c, apierror.NotFound("perusahaan"))
				return
			}
			grants = append(grants, app.Grant{RoleID: g.RoleID, CompanyID: g.CompanyID})
		}
		if err := d.Users.SetGrants(c.Request.Context(), uid, grants, p.UserID); err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "user.set_roles", SubjectType: "app_user", SubjectID: &uid,
			After: grants, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleAudit(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		offset, _ := strconv.Atoi(c.Query("offset"))
		rows, total, err := d.Audit.List(c.Request.Context(), c.Query("q"), limit, offset)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
	}
}
