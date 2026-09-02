package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/calendar"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type MasterRepo struct{ db *gorm.DB }

func NewMasterRepo(db *gorm.DB) *MasterRepo { return &MasterRepo{db: db} }

func (r *MasterRepo) Companies(ctx context.Context) ([]app.Company, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT company_id, company_code, company_name, is_active FROM company ORDER BY company_name`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Company
	for rows.Next() {
		var c app.Company
		if err := rows.Scan(&c.CompanyID, &c.CompanyCode, &c.CompanyName, &c.IsActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Sites is scoped by company IN THE QUERY, never filtered afterwards (BR-1.5),
// and by the caller's site scope where one is set (BR-5.5).
//
// An EMPTY siteScope means "no site restriction", not "no sites". The two read
// the same in a naive implementation and are opposites: getting it backwards
// either shows everything to a restricted user or nothing to an unrestricted
// one. The cardinality() test says which is meant, explicitly.
func (r *MasterRepo) Sites(ctx context.Context, companies, siteScope []uuid.UUID, q string) ([]app.Site, error) {
	like := "%" + q + "%"
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT site_id, company_id, site_code, site_name, site_type, is_active
		  FROM site
		 WHERE company_id = ANY(?::uuid[])
		   AND (cardinality(?::uuid[]) = 0 OR site_id = ANY(?::uuid[]))
		   AND (? = '' OR site_code ILIKE ? OR site_name ILIKE ?)
		 ORDER BY site_code`,
		uuidList(companies), uuidList(siteScope), uuidList(siteScope), q, like, like).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Site
	for rows.Next() {
		var s app.Site
		if err := rows.Scan(&s.SiteID, &s.CompanyID, &s.SiteCode, &s.SiteName, &s.SiteType, &s.IsActive); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *MasterRepo) SiteByID(ctx context.Context, sid uuid.UUID) (*app.Site, error) {
	var s app.Site
	row := r.db.WithContext(ctx).Raw(`
		SELECT site_id, company_id, site_code, site_name, site_type, is_active
		  FROM site WHERE site_id = ?`, sid).Row()
	if err := row.Scan(&s.SiteID, &s.CompanyID, &s.SiteCode, &s.SiteName, &s.SiteType, &s.IsActive); err != nil {
		if isNoRows(err) {
			return nil, apierror.NotFound("toko")
		}
		return nil, err
	}
	return &s, nil
}

// CreateSite creates the site AND its system group in the same transaction
// (BR-1.3). A site without its group is a site no single-site promotion can
// target, and the two must not be able to drift apart.
func (r *MasterRepo) CreateSite(ctx context.Context, s app.Site, actor uuid.UUID) (uuid.UUID, error) {
	sid := id.New()
	gid := id.New()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO site (site_id, company_id, site_code, site_name, site_type, is_active)
			VALUES (?, ?, ?, ?, ?, ?)`,
			sid, s.CompanyID, s.SiteCode, s.SiteName, s.SiteType, s.IsActive).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO site_group (site_group_id, company_id, site_group_name, is_system, auto_for_site_id)
			VALUES (?, ?, ?, true, ?)`, gid, s.CompanyID, s.SiteName, sid).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO site_group_member (site_group_id, site_id, company_id)
			VALUES (?, ?, ?)`, gid, sid, s.CompanyID).Error
	})
	return sid, err
}

func (r *MasterRepo) UpdateSite(ctx context.Context, s app.Site, actor uuid.UUID) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE site SET site_code = ?, site_name = ?, site_type = ?, is_active = ?
		 WHERE site_id = ? AND company_id = ?`,
		s.SiteCode, s.SiteName, s.SiteType, s.IsActive, s.SiteID, s.CompanyID)
	if res.Error != nil {
		return res.Error
	}
	// A scoped UPDATE that matches nothing is an authorisation failure, not a
	// success: the row exists in another company.
	if res.RowsAffected == 0 {
		return apierror.NotFound("toko")
	}
	return nil
}

func (r *MasterRepo) SiteGroups(ctx context.Context, companies []uuid.UUID, q string) ([]app.SiteGroup, error) {
	like := "%" + q + "%"
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT g.site_group_id, g.company_id, g.site_group_name, g.is_system, g.is_active,
		       count(m.site_id)
		  FROM site_group g
		  LEFT JOIN site_group_member m ON m.site_group_id = g.site_group_id
		 WHERE g.company_id = ANY(?::uuid[])
		   AND (? = '' OR g.site_group_name ILIKE ?)
		 GROUP BY g.site_group_id
		 ORDER BY g.is_system, g.site_group_name`, uuidList(companies), q, like).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.SiteGroup
	for rows.Next() {
		var g app.SiteGroup
		if err := rows.Scan(&g.SiteGroupID, &g.CompanyID, &g.Name, &g.IsSystem, &g.IsActive, &g.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *MasterRepo) SiteGroupByID(ctx context.Context, gid uuid.UUID) (*app.SiteGroup, error) {
	var g app.SiteGroup
	row := r.db.WithContext(ctx).Raw(`
		SELECT site_group_id, company_id, site_group_name, is_system, is_active
		  FROM site_group WHERE site_group_id = ?`, gid).Row()
	if err := row.Scan(&g.SiteGroupID, &g.CompanyID, &g.Name, &g.IsSystem, &g.IsActive); err != nil {
		if isNoRows(err) {
			return nil, apierror.NotFound("kelompok toko")
		}
		return nil, err
	}
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT site_id FROM site_group_member WHERE site_group_id = ?`, gid).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s uuid.UUID
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		g.SiteIDs = append(g.SiteIDs, s)
	}
	g.MemberCount = len(g.SiteIDs)
	return &g, nil
}

func (r *MasterRepo) CreateSiteGroup(ctx context.Context, g app.SiteGroup, siteIDs []uuid.UUID, actor uuid.UUID) (uuid.UUID, error) {
	gid := id.New()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO site_group (site_group_id, company_id, site_group_name, is_system, is_active)
			VALUES (?, ?, ?, false, true)`, gid, g.CompanyID, g.Name).Error; err != nil {
			return err
		}
		return insertMembers(tx, gid, g.CompanyID, siteIDs)
	})
	return gid, err
}

// SetSiteGroupMembers replaces membership. A SYSTEM group refuses: its
// membership is exactly one site by construction (BR-1.3).
func (r *MasterRepo) SetSiteGroupMembers(ctx context.Context, gid uuid.UUID, siteIDs []uuid.UUID, actor uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var isSystem bool
		var companyID uuid.UUID
		row := tx.Raw(`SELECT is_system, company_id FROM site_group WHERE site_group_id = ? FOR UPDATE`, gid).Row()
		if err := row.Scan(&isSystem, &companyID); err != nil {
			if isNoRows(err) {
				return apierror.NotFound("kelompok toko")
			}
			return err
		}
		if isSystem {
			return apierror.New(apierror.CodeConflict,
				"kelompok otomatis per toko tidak dapat diubah keanggotaannya")
		}
		if err := tx.Exec(`DELETE FROM site_group_member WHERE site_group_id = ?`, gid).Error; err != nil {
			return err
		}
		return insertMembers(tx, gid, companyID, siteIDs)
	})
}

// insertMembers writes membership rows with the group's company_id. The
// composite foreign key (D31) then refuses any site from another brand — the
// database does the checking, not this function.
func insertMembers(tx *gorm.DB, gid, companyID uuid.UUID, siteIDs []uuid.UUID) error {
	for _, s := range siteIDs {
		err := tx.Exec(`
			INSERT INTO site_group_member (site_group_id, site_id, company_id)
			VALUES (?, ?, ?)`, gid, s, companyID).Error
		if err != nil {
			if isForeignKeyViolation(err) {
				return apierror.New(apierror.CodeCrossBrand,
					"toko dari merek lain tidak dapat masuk ke kelompok ini")
			}
			return err
		}
	}
	return nil
}

func (r *MasterRepo) Holidays(ctx context.Context, year int) ([]app.Holiday, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT holiday_id, holiday_date, holiday_name, country, is_active
		  FROM holiday
		 WHERE (? = 0 OR EXTRACT(YEAR FROM holiday_date) = ?)
		 ORDER BY holiday_date`, year, year).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.Holiday
	for rows.Next() {
		var h app.Holiday
		if err := rows.Scan(&h.HolidayID, &h.Date, &h.Name, &h.Country, &h.IsActive); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// HolidaySet feeds the working-day arithmetic. Only ACTIVE holidays count: a
// holiday an administrator has deactivated must stop shifting the lead time.
func (r *MasterRepo) HolidaySet(ctx context.Context) (calendar.HolidaySet, error) {
	rows, err := r.db.WithContext(ctx).Raw(
		`SELECT holiday_date, holiday_name FROM holiday WHERE is_active`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := calendar.HolidaySet{}
	for rows.Next() {
		var d time.Time
		var name string
		if err := rows.Scan(&d, &name); err != nil {
			return nil, err
		}
		out[d.Format("2006-01-02")] = name
	}
	return out, nil
}

func (r *MasterRepo) UpsertHoliday(ctx context.Context, h app.Holiday, actor uuid.UUID) error {
	hid := h.HolidayID
	if hid == uuid.Nil {
		hid = id.New()
	}
	country := h.Country
	if country == "" {
		country = "ID"
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO holiday (holiday_id, holiday_date, holiday_name, country, is_active)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (country, holiday_date)
		DO UPDATE SET holiday_name = EXCLUDED.holiday_name, is_active = EXCLUDED.is_active`,
		hid, h.Date, h.Name, country, h.IsActive).Error
}

func (r *MasterRepo) DeleteHoliday(ctx context.Context, hid uuid.UUID, actor uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(`DELETE FROM holiday WHERE holiday_id = ?`, hid).Error
}
