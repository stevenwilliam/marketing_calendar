// Package id issues UUIDv7 primary keys.
//
// v7 is time-ordered, which gives index locality on insert without the volume
// leak of a sequential integer: a competitor counting our plan ids learns how
// many promotions the group runs.
package id

import "github.com/google/uuid"

// New returns a UUIDv7. It panics only if the system CSPRNG fails, which is
// not a condition any caller can handle meaningfully.
func New() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		panic("id: CSPRNG tidak tersedia: " + err.Error())
	}
	return u
}

func NewString() string { return New().String() }

// Parse is strict: an invalid id is a 404 to the caller, never a zero UUID
// that silently matches nothing or, worse, matches a zero-valued row.
func Parse(s string) (uuid.UUID, error) { return uuid.Parse(s) }
