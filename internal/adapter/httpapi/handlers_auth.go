package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
)

const refreshCookie = "mc_refresh"

func setRefreshCookie(c *gin.Context, d *app.Deps, token string) {
	// HttpOnly so script cannot read it; SameSite=Strict because there is no
	// cross-site flow in an internal tool; Secure in production, where nginx
	// terminates TLS.
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Value: token, Path: "/api/v1/auth",
		HttpOnly: true, Secure: d.Cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(d.Cfg.RefreshTokenTTL / time.Second),
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		res, err := d.Login(c.Request.Context(), req.Email, req.Password, clientIP(c))
		if err != nil {
			fail(c, err)
			return
		}
		// A password alone NEVER returns a session (BR-5.3). What comes back
		// is a challenge and, on first login, the enrolment material.
		out := gin.H{"code": string(apierror.CodeTOTPRequired), "challenge": res.Challenge,
			"needs_enrolment": res.NeedsEnrol}
		if res.NeedsEnrol {
			out["provisioning_uri"] = res.ProvisionURI
			out["secret"] = res.Secret
		}
		c.JSON(http.StatusOK, out)
	}
}

type totpRequest struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
}

func handleVerifyTOTP(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req totpRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, apierror.Validation("badan permintaan tidak valid", nil))
			return
		}
		s, err := d.VerifyTOTP(c.Request.Context(), req.Challenge, req.Code,
			c.GetHeader("User-Agent"), clientIP(c))
		if err != nil {
			fail(c, err)
			return
		}
		setRefreshCookie(c, d, s.RefreshToken)
		c.JSON(http.StatusOK, gin.H{
			"access_token": s.AccessToken, "expires_at": s.ExpiresAt,
			"user": principalJSON(*s.Principal)})
	}
}

func handleRefresh(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(refreshCookie)
		if err != nil || token == "" {
			fail(c, apierror.New(apierror.CodeUnauthenticated, "sesi tidak ditemukan"))
			return
		}
		s, err := d.Refresh(c.Request.Context(), token, c.GetHeader("User-Agent"), clientIP(c))
		if err != nil {
			// Clear the cookie so a revoked family does not loop the client.
			http.SetCookie(c.Writer, &http.Cookie{Name: refreshCookie, Value: "",
				Path: "/api/v1/auth", MaxAge: -1, HttpOnly: true})
			fail(c, err)
			return
		}
		setRefreshCookie(c, d, s.RefreshToken)
		c.JSON(http.StatusOK, gin.H{
			"access_token": s.AccessToken, "expires_at": s.ExpiresAt,
			"user": principalJSON(*s.Principal)})
	}
}

func handleLogout(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		jti, _ := c.Get(ctxJTI)
		exp, _ := c.Get(ctxJTIExpiry)
		jtiStr, _ := jti.(string)
		expTime, _ := exp.(time.Time)
		if expTime.IsZero() {
			expTime = d.Now().Add(d.Cfg.AccessTokenTTL)
		}
		token, _ := c.Cookie(refreshCookie)
		if err := d.Logout(c.Request.Context(), jtiStr, expTime, token, p.UserID); err != nil {
			fail(c, err)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: refreshCookie, Value: "",
			Path: "/api/v1/auth", MaxAge: -1, HttpOnly: true})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleMe(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, principalJSON(principal(c)))
	}
}

func principalJSON(p app.Principal) gin.H {
	perms := make([]string, 0, len(p.Permissions))
	for k := range p.Permissions {
		perms = append(perms, k)
	}
	grants := make([]gin.H, 0, len(p.Grants))
	for _, g := range p.Grants {
		grants = append(grants, gin.H{"role_id": g.RoleID, "role_code": g.RoleCode,
			"company_id": g.CompanyID})
	}
	return gin.H{
		"user_id": p.UserID, "email": p.Email, "full_name": p.FullName,
		"permissions": perms, "grants": grants, "company_ids": p.CompanyIDs,
		"is_superadmin": p.IsSuperadmin,
	}
}
