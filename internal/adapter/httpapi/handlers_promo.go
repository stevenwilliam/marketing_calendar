package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/csvexport"
)

func planFilter(c *gin.Context, p app.Principal) app.PlanFilter {
	f := app.PlanFilter{
		Companies: p.CompanyIDs,
		Status:    c.Query("status"),
		OrderMode: c.Query("order_mode"),
		Query:     c.Query("q"),
	}
	if v := c.Query("company_id"); v != "" {
		if u, err := uuid.Parse(v); err == nil && p.InCompany(u) {
			f.Companies = []uuid.UUID{u}
		}
	}
	if v := c.Query("from"); v != "" {
		if t, err := calendar.ParseDate(v); err == nil {
			f.From = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := calendar.ParseDate(v); err == nil {
			f.To = t
		}
	}
	f.Limit, _ = strconv.Atoi(c.Query("limit"))
	f.Offset, _ = strconv.Atoi(c.Query("offset"))
	return f
}

func planJSON(r app.PlanRow) gin.H {
	return gin.H{
		"plan_id": r.PlanID, "plan_code": r.PlanCode, "status": r.Status,
		"company_id": r.CompanyID, "company_name": r.CompanyName, "company_code": r.CompanyCode,
		"site_group_id": r.SiteGroupID, "site_group_name": r.SiteGroupName,
		"force_released": r.ForceReleased, "created_by_name": r.CreatedByName,
		"current_step_no": r.CurrentStepNo, "current_step_name": r.CurrentStepName,
		"version": gin.H{
			"version_id": r.Version.VersionID, "version_no": r.Version.VersionNo,
			"promo_name":           r.Version.PromoName,
			"start_date":           r.Version.StartDate.Format("2006-01-02"),
			"end_date":             r.Version.EndDate.Format("2006-01-02"),
			"target_sales_idr":     int64(r.Version.TargetSalesIDR),
			"target_receipt_count": r.Version.TargetReceiptCount,
			"order_mode":           r.Version.OrderMode,
			"promo_rule":           r.Version.PromoRule,
			"lead_time_overridden": r.Version.LeadTimeOverridden,
			"media":                mediaJSON(r.Version.Media),
			"media_total_idr":      mediaTotal(r.Version.Media),
		},
	}
}

func mediaJSON(lines []promo.Media) []gin.H {
	out := make([]gin.H, 0, len(lines))
	for _, m := range lines {
		out = append(out, gin.H{"media_id": m.MediaID, "line_no": m.LineNo,
			"media_name": m.Name, "price_idr": int64(m.PriceIDR)})
	}
	return out
}

// mediaTotal is computed on read, never stored (D53). An error can only come
// from overflow, which validation refuses at write time; zero is the honest
// answer if it somehow arrives.
func mediaTotal(lines []promo.Media) int64 {
	total, err := promo.MediaTotal(lines)
	if err != nil {
		return 0
	}
	return int64(total)
}

// handleLeadTime tells the date picker where the lead time actually starts.
//
// The rule is evaluated server-side at submit either way (BR-3.3); this exists
// so the picker can DISABLE the impossible dates and say why, rather than
// letting someone choose one and be refused afterwards. Computing it in the
// browser would mean two implementations of the working-day arithmetic, and
// the browser's would be the one without the holiday table.
func handleLeadTime(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := d.Params.Int(c.Request.Context(), app.ParamLeadTimeDays, 7)
		holidays, err := d.Master.HolidaySet(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		earliest := holidays.EarliestStart(d.Now(), days)
		c.JSON(http.StatusOK, gin.H{
			"earliest_start":          earliest.Format("2006-01-02"),
			"lead_time_working_days":  days,
			"today":                   calendar.Today(d.Now()).Format("2006-01-02"),
			"auto_cancel_days_before": d.Params.Int(c.Request.Context(), app.ParamAutoCancelDays, 5),
		})
	}
}

func handlePlans(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		rows, total, err := d.Promos.List(c.Request.Context(), planFilter(c, p))
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

// handleCalendar returns a month of plans across every brand the caller can
// see — the month grid of the design.
func handleCalendar(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		year, _ := strconv.Atoi(c.Query("year"))
		month, _ := strconv.Atoi(c.Query("month"))
		now := calendar.Today(d.Now())
		if year == 0 {
			year = now.Year()
		}
		if month < 1 || month > 12 {
			month = int(now.Month())
		}
		from, to := calendar.MonthBounds(year, time.Month(month))
		f := planFilter(c, p)
		f.From, f.To, f.Limit = from, to, 1000
		rows, _, err := d.Promos.List(c.Request.Context(), f)
		if err != nil {
			fail(c, err)
			return
		}
		// Per-day achievement against the daily target (D55). Only released
		// and pending plans can have actuals attributed to them, but asking
		// for all of them costs one query either way.
		ids := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.PlanID)
		}
		// The green line is a parameter, not a constant (D59): Steven has
		// retuned it twice and will again.
		greenBPS := int64(d.Params.Int(c.Request.Context(), app.ParamAchievementGreen,
			int(promo.DefaultAchievementGreenBPS)))

		daily, err := d.Facts.PromoDailyActuals(c.Request.Context(), ids, from, to)
		if err != nil {
			fail(c, err)
			return
		}

		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			j := planJSON(r)
			target, days, ok := r.Version.DailyTarget()
			j["days"] = days
			j["daily_target_idr"] = int64(target)
			j["has_daily_target"] = ok

			// An entry for every day the promotion HAS ALREADY RUN inside this
			// window, not only the days that have rows.
			//
			// A day with no transactions is a day that sold NOTHING, which is
			// 0% — not an unknown. The em dash is reserved for the one case
			// where a percentage genuinely does not exist: no daily target.
			// Emitting only the days that have rows made a shop that sold
			// nothing look the same as a promotion nobody set a number on,
			// and those are opposite facts (D57).
			//
			// The two gates matter as much as the rule (D58). A day that has
			// not happened yet has sold nothing for the obvious reason, and a
			// plan that was never released could not have taken a rupiah — a
			// red 0% on either is an accusation about something that never
			// had the chance to succeed. A day with real actuals is always
			// reported regardless, by the loop below.
			perDay := make(map[string]gin.H)
			if ok {
				lo, hi := r.Version.StartDate, r.Version.EndDate
				if lo.Before(from) {
					lo = from
				}
				if hi.After(to) {
					hi = to
				}
				// Compared as Jakarta date KEYS, not as instants: a `date`
				// column arrives as UTC midnight while `now` is Jakarta
				// midnight, and the two are seven hours apart on the same
				// calendar day. An instant comparison would drop today.
				todayKey := calendar.Key(now)
				for day := lo; !day.After(hi); day = day.AddDate(0, 0, 1) {
					key := calendar.Key(day)
					a, has := daily[r.PlanID][key] // absent: nothing was sold
					// BR-7.5b, from the domain: a plan that was never
					// released and a day nobody has lived through yet have
					// no verdict, and must not read as a red 0%.
					if promo.Verdict(r.Status, key, todayKey, has, ok) != promo.VerdictBanded {
						continue
					}
					band, bps, _ := promo.BandFor(a.GrossIDR, target, greenBPS)
					perDay[key] = gin.H{
						"actual_idr":    int64(a.GrossIDR),
						"receipt_count": a.ReceiptCount,
						"band":          string(band),
						"achieved_bps":  bps,
					}
				}
			}
			for day, a := range daily[r.PlanID] {
				if _, already := perDay[day]; already {
					continue
				}
				band, bps, defined := promo.BandFor(a.GrossIDR, target, greenBPS)
				perDay[day] = gin.H{
					"actual_idr":    int64(a.GrossIDR),
					"receipt_count": a.ReceiptCount,
					"band":          string(band),
					// Null rather than 0 when undefined: a percentage of a
					// zero target is not zero percent, and 0% would read as a
					// total miss for a promotion nobody set a number on.
					"achieved_bps": func() any {
						if defined {
							return bps
						}
						return nil
					}(),
				}
			}
			j["daily"] = perDay
			out = append(out, j)
		}
		c.JSON(http.StatusOK, gin.H{
			"data": out, "year": year, "month": month,
			"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"),
			// The UI must not hold its own copy of the threshold: a legend
			// reading "≥80%" beside chips banded at 70% is worse than either.
			"achievement_green_bps": greenBPS})
	}
}

func handlePlan(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		row, err := d.Promos.ByID(c.Request.Context(), pid)
		if err != nil {
			fail(c, err)
			return
		}
		// IDOR: another company's plan is a 404, never a 403. The existence of
		// the row is not disclosed.
		if !p.InCompany(row.CompanyID) {
			fail(c, apierror.NotFound("rencana promo"))
			return
		}
		versions, err := d.Promos.Versions(c.Request.Context(), pid)
		if err != nil {
			fail(c, err)
			return
		}
		out := planJSON(*row)
		vs := make([]gin.H, 0, len(versions))
		for _, v := range versions {
			vs = append(vs, gin.H{"version_id": v.VersionID, "version_no": v.VersionNo,
				"promo_name": v.PromoName, "start_date": v.StartDate.Format("2006-01-02"),
				"end_date":             v.EndDate.Format("2006-01-02"),
				"target_sales_idr":     int64(v.TargetSalesIDR),
				"target_receipt_count": v.TargetReceiptCount,
				"order_mode":           v.OrderMode, "promo_rule": v.PromoRule,
				"created_at": v.CreatedAt})
		}
		out["versions"] = vs

		if inst, err := d.Approval.InstanceBySubject(c.Request.Context(),
			"promotion_plan", pid); err == nil && inst != nil {
			events, _ := d.Approval.Events(c.Request.Context(), inst.InstanceID)
			chain, _ := d.Approval.ChainVersion(c.Request.Context(), inst.VersionID)
			steps := make([]gin.H, 0, len(chain.Steps))
			for _, s := range chain.Steps {
				steps = append(steps, gin.H{"step_no": s.StepNo, "step_name": s.Name,
					"satisfaction": s.Satisfaction})
			}
			out["approval"] = gin.H{"instance_id": inst.InstanceID, "status": inst.Status,
				"current_step_no": inst.CurrentStepNo, "steps": steps, "events": events}
		}
		c.JSON(http.StatusOK, out)
	}
}

type planRequest struct {
	CompanyID          uuid.UUID `json:"company_id"`
	SiteGroupID        uuid.UUID `json:"site_group_id"`
	PromoName          string    `json:"promo_name"`
	StartDate          string    `json:"start_date"`
	EndDate            string    `json:"end_date"`
	TargetSalesIDR     int64     `json:"target_sales_idr"`
	TargetReceiptCount int       `json:"target_receipt_count"`
	OrderMode          string    `json:"order_mode"`
	PromoRule          string    `json:"promo_rule"`
	Media              []struct {
		Name     string `json:"media_name"`
		PriceIDR int64  `json:"price_idr"`
	} `json:"media"`
	AcknowledgeOverlap bool   `json:"acknowledge_overlap"`
	OverrideLeadTime   bool   `json:"override_lead_time"`
	OverrideReason     string `json:"override_reason"`
}

func (r planRequest) toInput() (app.PromoInput, error) {
	start, err := calendar.ParseDate(r.StartDate)
	if err != nil {
		return app.PromoInput{}, apierror.Validation("tanggal mulai tidak valid",
			map[string]string{"start_date": "format harus YYYY-MM-DD"})
	}
	end, err := calendar.ParseDate(r.EndDate)
	if err != nil {
		return app.PromoInput{}, apierror.Validation("tanggal selesai tidak valid",
			map[string]string{"end_date": "format harus YYYY-MM-DD"})
	}
	media := make([]app.MediaInput, 0, len(r.Media))
	for _, m := range r.Media {
		media = append(media, app.MediaInput{Name: m.Name, PriceIDR: money.IDR(m.PriceIDR)})
	}
	return app.PromoInput{
		CompanyID: r.CompanyID, SiteGroupID: r.SiteGroupID, PromoName: r.PromoName,
		StartDate: start, EndDate: end, TargetSalesIDR: money.IDR(r.TargetSalesIDR),
		TargetReceiptCount: r.TargetReceiptCount, OrderMode: r.OrderMode,
		PromoRule: r.PromoRule, Media: media, AcknowledgeOverlap: r.AcknowledgeOverlap,
		OverrideLeadTime: r.OverrideLeadTime, OverrideReason: r.OverrideReason,
	}, nil
}

func handleCreatePlan(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		var req planRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		in, err := req.toInput()
		if err != nil {
			fail(c, err)
			return
		}
		pid, err := d.CreatePlan(c.Request.Context(), p, in, clientIP(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"plan_id": pid})
	}
}

func handleUpdatePlan(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req planRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		in, err := req.toInput()
		if err != nil {
			fail(c, err)
			return
		}
		if err := d.UpdatePlan(c.Request.Context(), p, pid, in, clientIP(c)); err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleSubmit(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		pid, ok := parseUUID(c, "id")
		if !ok {
			return
		}
		var req struct {
			AcknowledgeOverlap bool `json:"acknowledge_overlap"`
		}
		_ = c.ShouldBindJSON(&req)

		out, err := d.Submit(c.Request.Context(), p, pid, req.AcknowledgeOverlap, clientIP(c))
		if err != nil {
			e := apierror.As(err)
			// The overlap refusal carries the list so the UI can name what it
			// conflicts with rather than saying "there is a conflict".
			if e.Code == apierror.CodeOverlap && out != nil {
				overlaps := make([]gin.H, 0, len(out.Overlaps))
				for _, o := range out.Overlaps {
					overlaps = append(overlaps, gin.H{"plan_id": o.PlanID, "plan_code": o.PlanCode,
						"promo_name": o.PromoName, "start_date": o.StartDate.Format("2006-01-02"),
						"end_date":          o.EndDate.Format("2006-01-02"),
						"shared_site_count": len(o.SharedSiteIDs)})
				}
				trace, _ := c.Get(ctxTrace)
				traceID, _ := trace.(string)
				c.AbortWithStatusJSON(e.HTTPStatus(), gin.H{
					"code": e.Code, "message": e.Message, "overlaps": overlaps, "trace_id": traceID})
				return
			}
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "submitted"})
	}
}

func handleExportPlans(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		f := planFilter(c, p)
		writeCSV(c, d, "rencana-promo", func() (int, error) {
			return d.ExportPlans(c.Request.Context(), p, f, c.Writer)
		})
	}
}

// writeCSV sets the headers, streams, and audits the export with its row count
// and filters — the control that makes report.export a permission worth
// separating from report.view.
func writeCSV(c *gin.Context, d *app.Deps, name string, run func() (int, error)) {
	p := principal(c)
	filename := csvexport.Filename(name, d.Now(), calendar.Jakarta)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Status(http.StatusOK)

	n, err := run()
	if err != nil {
		// The status is already written; there is no way to turn this into a
		// clean error response, so the failure is logged and the client gets a
		// short file. Saying so is better than pretending it succeeded.
		d.Log.Error("csv export failed midway", "report", name, "err", err.Error())
		return
	}
	_ = d.Audit.Write(c.Request.Context(), app.AuditEntry{ActorID: &p.UserID,
		Action: "report.export", SubjectType: name,
		After: map[string]any{"rows": n, "filters": c.Request.URL.RawQuery},
		IP:    clientIP(c)})
}
