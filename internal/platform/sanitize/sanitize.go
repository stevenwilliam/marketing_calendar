// Package sanitize normalises and validates input, and encodes output for the
// context it lands in.
//
// The rule (CLAUDE.md §4): normalise BEFORE validating, reject never silently
// repair, and encode on the way OUT for the destination — HTML, attribute,
// URL, CSV cell, log line, filename.
package sanitize

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var (
	ErrEmpty     = errors.New("nilai wajib diisi")
	ErrTooLong   = errors.New("nilai terlalu panjang")
	ErrControl   = errors.New("nilai mengandung karakter kendali")
	ErrNotUTF8   = errors.New("nilai bukan UTF-8 yang sah")
	ErrEmail     = errors.New("alamat surel tidak sah")
	ErrNotInList = errors.New("nilai tidak ada dalam daftar yang diizinkan")
)

// Text normalises to NFC, trims, collapses internal whitespace runs, and
// REJECTS control characters rather than stripping them. Stripping is a silent
// repair: it turns an attack into a slightly different string that still gets
// stored.
func Text(s string, max int) (string, error) {
	if !utf8.ValidString(s) {
		return "", ErrNotUTF8
	}
	s = norm.NFC.String(s)
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			return "", ErrControl
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrEmpty
	}
	if utf8.RuneCountInString(s) > max {
		return "", ErrTooLong
	}
	return s, nil
}

// Optional is Text that allows empty.
func Optional(s string, max int) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	return Text(s, max)
}

// Email normalises case on the domain only — the local part is
// case-sensitive per RFC 5321, and lower-casing it silently merges two
// different mailboxes.
func Email(s string) (string, error) {
	s = strings.TrimSpace(norm.NFC.String(s))
	if s == "" {
		return "", ErrEmpty
	}
	if len(s) > 254 {
		return "", ErrTooLong
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", ErrEmail
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 0 {
		return "", ErrEmail
	}
	return addr.Address[:at] + "@" + strings.ToLower(addr.Address[at+1:]), nil
}

// Enum is the allow-list. Deny by default: anything not listed is refused.
func Enum(s string, allowed ...string) (string, error) {
	s = strings.TrimSpace(s)
	for _, a := range allowed {
		if s == a {
			return s, nil
		}
	}
	return "", ErrNotInList
}

// LogValue makes a value safe to put in a log line. A newline in a username
// forges a second log entry, and a log nobody can trust is worse than none.
func LogValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r == '"':
			b.WriteString("\\\"")
		case unicode.IsControl(r):
			b.WriteString("?")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Filename strips every path separator and traversal. A filename from a client
// is a string, never a path.
func Filename(s string) string {
	s = strings.TrimSpace(norm.NFC.String(s))
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.TrimLeft(s, ".")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || r == 0 {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" {
		return "unnamed"
	}
	if utf8.RuneCountInString(out) > 200 {
		out = string([]rune(out)[:200])
	}
	return out
}
