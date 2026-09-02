package postgres

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"gorm.io/gorm"
)

// ParamRepo caches sys_parameters for a short window. The lead time is read on
// every promotion submit; a database round trip per read would be waste, and a
// cache without an expiry would mean an administrator's change appears to do
// nothing (BR-1.6 says a change takes effect on the next evaluation).
type ParamRepo struct {
	db  *gorm.DB
	mu  sync.RWMutex
	val map[string]app.Param
	at  time.Time
	ttl time.Duration
}

func NewParamRepo(db *gorm.DB) *ParamRepo {
	return &ParamRepo{db: db, ttl: 15 * time.Second}
}

func (r *ParamRepo) load(ctx context.Context) map[string]app.Param {
	r.mu.RLock()
	if r.val != nil && time.Since(r.at) < r.ttl {
		v := r.val
		r.mu.RUnlock()
		return v
	}
	r.mu.RUnlock()

	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT param_key, param_value, value_type, description, is_secret, updated_by, updated_at
		  FROM sys_parameters`).Rows()
	if err != nil {
		// A failed refresh keeps the last good values rather than silently
		// reverting every parameter to its compiled-in default.
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.val
	}
	defer rows.Close()
	out := map[string]app.Param{}
	for rows.Next() {
		var p app.Param
		if err := rows.Scan(&p.Key, &p.Value, &p.ValueType, &p.Description,
			&p.IsSecret, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			continue
		}
		out[p.Key] = p
	}
	r.mu.Lock()
	r.val, r.at = out, time.Now()
	r.mu.Unlock()
	return out
}

func (r *ParamRepo) All(ctx context.Context) ([]app.Param, error) {
	m := r.load(ctx)
	out := make([]app.Param, 0, len(m))
	for _, p := range m {
		// BR-8.4: a secret-flagged value is masked wherever it is rendered.
		if p.IsSecret {
			p.Value = "••••••"
		}
		out = append(out, p)
	}
	sortParams(out)
	return out, nil
}

func sortParams(p []app.Param) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j].Key < p[j-1].Key; j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}

func (r *ParamRepo) Int(ctx context.Context, key string, def int) int {
	if p, ok := r.load(ctx)[key]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(p.Value)); err == nil {
			return n
		}
	}
	return def
}

func (r *ParamRepo) Bool(ctx context.Context, key string, def bool) bool {
	if p, ok := r.load(ctx)[key]; ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(p.Value)); err == nil {
			return b
		}
	}
	return def
}

func (r *ParamRepo) String(ctx context.Context, key, def string) string {
	if p, ok := r.load(ctx)[key]; ok && p.Value != "" {
		return p.Value
	}
	return def
}

// List splits a comma-separated parameter, trimming and dropping empties. The
// release recipient list (D32) is the reason this exists.
func (r *ParamRepo) List(ctx context.Context, key string) []string {
	raw := r.String(ctx, key, "")
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (r *ParamRepo) Set(ctx context.Context, key, value string, actor uuid.UUID) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE sys_parameters SET param_value = ?, updated_by = ?, updated_at = now()
		 WHERE param_key = ?`, value, actor, key).Error
	if err == nil {
		r.mu.Lock()
		r.val = nil // force a refresh so the change is visible immediately
		r.mu.Unlock()
	}
	return err
}
