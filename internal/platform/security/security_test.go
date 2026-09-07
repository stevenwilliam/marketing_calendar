package security

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestArgon2idRoundTrip(t *testing.T) {
	h, err := HashPassword("kata sandi yang cukup panjang")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("hash is not argon2id: %q", h)
	}
	if err := VerifyPassword("kata sandi yang cukup panjang", h); err != nil {
		t.Fatalf("correct password rejected: %v", err)
	}
	if err := VerifyPassword("kata sandi yang salah", h); err != ErrMismatch {
		t.Fatalf("want ErrMismatch, got %v", err)
	}
}

// The same password hashed twice must differ — otherwise the salt is not
// random and the whole table is one rainbow-table lookup.
func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("sama persis")
	b, _ := HashPassword("sama persis")
	if a == b {
		t.Fatal("two hashes of the same password are identical: the salt is not random")
	}
}

// A bcrypt or plaintext value loaded by a fixture must be reported as a schema
// violation, not silently "not matching" like a wrong password.
func TestNonArgonHashIsDistinguishedFromAWrongPassword(t *testing.T) {
	for _, stored := range []string{
		"$2y$10$abcdefghijklmnopqrstuv", // bcrypt
		"plaintext",
		"",
		"$argon2id$broken",
	} {
		if err := VerifyPassword("apa saja", stored); err != ErrBadHash {
			t.Fatalf("stored %q: want ErrBadHash, got %v", stored, err)
		}
	}
}

func TestPasswordStrengthIsLengthNotComposition(t *testing.T) {
	// Seven characters is refused however many symbol classes it carries.
	if err := CheckPasswordStrength("Pa$$w0r", DefaultPasswordMinLength); err != ErrWeak {
		t.Fatal("seven characters must be refused however many symbol classes it has")
	}
	// The boundary itself is legal. An off-by-one here refuses a password the
	// policy permits, which is the kind of thing nobody reports as a bug.
	if err := CheckPasswordStrength("lalalala", DefaultPasswordMinLength); err != nil {
		t.Fatalf("exactly the minimum must be accepted: %v", err)
	}
	if err := CheckPasswordStrength("kucing oranye makan nasi goreng", DefaultPasswordMinLength); err != nil {
		t.Fatalf("a long passphrase must be accepted: %v", err)
	}
}

// The minimum is a sys_parameter, so it moves without a deploy.
func TestPasswordMinimumIsConfigurable(t *testing.T) {
	if err := CheckPasswordStrength("lalalala", 16); err != ErrWeak {
		t.Fatal("raising the minimum must refuse a password that was previously fine")
	}
	if err := CheckPasswordStrength("abc", 3); err != nil {
		t.Fatalf("lowering the minimum must accept a shorter password: %v", err)
	}
}

// A misconfigured parameter must not silently turn the control off. Zero or a
// negative value falls back to the default rather than accepting anything.
func TestNonPositiveMinimumFallsBackNotOpen(t *testing.T) {
	for _, min := range []int{0, -1, -100} {
		if err := CheckPasswordStrength("x", min); err != ErrWeak {
			t.Fatalf("min=%d accepted a one-character password: the control was disabled by a bad value", min)
		}
	}
}

// Login must cost the same whether the email exists or not, or its timing
// enumerates every staff address in the group.
func TestSpendDummyVerifyCostsRealWork(t *testing.T) {
	real, _ := HashPassword("kata sandi panjang sekali")
	start := time.Now()
	_ = VerifyPassword("tebakan yang salah", real)
	realCost := time.Since(start)

	start = time.Now()
	SpendDummyVerify("tebakan yang salah")
	dummyCost := time.Since(start)

	// Within an order of magnitude is the claim; anything tighter is flaky on
	// a shared box. The failure this catches is the dummy path returning
	// instantly, which is a 1000x difference, not a 2x one.
	if dummyCost*10 < realCost {
		t.Fatalf("the unknown-email path is far cheaper than a real verify: %v vs %v", dummyCost, realCost)
	}
}

func TestJWTRoundTrip(t *testing.T) {
	iss := NewTokenIssuer(strings.Repeat("k", 32), 15*time.Minute)
	uid := uuid.New()
	now := time.Now()
	tok, jti, exp, err := iss.Issue(uid, now)
	if err != nil {
		t.Fatal(err)
	}
	if jti == "" || !exp.After(now) {
		t.Fatal("issue must return a jti and a future expiry")
	}
	c, err := iss.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != uid || c.JTI != jti {
		t.Fatal("claims did not round trip")
	}
}

// alg=none and algorithm confusion both authenticate anybody without the pin.
func TestJWTRefusesUnpinnedAlgorithms(t *testing.T) {
	iss := NewTokenIssuer(strings.Repeat("k", 32), time.Minute)
	// A hand-built alg=none token for a real-looking user.
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJ1aWQiOiIwMTkyYjRlNy0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDEifQ."
	if _, err := iss.Parse(none); err == nil {
		t.Fatal("alg=none must be refused")
	}
	if _, err := iss.Parse("not.a.token"); err == nil {
		t.Fatal("garbage must be refused")
	}
}

func TestJWTRefusesAnotherSecret(t *testing.T) {
	a := NewTokenIssuer(strings.Repeat("a", 32), time.Minute)
	b := NewTokenIssuer(strings.Repeat("b", 32), time.Minute)
	tok, _, _, _ := a.Issue(uuid.New(), time.Now())
	if _, err := b.Parse(tok); err != ErrTokenInvalid {
		t.Fatalf("a token signed with another secret must be invalid, got %v", err)
	}
}

func TestJWTExpiryIsReported(t *testing.T) {
	iss := NewTokenIssuer(strings.Repeat("k", 32), time.Minute)
	tok, _, _, _ := iss.Issue(uuid.New(), time.Now().Add(-2*time.Hour))
	if _, err := iss.Parse(tok); err != ErrTokenExpired {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

// A refresh token is stored hashed, so a database read does not hand over
// live sessions.
func TestTokenHashingIsStable(t *testing.T) {
	tok, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if HashToken(tok) != HashToken(tok) {
		t.Fatal("HashToken is not deterministic")
	}
	if HashToken(tok) == tok {
		t.Fatal("the stored value must not be the token itself")
	}
	other, _ := NewToken()
	if tok == other {
		t.Fatal("NewToken repeated itself")
	}
}

// Crockford base32 omits I, L, O and U so a code read aloud cannot be
// transcribed into a different one.
func TestNewCodeAvoidsAmbiguousLetters(t *testing.T) {
	for i := 0; i < 200; i++ {
		c, err := NewCode(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != 10 {
			t.Fatalf("length = %d", len(c))
		}
		if strings.ContainsAny(c, "ILOU") {
			t.Fatalf("code %q contains an ambiguous letter", c)
		}
	}
}
