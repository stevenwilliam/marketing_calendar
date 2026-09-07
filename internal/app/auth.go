package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
)

type LoginResult struct {
	// Challenge is a short-lived token proving the password step passed. It is
	// returned only when the second factor is required.
	Challenge    string
	NeedsEnrol   bool
	ProvisionURI string
	Secret       string

	// Session is set when `auth.totp_required` is false: the password alone
	// completes the login and there is no second step. Exactly one of Session
	// and Challenge is ever non-nil, so a caller cannot mistake a challenge
	// for a session (D46).
	Session *Session
}

type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Principal    *Principal
}

// Login verifies the password.
//
// When `auth.totp_required` is true it returns a CHALLENGE and never a session:
// a password alone must not be enough. When it is false the password completes
// the login and the session is issued here.
//
// Either way the unknown-email and wrong-password paths are byte-identical in
// their response and spend the same work, so login timing cannot enumerate
// staff addresses.
func (d *Deps) Login(ctx context.Context, email, password, ua, ip string) (*LoginResult, error) {
	if ok, wait := d.LoginLimiter.Allow(ip); !ok {
		return nil, &apierror.Error{Code: apierror.CodeRateLimited,
			Message: "terlalu banyak percobaan; coba lagi dalam " + wait.Round(time.Second).String()}
	}

	normalised, err := sanitize.Email(email)
	if err != nil {
		security.SpendDummyVerify(password)
		return nil, errInvalidCredentials()
	}

	u, err := d.Users.ByEmail(ctx, normalised)
	if err != nil {
		return nil, err
	}
	if u == nil {
		security.SpendDummyVerify(password)
		return nil, errInvalidCredentials()
	}
	if !u.IsActive {
		security.SpendDummyVerify(password)
		return nil, errInvalidCredentials()
	}
	// A locked account refuses the CORRECT password too — otherwise the lock
	// tells an attacker when they have found it.
	if u.LockedUntil != nil && u.LockedUntil.After(d.Now()) {
		security.SpendDummyVerify(password)
		return nil, errInvalidCredentials()
	}

	if err := security.VerifyPassword(password, u.PasswordHash); err != nil {
		threshold := d.Params.Int(ctx, ParamLockoutThreshold, 5)
		minutes := d.Params.Int(ctx, ParamLockoutMinutes, 15)
		_ = d.Users.RecordLoginFailure(ctx, u.UserID, time.Duration(minutes)*time.Minute, threshold)
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &u.UserID, Action: "auth.login_failed",
			SubjectType: "app_user", SubjectID: &u.UserID, IP: ip})
		return nil, errInvalidCredentials()
	}

	// The second factor is a parameter (D46). Read once, so a change part way
	// through a login cannot leave a half-authenticated state.
	if !d.Params.Bool(ctx, ParamTOTPRequired, false) {
		d.LoginLimiter.Reset(ip)
		_ = d.Users.ClearLoginFailures(ctx, u.UserID)
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &u.UserID, Action: "auth.login",
			SubjectType: "app_user", SubjectID: &u.UserID, IP: ip,
			Reason: "faktor kedua dinonaktifkan (auth.totp_required=false)"})
		s, err := d.issueSession(ctx, u.UserID, uuid.Nil, ua, ip)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Session: s}, nil
	}

	res := &LoginResult{}
	if !u.TOTPConfirmed {
		// BR-5.3: without a confirmed enrolment the user reaches only the
		// enrolment flow.
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer: d.Cfg.TOTPIssuer, AccountName: u.Email})
		if err != nil {
			return nil, apierror.Wrap(apierror.CodeInternal, "tidak dapat membuat kunci TOTP", err)
		}
		if err := d.Users.SetTOTP(ctx, u.UserID, key.Secret()); err != nil {
			return nil, err
		}
		res.NeedsEnrol = true
		res.ProvisionURI = key.URL()
		res.Secret = key.Secret()
	}

	challenge, _, _, err := d.Tokens.Issue(u.UserID, d.Now())
	if err != nil {
		return nil, err
	}
	res.Challenge = challenge
	return res, nil
}

// VerifyTOTP completes the second factor and issues the session.
func (d *Deps) VerifyTOTP(ctx context.Context, challenge, code, ua, ip string) (*Session, error) {
	if ok, wait := d.LoginLimiter.Allow("totp:" + ip); !ok {
		return nil, &apierror.Error{Code: apierror.CodeRateLimited,
			Message: "terlalu banyak percobaan; coba lagi dalam " + wait.Round(time.Second).String()}
	}
	if !d.Params.Bool(ctx, ParamTOTPRequired, false) {
		// Switched off mid-flight. Refusing is the honest answer: the caller's
		// challenge is real, but the step it belongs to no longer exists, and
		// issuing a session here would accept a token minted for another flow.
		return nil, apierror.New(apierror.CodeConflict,
			"faktor kedua sedang dinonaktifkan; masuk kembali dengan kata sandi saja")
	}
	claims, err := d.Tokens.Parse(challenge)
	if err != nil {
		return nil, apierror.New(apierror.CodeUnauthenticated, "tantangan tidak sah atau kedaluwarsa")
	}
	u, err := d.Users.ByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if u.TOTPSecret == "" {
		return nil, apierror.New(apierror.CodeTOTPRequired, "TOTP belum didaftarkan")
	}
	if !totp.Validate(code, u.TOTPSecret) {
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &u.UserID, Action: "auth.totp_failed",
			SubjectType: "app_user", SubjectID: &u.UserID, IP: ip})
		return nil, apierror.New(apierror.CodeUnauthenticated, "kode autentikasi salah")
	}
	if !u.TOTPConfirmed {
		if err := d.Users.ConfirmTOTP(ctx, u.UserID); err != nil {
			return nil, err
		}
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &u.UserID, Action: "auth.totp_enrolled",
			SubjectType: "app_user", SubjectID: &u.UserID, IP: ip})
	}

	d.LoginLimiter.Reset(ip)
	_ = d.Users.ClearLoginFailures(ctx, u.UserID)
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &u.UserID, Action: "auth.login",
		SubjectType: "app_user", SubjectID: &u.UserID, IP: ip})
	return d.issueSession(ctx, u.UserID, uuid.Nil, ua, ip)
}

func (d *Deps) issueSession(ctx context.Context, userID, familyID uuid.UUID, ua, ip string) (*Session, error) {
	access, _, expires, err := d.Tokens.Issue(userID, d.Now())
	if err != nil {
		return nil, err
	}
	refresh, err := security.NewToken()
	if err != nil {
		return nil, err
	}
	if familyID == uuid.Nil {
		familyID = id.New()
	}
	err = d.Sessions.IssueRefresh(ctx, userID, familyID, security.HashToken(refresh),
		d.Now().Add(d.Cfg.RefreshTokenTTL), ua, ip)
	if err != nil {
		return nil, err
	}
	p, err := d.Users.Principal(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires, Principal: p}, nil
}

// Refresh rotates. Presenting a token that was already used revokes the whole
// family — the signature of a stolen token (BR-5.6).
func (d *Deps) Refresh(ctx context.Context, refreshToken, ua, ip string) (*Session, error) {
	userID, familyID, reuse, err := d.Sessions.RotateRefresh(ctx, security.HashToken(refreshToken), d.Now())
	if err != nil {
		return nil, apierror.New(apierror.CodeUnauthenticated, "sesi tidak sah; masuk kembali")
	}
	if reuse {
		_ = d.Audit.Write(ctx, AuditEntry{ActorID: &userID, Action: "auth.refresh_reuse_detected",
			SubjectType: "app_user", SubjectID: &userID, IP: ip,
			Reason: "token refresh yang sudah dipakai dipresentasikan lagi; seluruh keluarga dicabut"})
		return nil, apierror.New(apierror.CodeUnauthenticated, "sesi dicabut; masuk kembali")
	}
	return d.issueSession(ctx, userID, familyID, ua, ip)
}

func (d *Deps) Logout(ctx context.Context, jti string, jtiExpiry time.Time, refreshToken string, userID uuid.UUID) error {
	if jti != "" {
		if err := d.Sessions.DenyJTI(ctx, jti, jtiExpiry); err != nil {
			return err
		}
	}
	if refreshToken != "" {
		_, familyID, _, err := d.Sessions.RotateRefresh(ctx, security.HashToken(refreshToken), d.Now())
		if err == nil {
			_ = d.Sessions.RevokeFamily(ctx, familyID, d.Now())
		}
	}
	_ = d.Audit.Write(ctx, AuditEntry{ActorID: &userID, Action: "auth.logout",
		SubjectType: "app_user", SubjectID: &userID})
	return nil
}

// errInvalidCredentials is ONE message for every failure mode above: unknown
// email, wrong password, inactive account, locked account. Distinguishing them
// is how an attacker enumerates.
func errInvalidCredentials() error {
	return apierror.New(apierror.CodeUnauthenticated, "surel atau kata sandi salah")
}
