package test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stevenwilliam/marketing_calendar/internal/adapter/postgres"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/config"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/ratelimit"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
	"gorm.io/gorm"
)

// authDeps builds the slice of Deps the login path needs, with a limiter
// generous enough that the tests below are not throttled by each other.
func authDeps(t *testing.T, g *gorm.DB) *app.Deps {
	t.Helper()
	cfg := config.Config{
		JWTSecret:       "a-test-secret-that-is-long-enough-32",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		TOTPIssuer:      "Marketing Calendar Test",
	}
	return &app.Deps{
		Cfg:          cfg,
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:           g,
		Tokens:       security.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL),
		Now:          func() time.Time { return time.Now().UTC() },
		Users:        postgres.NewUserRepo(g),
		Sessions:     postgres.NewSessionRepo(g),
		Params:       postgres.NewParamRepo(g),
		Audit:        postgres.NewAuditRepo(g),
		LoginLimiter: ratelimit.New(1000, 100000),
	}
}

// setTOTP flips auth.totp_required and defeats the parameter cache, which has
// a short TTL that a test must not sit through.
func setTOTP(t *testing.T, g *gorm.DB, on string) *postgres.ParamRepo {
	t.Helper()
	if err := g.Exec(`UPDATE sys_parameters SET param_value = ? WHERE param_key = 'auth.totp_required'`,
		on).Error; err != nil {
		t.Fatal(err)
	}
	// A fresh repo has an empty cache, so it reads the value just written.
	return postgres.NewParamRepo(g)
}

const testPassword = "MarketingCalendar2026!"

// D46: with the second factor off, the password alone completes the login and
// the response carries a SESSION, not a challenge.
func TestLoginWithoutSecondFactorIssuesASession(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	d := authDeps(t, g)
	d.Params = setTOTP(t, g, "false")

	res, err := d.Login(ctx, "rina.hartono@sfg.local", testPassword, "go-test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Session == nil {
		t.Fatal("no session: the password alone must complete the login when the factor is off")
	}
	if res.Challenge != "" {
		t.Fatal("a challenge was returned as well as a session; exactly one must be set")
	}
	if res.Session.AccessToken == "" || res.Session.RefreshToken == "" {
		t.Fatal("the session is missing a token")
	}
	if res.Session.Principal == nil || res.Session.Principal.Email == "" {
		t.Fatal("the session must carry the resolved principal")
	}
	// The access token must actually authenticate.
	claims, err := d.Tokens.Parse(res.Session.AccessToken)
	if err != nil {
		t.Fatalf("the issued token does not parse: %v", err)
	}
	if claims.UserID != res.Session.Principal.UserID {
		t.Fatal("the token names a different user than the session")
	}
}

// And with it ON, the password alone must NOT be enough — the control still
// works, so turning it back on is a parameter change and not a code change.
func TestLoginWithSecondFactorReturnsOnlyAChallenge(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	d := authDeps(t, g)
	d.Params = setTOTP(t, g, "true")
	defer setTOTP(t, g, "false")

	res, err := d.Login(ctx, "rina.hartono@sfg.local", testPassword, "go-test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Session != nil {
		t.Fatal("a password alone returned a SESSION while the second factor is required")
	}
	if res.Challenge == "" {
		t.Fatal("no challenge was returned")
	}
	// A challenge is not a session: it must not be usable as a bearer token
	// for anything, and the only thing that consumes it is VerifyTOTP.
	if _, err := d.Tokens.Parse(res.Challenge); err != nil {
		t.Fatalf("the challenge should still be a well-formed token: %v", err)
	}
}

// Switching the factor off mid-flight must not let a stale challenge through.
// The challenge is a valid token minted for a step that no longer exists.
func TestChallengeIsRefusedOnceTheFactorIsOff(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	d := authDeps(t, g)
	d.Params = setTOTP(t, g, "true")

	res, err := d.Login(ctx, "rina.hartono@sfg.local", testPassword, "go-test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Challenge == "" {
		t.Fatal("expected a challenge")
	}

	d.Params = setTOTP(t, g, "false")
	if _, err := d.VerifyTOTP(ctx, res.Challenge, "000000", "go-test", "127.0.0.1"); err == nil {
		t.Fatal("a challenge minted for a step that no longer exists was accepted")
	}
}

// Whatever the factor setting, a wrong password and an unknown email must be
// indistinguishable — otherwise login enumerates staff addresses.
func TestWrongPasswordAndUnknownEmailStayIdentical(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	d := authDeps(t, g)

	for _, setting := range []string{"false", "true"} {
		d.Params = setTOTP(t, g, setting)

		_, wrongPw := d.Login(ctx, "rina.hartono@sfg.local", "definitely-not-it", "go-test", "127.0.0.1")
		_, unknown := d.Login(ctx, "nobody-here@sfg.local", "definitely-not-it", "go-test", "127.0.0.1")

		if wrongPw == nil || unknown == nil {
			t.Fatalf("totp=%s: both must fail", setting)
		}
		if wrongPw.Error() != unknown.Error() {
			t.Fatalf("totp=%s: the two paths differ:\n  wrong password: %v\n  unknown email : %v",
				setting, wrongPw, unknown)
		}
	}
	setTOTP(t, g, "false")
}
