package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type SessionRepo struct{ db *gorm.DB }

func NewSessionRepo(db *gorm.DB) *SessionRepo { return &SessionRepo{db: db} }

func (r *SessionRepo) IssueRefresh(ctx context.Context, userID, familyID uuid.UUID, hash string, expires time.Time, ua, ip string) error {
	var ipVal any
	if ip != "" {
		ipVal = ip
	}
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO refresh_token (token_id, user_id, family_id, token_hash, expires_at, user_agent, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.New(), userID, familyID, hash, expires, ua, ipVal).Error
}

// RotateRefresh is the whole of BR-5.6 in one transaction.
//
// Presenting a token that has ALREADY been used is the signature of a stolen
// token: the legitimate client and the thief both hold a copy, and whichever
// arrives second proves the first was copied. The response is to revoke the
// entire family, not just that token — revoking one leaves the thief holding
// the newer one.
func (r *SessionRepo) RotateRefresh(ctx context.Context, hash string, now time.Time) (uuid.UUID, uuid.UUID, bool, error) {
	var userID, familyID uuid.UUID
	var reuse bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var usedAt, revokedAt *time.Time
		var expires time.Time
		row := tx.Raw(`
			SELECT user_id, family_id, used_at, revoked_at, expires_at
			  FROM refresh_token WHERE token_hash = ? FOR UPDATE`, hash).Row()
		if err := row.Scan(&userID, &familyID, &usedAt, &revokedAt, &expires); err != nil {
			if isNoRows(err) {
				return errNoToken
			}
			return err
		}
		if usedAt != nil {
			reuse = true
			return tx.Exec(`
				UPDATE refresh_token SET revoked_at = ?
				 WHERE family_id = ? AND revoked_at IS NULL`, now, familyID).Error
		}
		if revokedAt != nil || now.After(expires) {
			return errNoToken
		}
		return tx.Exec(`UPDATE refresh_token SET used_at = ? WHERE token_hash = ?`, now, hash).Error
	})
	return userID, familyID, reuse, err
}

var errNoToken = gorm.ErrRecordNotFound

func (r *SessionRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE refresh_token SET revoked_at = ? WHERE family_id = ? AND revoked_at IS NULL`,
		now, familyID).Error
}

func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE refresh_token SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		now, userID).Error
}

func (r *SessionRepo) DenyJTI(ctx context.Context, jti string, expires time.Time) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO jti_denylist (jti, expires_at) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		jti, expires).Error
}

func (r *SessionRepo) IsJTIDenied(ctx context.Context, jti string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Raw(
		`SELECT count(*) FROM jti_denylist WHERE jti = ? AND expires_at > now()`, jti).Scan(&n).Error
	return n > 0, err
}

// PurgeExpired keeps the two session tables bounded. They are NOT history
// tables — BR-8.5's "nothing is purged" is about audit and approval events,
// and a dead token row proves nothing.
func (r *SessionRepo) PurgeExpired(ctx context.Context, now time.Time) (int, error) {
	res := r.db.WithContext(ctx).Exec(
		`DELETE FROM refresh_token WHERE expires_at < ?`, now.Add(-24*time.Hour))
	if res.Error != nil {
		return 0, res.Error
	}
	n := res.RowsAffected
	res = r.db.WithContext(ctx).Exec(`DELETE FROM jti_denylist WHERE expires_at < ?`, now)
	return int(n + res.RowsAffected), res.Error
}
