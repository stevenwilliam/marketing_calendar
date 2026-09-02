package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

func handleCompanies(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		all, err := d.Master.Companies(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		// Scoped to the caller's companies. A user assigned one brand must not
		// learn the names of the other two from a picker.
		out := make([]app.Company, 0, len(all))
		for _, x := range all {
			if p.InCompany(x.CompanyID) {
				out = append(out, x)
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
	}
}

func handleSites(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, err := d.Master.Sites(c.Request.Context(), p.CompanyIDs, p.SiteIDs, c.Query("q"))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

type siteRequest struct {
	CompanyID uuid.UUID `json:"company_id"`
	SiteCode  string    `json:"site_code"`
	SiteName  string    `json:"site_name"`
	SiteType  string    `json:"site_type"`
	IsActive  *bool     `json:"is_active"`
}

func (r *siteRequest) clean() (app.Site, error) {
	fields := map[string]string{}
	code, err := sanitize.Text(r.SiteCode, 40)
	if err != nil {
		fields["site_code"] = err.Error()
	}
	name, err := sanitize.Text(r.SiteName, 200)
	if err != nil {
		fields["site_name"] = err.Error()
	}
	stype, err := sanitize.Enum(r.SiteType, "coffee_shop", "restaurant", "catering")
	if err != nil {
		fields["site_type"] = "harus coffee_shop, restaurant atau catering"
	}
	if len(fields) > 0 {
		return app.Site{}, apierror.Validation("periksa kembali isian", fields)
	}
	active := true
	if r.IsActive != nil {
		active = *r.IsActive
	}
	return app.Site{CompanyID: r.CompanyID, SiteCode: code, SiteName: name,
		SiteType: stype, IsActive: active}, nil
}

func handleCreateSite(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req siteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		if !p.InCompany(req.CompanyID) {
			fail(c, apierror.NotFound("perusahaan"))
			return
		}
		s, err := req.clean()
		if err != nil {
			fail(c, err)
			return
		}
		sid, err := d.Master.CreateSite(c.Request.Context(), s, p.UserID)
		if err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "site.create", SubjectType: "site", SubjectID: &sid,
			CompanyID: &s.CompanyID, After: s, IP: clientIP(c)})
		c.JSON(http.StatusCreated, gin.H{"site_id": sid})
	}
}

func handleUpdateSite(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		sid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		existing, err := d.Master.SiteByID(c.Request.Context(), sid)
		if err != nil {
			fail(c, err)
			return
		}
		if !p.InCompany(existing.CompanyID) {
			fail(c, apierror.NotFound("toko"))
			return
		}
		var req siteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		req.CompanyID = existing.CompanyID // a site never changes brand
		s, err := req.clean()
		if err != nil {
			fail(c, err)
			return
		}
		s.SiteID = sid
		if err := d.Master.UpdateSite(c.Request.Context(), s, p.UserID); err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "site.update", SubjectType: "site", SubjectID: &sid,
			CompanyID: &s.CompanyID, Before: existing, After: s, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleSiteGroups(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, err := d.Master.SiteGroups(c.Request.Context(), p.CompanyIDs, c.Query("q"))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

type groupRequest struct {
	CompanyID uuid.UUID   `json:"company_id"`
	Name      string      `json:"site_group_name"`
	SiteIDs   []uuid.UUID `json:"site_ids"`
}

func handleCreateSiteGroup(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req groupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		if !p.InCompany(req.CompanyID) {
			fail(c, apierror.NotFound("perusahaan"))
			return
		}
		name, err := sanitize.Text(req.Name, 200)
		if err != nil {
			fail(c, apierror.Validation("nama kelompok wajib diisi",
				map[string]string{"site_group_name": err.Error()}))
			return
		}
		gid, err := d.Master.CreateSiteGroup(c.Request.Context(),
			app.SiteGroup{CompanyID: req.CompanyID, Name: name}, req.SiteIDs, p.UserID)
		if err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "site_group.create", SubjectType: "site_group", SubjectID: &gid,
			CompanyID: &req.CompanyID, After: req, IP: clientIP(c)})
		c.JSON(http.StatusCreated, gin.H{"site_group_id": gid})
	}
}

func handleSetGroupMembers(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		gid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		g, err := d.Master.SiteGroupByID(c.Request.Context(), gid)
		if err != nil {
			fail(c, err)
			return
		}
		if !p.InCompany(g.CompanyID) {
			fail(c, apierror.NotFound("kelompok toko"))
			return
		}
		var req groupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		if err := d.Master.SetSiteGroupMembers(c.Request.Context(), gid, req.SiteIDs, p.UserID); err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "site_group.set_members", SubjectType: "site_group", SubjectID: &gid,
			CompanyID: &g.CompanyID, Before: g.SiteIDs, After: req.SiteIDs, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleHolidays(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		year, _ := strconv.Atoi(c.Query("year"))
		rows, err := d.Master.Holidays(c.Request.Context(), year)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

type holidayRequest struct {
	Date     string `json:"holiday_date"`
	Name     string `json:"holiday_name"`
	IsActive *bool  `json:"is_active"`
}

func handleUpsertHoliday(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req holidayRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		date, err := calendar.ParseDate(req.Date)
		if err != nil {
			fail(c, apierror.Validation("tanggal tidak valid",
				map[string]string{"holiday_date": "format harus YYYY-MM-DD"}))
			return
		}
		name, err := sanitize.Text(req.Name, 200)
		if err != nil {
			fail(c, apierror.Validation("nama hari libur wajib diisi",
				map[string]string{"holiday_name": err.Error()}))
			return
		}
		active := true
		if req.IsActive != nil {
			active = *req.IsActive
		}
		h := app.Holiday{Date: date, Name: name, Country: "ID", IsActive: active}
		if err := d.Master.UpsertHoliday(c.Request.Context(), h, p.UserID); err != nil {
			fail(c, err)
			return
		}
		// BR-8.2: a holiday change moves every lead-time calculation, so it is
		// audited like a parameter change.
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "holiday.upsert", SubjectType: "holiday", After: h, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleDeleteHoliday(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		hid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		if err := d.Master.DeleteHoliday(c.Request.Context(), hid, p.UserID); err != nil {
			fail(c, err)
			return
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "holiday.delete", SubjectType: "holiday", SubjectID: &hid, IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleParameters(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := d.Params.All(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

func handleSetParameter(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		key := c.Param("key")
		var req struct {
			Value string `json:"param_value"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		value, err := sanitize.Text(req.Value, 2000)
		if err != nil {
			fail(c, apierror.Validation("nilai tidak valid", map[string]string{"param_value": err.Error()}))
			return
		}
		before, _ := d.Params.All(c.Request.Context())
		if err := d.Params.Set(c.Request.Context(), key, value, p.UserID); err != nil {
			fail(c, err)
			return
		}
		var prev string
		for _, x := range before {
			if x.Key == key {
				prev = x.Value
			}
		}
		_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
			Action: "parameter.set", SubjectType: "sys_parameters",
			Before: map[string]string{key: prev}, After: map[string]string{key: value},
			IP: clientIP(c)})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
