// Package postgres holds the repositories. Money paths use explicit raw SQL
// with placeholders and integer arithmetic, never ORM arithmetic (99 §6).
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

const userSelect = `
SELECT u.user_id, u.email, u.full_name, u.password_hash, u.is_active,
       u.locked_until, u.failed_logins,
       COALESCE(t.secret, ''), (t.confirmed_at IS NOT NULL)
  FROM app_user u
  LEFT JOIN user_totp t ON t.user_id = u.user_id`

func scanUser(row interface{ Scan(...any) error }) (*app.User, error) {
	var u app.User
	err := row.Scan(&u.UserID, &u.Email, &u.FullName, &u.PasswordHash, &u.IsActive,
		&u.LockedUntil, &u.FailedLogins, &u.TOTPSecret, &u.TOTPConfirmed)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*app.User, error) {
	row := r.db.WithContext(ctx).Raw(userSelect+` WHERE lower(u.email) = lower(?)`, email).Row()
	u, err := scanUser(row)
	if errors.Is(err, gorm.ErrRecordNotFound) || isNoRows(err) {
		return nil, nil // an unknown email is not an error; the caller spends the dummy verify
	}
	return u, err
}

func (r *UserRepo) ByID(ctx context.Context, uid uuid.UUID) (*app.User, error) {
	row := r.db.WithContext(ctx).Raw(userSelect+` WHERE u.user_id = ?`, uid).Row()
	u, err := scanUser(row)
	if isNoRows(err) {
		return nil, apierror.NotFound("pengguna")
	}
	return u, err
}

// Principal resolves permissions FRESH on every request. Baking them into the
// access token would mean a revoked role keeps working for fifteen minutes.
func (r *UserRepo) Principal(ctx context.Context, uid uuid.UUID) (*app.Principal, error) {
	u, err := r.ByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	p := &app.Principal{UserID: u.UserID, Email: u.Email, FullName: u.FullName,
		Permissions: map[string]bool{}}

	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT ur.role_id, r.role_code, ur.company_id
		  FROM user_role ur JOIN role r ON r.role_id = ur.role_id
		 WHERE ur.user_id = ?`, uid).Rows()
	if err != nil {
		return nil, err
	}
	seenCompany := map[uuid.UUID]bool{}
	for rows.Next() {
		var g app.Grant
		if err := rows.Scan(&g.RoleID, &g.RoleCode, &g.CompanyID); err != nil {
			rows.Close()
			return nil, err
		}
		p.Grants = append(p.Grants, g)
		if g.RoleCode == "superadmin" {
			p.IsSuperadmin = true
		}
		if !seenCompany[g.CompanyID] {
			seenCompany[g.CompanyID] = true
			p.CompanyIDs = append(p.CompanyIDs, g.CompanyID)
		}
	}
	rows.Close()

	perms, err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT rp.permission_code
		  FROM user_role ur JOIN role_permission rp ON rp.role_id = ur.role_id
		 WHERE ur.user_id = ?`, uid).Rows()
	if err != nil {
		return nil, err
	}
	defer perms.Close()
	for perms.Next() {
		var code string
		if err := perms.Scan(&code); err != nil {
			return nil, err
		}
		p.Permissions[code] = true
	}

	scopes, err := r.db.WithContext(ctx).Raw(
		`SELECT site_id FROM user_site_scope WHERE user_id = ?`, uid).Rows()
	if err != nil {
		return nil, err
	}
	defer scopes.Close()
	for scopes.Next() {
		var s uuid.UUID
		if err := scopes.Scan(&s); err != nil {
			return nil, err
		}
		p.SiteIDs = append(p.SiteIDs, s)
	}
	return p, nil
}

func (r *UserRepo) RecordLoginFailure(ctx context.Context, uid uuid.UUID, lockFor time.Duration, threshold int) error {
	// The lock is computed in SQL so two concurrent failures cannot both read
	// the same count and each write count+1.
	return r.db.WithContext(ctx).Exec(`
		UPDATE app_user
		   SET failed_logins = failed_logins + 1,
		       locked_until = CASE WHEN failed_logins + 1 >= ?
		                           THEN now() + ?::interval ELSE locked_until END,
		       updated_at = now()
		 WHERE user_id = ?`,
		threshold, lockFor.String(), uid).Error
}

func (r *UserRepo) ClearLoginFailures(ctx context.Context, uid uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE app_user SET failed_logins = 0, locked_until = NULL, updated_at = now() WHERE user_id = ?`,
		uid).Error
}

func (r *UserRepo) SetTOTP(ctx context.Context, uid uuid.UUID, secret string) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO user_totp (user_id, secret) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET secret = EXCLUDED.secret, confirmed_at = NULL`,
		uid, secret).Error
}

func (r *UserRepo) ConfirmTOTP(ctx context.Context, uid uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE user_totp SET confirmed_at = now() WHERE user_id = ?`, uid).Error
}

// Create inserts the user and its grants in ONE transaction. A user created
// without grants would be able to log in and see nothing, which reads as a
// broken account rather than as the deliberate state it is not.
func (r *UserRepo) Create(ctx context.Context, u app.User, grants []app.Grant, actor *uuid.UUID) (uuid.UUID, error) {
	if len(grants) == 0 {
		return uuid.Nil, apierror.Validation("pengguna harus memiliki minimal satu peran dan perusahaan",
			map[string]string{"grants": "minimal satu peran di satu perusahaan (BR-5.4)"})
	}
	uid := id.New()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO app_user (user_id, email, full_name, password_hash, is_active)
			VALUES (?, ?, ?, ?, true)`, uid, u.Email, u.FullName, u.PasswordHash).Error; err != nil {
			return err
		}
		for _, g := range grants {
			if err := tx.Exec(`
				INSERT INTO user_role (user_id, role_id, company_id, granted_by)
				VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`,
				uid, g.RoleID, g.CompanyID, actor).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return uid, err
}

func (r *UserRepo) SetGrants(ctx context.Context, uid uuid.UUID, grants []app.Grant, actor uuid.UUID) error {
	if len(grants) == 0 {
		return apierror.Validation("pengguna harus memiliki minimal satu peran dan perusahaan",
			map[string]string{"grants": "minimal satu peran di satu perusahaan (BR-5.4)"})
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM user_role WHERE user_id = ?`, uid).Error; err != nil {
			return err
		}
		for _, g := range grants {
			if err := tx.Exec(`
				INSERT INTO user_role (user_id, role_id, company_id, granted_by)
				VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`,
				uid, g.RoleID, g.CompanyID, actor).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *UserRepo) SetActive(ctx context.Context, uid uuid.UUID, active bool) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE app_user SET is_active = ?, updated_at = now() WHERE user_id = ?`, active, uid).Error
}

func (r *UserRepo) List(ctx context.Context, q string, limit, offset int) ([]app.UserRow, int, error) {
	like := "%" + q + "%"
	var total int64
	if err := r.db.WithContext(ctx).Raw(`
		SELECT count(*) FROM app_user u
		 WHERE (? = '' OR u.email ILIKE ? OR u.full_name ILIKE ?)`,
		q, like, like).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT u.user_id, u.email, u.full_name, u.is_active,
		       (t.confirmed_at IS NOT NULL) AS totp_ready,
		       COALESCE(array_agg(DISTINCT r.label_id) FILTER (WHERE r.role_id IS NOT NULL), '{}'),
		       COALESCE(array_agg(DISTINCT c.company_name) FILTER (WHERE c.company_id IS NOT NULL), '{}')
		  FROM app_user u
		  LEFT JOIN user_totp t ON t.user_id = u.user_id
		  LEFT JOIN user_role ur ON ur.user_id = u.user_id
		  LEFT JOIN role r ON r.role_id = ur.role_id
		  LEFT JOIN company c ON c.company_id = ur.company_id
		 WHERE (? = '' OR u.email ILIKE ? OR u.full_name ILIKE ?)
		 GROUP BY u.user_id, t.confirmed_at
		 ORDER BY u.full_name
		 LIMIT ? OFFSET ?`, q, like, like, limit, offset).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.UserRow
	for rows.Next() {
		var u app.UserRow
		if err := rows.Scan(&u.UserID, &u.Email, &u.FullName, &u.IsActive, &u.TOTPReady,
			pqArray(&u.Roles), pqArray(&u.Companies)); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, int(total), nil
}
