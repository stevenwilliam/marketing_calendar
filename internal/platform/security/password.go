// Package security is password hashing, tokens and TOTP.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (12-security.md §3): 64 MiB, t=3, p=4.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

var (
	ErrBadHash  = errors.New("format hash kata sandi tidak dikenali")
	ErrMismatch = errors.New("kata sandi salah")
	ErrWeak     = errors.New("kata sandi terlalu lemah")
)

// HashPassword returns a PHC-encoded argon2id hash.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword is constant-time in the comparison. It returns ErrMismatch
// for a wrong password and ErrBadHash for a stored value that is not an
// argon2id hash — the two must not be conflated, because a bcrypt or plaintext
// value loaded by a fixture would otherwise silently "not match" instead of
// being reported as the schema violation it is.
func VerifyPassword(plain, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return ErrBadHash
	}
	var mem uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return ErrBadHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrBadHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrBadHash
	}
	got := argon2.IDKey([]byte(plain), salt, time, mem, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// dummyHash is verified against when the email is unknown, so an unknown
// account and a wrong password cost the same work. Without it, login timing
// enumerates every staff address in the group.
var dummyHash string

func init() {
	h, err := HashPassword("dummy-password-for-constant-time-login")
	if err != nil {
		panic("security: cannot build dummy hash: " + err.Error())
	}
	dummyHash = h
}

// SpendDummyVerify burns the same work as a real verify. Call it on the
// unknown-email path.
func SpendDummyVerify(plain string) {
	_ = VerifyPassword(plain, dummyHash)
}

// DefaultPasswordMinLength is the fallback when the parameter is unreadable.
// It is the floor the product ships with, not a recommendation: every extra
// character is worth far more than any composition rule.
const DefaultPasswordMinLength = 8

// CheckPasswordStrength is deliberately about length, not character classes.
// Composition rules push people to "Password1!" — length is what actually
// costs an attacker.
//
// The minimum is passed in rather than fixed here, because it is a threshold
// the business changes without a deploy (CLAUDE.md §7) and it now lives in
// `sys_parameters` as `auth.password_min_length`. A non-positive value falls
// back to the default rather than disabling the check: a misconfigured
// parameter must not silently turn a control off.
func CheckPasswordStrength(plain string, min int) error {
	if min <= 0 {
		min = DefaultPasswordMinLength
	}
	if len([]rune(plain)) < min {
		return ErrWeak
	}
	return nil
}

// HashToken is SHA-256 for refresh tokens: they are high-entropy random
// values, so a slow KDF buys nothing and costs latency on every refresh.
func HashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// NewToken returns 32 bytes of CSPRNG as URL-safe base64.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// crockford is the human-facing code alphabet: no I, L, O or U, so a code read
// over the phone cannot be transcribed into a different one.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewCode returns a CSPRNG Crockford base32 code of n characters.
func NewCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, x := range b {
		out[i] = crockford[int(x)%len(crockford)]
	}
	return string(out), nil
}
