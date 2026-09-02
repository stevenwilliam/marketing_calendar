package postgres

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func isNoRows(err error) bool {
	return err != nil && errors.Is(err, sql.ErrNoRows)
}

// textArray scans a PostgreSQL text[] into a []string without pulling in a
// driver-specific array type. The pgx/pq wire form is {a,b,"c,d"}.
type textArray struct{ dst *[]string }

func pqArray(dst *[]string) sql.Scanner { return &textArray{dst: dst} }

func (a *textArray) Scan(src any) error {
	*a.dst = nil
	if src == nil {
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("textArray: tipe tidak terduga %T", src)
	}
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}
	var cur strings.Builder
	inQuote, escaped := false, false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case r == ',' && !inQuote:
			*a.dst = append(*a.dst, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	*a.dst = append(*a.dst, cur.String())
	return nil
}

// uuidList renders ids as a PostgreSQL array LITERAL, e.g. "{a,b,c}", to be
// passed as ONE placeholder and cast with `?::uuid[]`.
//
// It is a single string on purpose. Passing a Go slice to gorm makes gorm
// expand it into a comma-separated placeholder list — correct for `IN (?)`
// and malformed for `= ANY(?)`, which is how this produced
// "malformed array literal" against the live database. A UUID cannot contain
// a comma or a quote, so the literal needs no escaping and cannot be injected
// through; the values are typed uuid.UUID, not caller strings.
func uuidList(in []uuid.UUID) string {
	if len(in) == 0 {
		return "{}"
	}
	parts := make([]string, len(in))
	for i, u := range in {
		parts[i] = u.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// textList is the same for text[], with quoting, used for the role:company
// pairs of the approval inbox.
func textList(in []string) string {
	if len(in) == 0 {
		return "{}"
	}
	parts := make([]string, len(in))
	for i, s := range in {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		parts[i] = `"` + s + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// nullUUID keeps a nullable id readable at the call site.
func nullUUID(u *uuid.UUID) driver.Valuer {
	if u == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *u, Valid: true}
}
