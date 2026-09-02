package postgres

import "strings"

// isForeignKeyViolation and friends read the driver's message rather than a
// typed code, because the pgx and lib/pq shapes differ and the repository must
// map either into a domain error rather than leaking it to a client.
func isForeignKeyViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "SQLSTATE 23503") ||
		strings.Contains(err.Error(), "violates foreign key constraint"))
}

func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "SQLSTATE 23505") ||
		strings.Contains(err.Error(), "duplicate key value"))
}

func isCheckViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "SQLSTATE 23514") ||
		strings.Contains(err.Error(), "violates check constraint"))
}

// isAppendOnlyViolation is refuse_mutation() firing (BR-8.1).
func isAppendOnlyViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "SQLSTATE 2F004") ||
		strings.Contains(err.Error(), "append-only"))
}
