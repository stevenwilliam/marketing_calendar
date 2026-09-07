package app

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/money"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/promo"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/target"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/csvexport"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
)

// PromoReportRow is BR-7.5: the plan, the actuals attributed by promo_id, the
// variance in both absolute and percentage terms, and whether the plan was
// force-released.
type PromoReportRow struct {
	PlanID         uuid.UUID
	PlanCode       string
	PromoName      string
	CompanyName    string
	SiteGroupName  string
	StartDate      time.Time
	EndDate        time.Time
	OrderMode      string
	Status         string
	ForceReleased  bool
	TargetSalesIDR money.IDR
	ActualSalesIDR money.IDR
	SalesDeltaIDR  money.IDR
	SalesBPS       int64
	SalesDefined   bool
	TargetReceipts int
	ActualReceipts int
	ReceiptDelta   int
}

func (d *Deps) PromoReport(ctx context.Context, p Principal, f PlanFilter) ([]PromoReportRow, error) {
	if !p.Can("report.view") {
		return nil, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.view")
	}
	f.Companies = intersectCompanies(p, f.Companies)
	f.Limit = 5000
	plans, _, err := d.Promos.List(ctx, f)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(plans))
	for _, pl := range plans {
		ids = append(ids, pl.PlanID)
	}
	actuals, err := d.Facts.PromoActuals(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]PromoReportRow, 0, len(plans))
	for _, pl := range plans {
		a := actuals[pl.PlanID]
		ach, err := target.Achieved(pl.Version.TargetSalesIDR, a.GrossIDR)
		if err != nil {
			return nil, err
		}
		out = append(out, PromoReportRow{
			PlanID: pl.PlanID, PlanCode: pl.PlanCode, PromoName: pl.Version.PromoName,
			CompanyName: pl.CompanyName, SiteGroupName: pl.SiteGroupName,
			StartDate: pl.Version.StartDate, EndDate: pl.Version.EndDate,
			OrderMode: string(pl.Version.OrderMode), Status: string(pl.Status),
			ForceReleased:  pl.ForceReleased,
			TargetSalesIDR: pl.Version.TargetSalesIDR, ActualSalesIDR: a.GrossIDR,
			SalesDeltaIDR: ach.DeltaIDR, SalesBPS: ach.AchievedBPS, SalesDefined: ach.Defined,
			TargetReceipts: pl.Version.TargetReceiptCount, ActualReceipts: a.ReceiptCount,
			ReceiptDelta: a.ReceiptCount - pl.Version.TargetReceiptCount,
		})
	}
	return out, nil
}

func (d *Deps) TargetReport(ctx context.Context, p Principal, f TargetFilter) ([]TargetActualRow, error) {
	if !p.Can("report.view") {
		return nil, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.view")
	}
	f.Companies = intersectCompanies(p, f.Companies)
	return d.Facts.TargetVsActual(ctx, f)
}

// IntersectSites is BR-5.5. An EMPTY site scope on the principal means "no
// site restriction", so a request without a site filter stays unfiltered; a
// scoped caller can narrow within their sites but never past them.
//
// The empty cases are opposites and are easy to swap: no scope means all,
// while a scope that excludes everything requested must return nothing. The
// sentinel below is what keeps "asked for a site I may not see" from
// collapsing into "asked for nothing, so show me everything".
func IntersectSites(p Principal, requested []uuid.UUID) []uuid.UUID {
	if len(p.SiteIDs) == 0 {
		return requested // unrestricted caller
	}
	if len(requested) == 0 {
		return p.SiteIDs // restricted caller, no narrowing asked for
	}
	allowed := map[uuid.UUID]bool{}
	for _, s := range p.SiteIDs {
		allowed[s] = true
	}
	out := make([]uuid.UUID, 0, len(requested))
	for _, s := range requested {
		if allowed[s] {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		// Asked only for sites they may not see. Returning an empty slice
		// would read as "no filter" downstream and show them everything, so
		// an impossible id is returned instead and the result is empty.
		return []uuid.UUID{uuid.Nil}
	}
	return out
}

// intersectCompanies is the scoping rule. A caller may narrow to a subset of
// their companies; they can never widen beyond them, and an empty request
// means "all of mine" rather than "all that exist".
func intersectCompanies(p Principal, requested []uuid.UUID) []uuid.UUID {
	if len(requested) == 0 {
		return p.CompanyIDs
	}
	allowed := map[uuid.UUID]bool{}
	for _, c := range p.CompanyIDs {
		allowed[c] = true
	}
	var out []uuid.UUID
	for _, c := range requested {
		if allowed[c] {
			out = append(out, c)
		}
	}
	return out
}

// CSV exports ---------------------------------------------------------------
//
// BR-7.4: an export reflects the SCREEN. These take the same filter the list
// took, so the file is what the user is looking at.

func (d *Deps) ExportPromoReport(ctx context.Context, p Principal, f PlanFilter, w io.Writer) (int, error) {
	if !p.Can("report.export") {
		return 0, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.export")
	}
	rows, err := d.PromoReport(ctx, p, f)
	if err != nil {
		return 0, err
	}
	if err := d.checkExportCap(ctx, len(rows)); err != nil {
		return 0, err
	}
	c := csvexport.New(w)
	c.Write([]string{"kode_rencana", "nama_promo", "merek", "kelompok_toko",
		"mulai", "selesai", "mode_pesanan", "status", "rilis_paksa",
		"target_penjualan_idr", "aktual_penjualan_idr", "selisih_penjualan_idr", "capaian_persen",
		"target_struk", "aktual_struk", "selisih_struk"})
	for _, r := range rows {
		achieved := ""
		if r.SalesDefined {
			// Basis points to a percentage with two decimals, in integer
			// arithmetic: 12345 bps -> "123.45".
			achieved = formatBPS(r.SalesBPS)
		}
		c.Write([]string{
			r.PlanCode, r.PromoName, r.CompanyName, r.SiteGroupName,
			csvexport.Date(r.StartDate, calendar.Jakarta), csvexport.Date(r.EndDate, calendar.Jakarta),
			r.OrderMode, r.Status, boolText(r.ForceReleased),
			csvexport.Money(r.TargetSalesIDR), csvexport.Money(r.ActualSalesIDR),
			csvexport.Money(r.SalesDeltaIDR), achieved,
			csvexport.Int(r.TargetReceipts), csvexport.Int(r.ActualReceipts), csvexport.Int(r.ReceiptDelta),
		})
	}
	return len(rows), c.Flush()
}

func (d *Deps) ExportTargetReport(ctx context.Context, p Principal, f TargetFilter, w io.Writer) (int, error) {
	if !p.Can("report.export") {
		return 0, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.export")
	}
	rows, err := d.TargetReport(ctx, p, f)
	if err != nil {
		return 0, err
	}
	if err := d.checkExportCap(ctx, len(rows)); err != nil {
		return 0, err
	}
	c := csvexport.New(w)
	c.Write([]string{"merek", "kode_toko", "nama_toko", "tahun", "bulan", "jenis_penjualan",
		"target_idr", "aktual_idr", "selisih_idr", "capaian_persen", "jumlah_struk"})
	for _, r := range rows {
		ach, err := target.Achieved(r.TargetIDR, r.ActualIDR)
		if err != nil {
			return 0, err
		}
		achieved := ""
		if ach.Defined {
			achieved = formatBPS(ach.AchievedBPS)
		}
		month := ""
		if r.Month > 0 {
			month = csvexport.Int(r.Month)
		}
		c.Write([]string{r.CompanyName, r.SiteCode, r.SiteName, csvexport.Int(r.Year), month,
			r.SalesType, csvexport.Money(r.TargetIDR), csvexport.Money(r.ActualIDR),
			csvexport.Money(ach.DeltaIDR), achieved, csvexport.Int(r.ReceiptCount)})
	}
	return len(rows), c.Flush()
}

func (d *Deps) ExportPlans(ctx context.Context, p Principal, f PlanFilter, w io.Writer) (int, error) {
	if !p.Can("report.export") {
		return 0, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.export")
	}
	f.Companies = intersectCompanies(p, f.Companies)
	f.Limit = 5000
	rows, _, err := d.Promos.List(ctx, f)
	if err != nil {
		return 0, err
	}
	if err := d.checkExportCap(ctx, len(rows)); err != nil {
		return 0, err
	}
	c := csvexport.New(w)
	c.Write([]string{"kode_rencana", "nama_promo", "merek", "kelompok_toko", "status",
		"mulai", "selesai", "mode_pesanan", "target_penjualan_idr", "target_struk",
		"versi", "langkah_saat_ini", "dibuat_oleh", "rilis_paksa",
		"total_biaya_media_idr", "jumlah_media", "aturan_promo"})
	for _, r := range rows {
		c.Write([]string{r.PlanCode, r.Version.PromoName, r.CompanyName, r.SiteGroupName,
			string(r.Status), csvexport.Date(r.Version.StartDate, calendar.Jakarta),
			csvexport.Date(r.Version.EndDate, calendar.Jakarta), string(r.Version.OrderMode),
			csvexport.Money(r.Version.TargetSalesIDR), csvexport.Int(r.Version.TargetReceiptCount),
			csvexport.Int(r.Version.VersionNo), r.CurrentStepName, r.CreatedByName,
			boolText(r.ForceReleased),
			csvexport.Money(mediaTotalOrZero(r.Version.Media)),
			csvexport.Int(len(r.Version.Media)),
			// Flattened to text: markup in a spreadsheet cell is noise, and
			// the formula-injection guard still applies to what comes out.
			sanitize.HTMLToText(r.Version.PromoRule)})
	}
	return len(rows), c.Flush()
}

func (d *Deps) ExportTargets(ctx context.Context, p Principal, f TargetFilter, w io.Writer) (int, error) {
	if !p.Can("report.export") {
		return 0, apierror.New(apierror.CodeForbidden, "tidak memiliki izin report.export")
	}
	f.Companies = intersectCompanies(p, f.Companies)
	rows, err := d.Targets.List(ctx, f)
	if err != nil {
		return 0, err
	}
	if err := d.checkExportCap(ctx, len(rows)); err != nil {
		return 0, err
	}
	c := csvexport.New(w)
	c.Write([]string{"kode_toko", "nama_toko", "periode", "tahun", "bulan", "jenis_penjualan", "target_idr"})
	for _, r := range rows {
		month := ""
		if r.Month > 0 {
			month = csvexport.Int(r.Month)
		}
		c.Write([]string{r.SiteCode, r.SiteName, r.PeriodKind, csvexport.Int(r.Year), month,
			r.SalesType, csvexport.Money(r.AmountIDR)})
	}
	return len(rows), c.Flush()
}

// checkExportCap enforces report.max_export_rows.
//
// It refuses BEFORE a byte is written, because the CSV streams straight to the
// response: once the 200 and the headers are out there is no way to turn a
// failure into a clean error, and the user would get a silently short file.
func (d *Deps) checkExportCap(ctx context.Context, rows int) error {
	max := d.Params.Int(ctx, ParamMaxExportRows, 100000)
	if max > 0 && rows > max {
		return apierror.Newf(apierror.CodeValidation,
			"ekspor berisi %d baris, melebihi batas %d — persempit filter atau naikkan report.max_export_rows",
			rows, max)
	}
	return nil
}

// mediaTotalOrZero is the export's view of a media total. Overflow is refused
// at write time, so a failure here cannot come from stored data; zero is the
// honest answer rather than a panic in the middle of a download.
func mediaTotalOrZero(lines []promo.Media) money.IDR {
	total, err := promo.MediaTotal(lines)
	if err != nil {
		return 0
	}
	return total
}

// formatBPS renders basis points as a percentage with two decimals, in
// integer arithmetic. 12345 -> "123.45". No float appears.
func formatBPS(bps int64) string {
	neg := bps < 0
	if neg {
		bps = -bps
	}
	whole := bps / 100
	frac := bps % 100
	s := itoa(int(whole)) + "." + pad2(int(frac))
	if neg {
		return "-" + s
	}
	return s
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func boolText(b bool) string {
	if b {
		return "ya"
	}
	return "tidak"
}
