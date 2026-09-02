// Package httpapi is the HTTP adapter: handlers, request and response mapping.
// It owns no business logic; every decision it renders was made in app or
// domain.
package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/logging"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/sanitize"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/security"
)

const (
	ctxPrincipal = "principal"
	ctxTrace     = "trace_id"
	ctxJTI       = "jti"
	ctxJTIExpiry = "jti_expiry"
)

// errorResponse is the ONE JSON error shape (04 §2). A driver error never
// reaches it: apierror.As guarantees an *Error, and only Code, Message and
// Fields are rendered.
type errorResponse struct {
	Code    apierror.Code     `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	TraceID string            `json:"trace_id"`
}

func fail(c *gin.Context, err error) {
	e := apierror.As(err)
	trace, _ := c.Get(ctxTrace)
	traceID, _ := trace.(string)

	if e.Code == apierror.CodeInternal && e.Cause != nil {
		// The cause is logged, never rendered. This is the line that stops a
		// constraint name or a table name reaching a client.
		if d := deps(c); d != nil {
			d.Log.Error("request failed",
				logging.Str("path", c.FullPath()),
				logging.Str("trace_id", traceID),
				logging.Str("err", e.Cause.Error()))
		}
	}
	c.AbortWithStatusJSON(e.HTTPStatus(), errorResponse{
		Code: e.Code, Message: e.Message, Fields: e.Fields, TraceID: traceID})
}

func deps(c *gin.Context) *app.Deps {
	if v, ok := c.Get("deps"); ok {
		if d, ok := v.(*app.Deps); ok {
			return d
		}
	}
	return nil
}

func principal(c *gin.Context) app.Principal {
	v, _ := c.Get(ctxPrincipal)
	p, _ := v.(app.Principal)
	return p
}

// traceMiddleware gives every request an id that appears in the log line and
// in the error body, so a user reporting "it said 500" can be found.
func traceMiddleware(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		trace := c.GetHeader("X-Request-Id")
		if trace == "" || len(trace) > 64 {
			trace = id.NewString()
		}
		trace = sanitize.LogValue(trace)
		c.Set(ctxTrace, trace)
		c.Set("deps", d)
		c.Header("X-Request-Id", trace)

		start := time.Now()
		c.Next()

		d.Log.Info("request",
			logging.Str("method", c.Request.Method),
			logging.Str("path", c.Request.URL.Path),
			logging.Str("trace_id", trace),
			logging.Str("status", http.StatusText(c.Writer.Status())),
			logging.Str("duration", time.Since(start).Round(time.Millisecond).String()))
	}
}

// securityHeaders. There is no public surface here (D26), but a staff browser
// is still a browser.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// No CDN is allowed: fonts and scripts are self-hosted, which is also
		// why the font-src list has no third party in it.
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
				"font-src 'self'; script-src 'self'; connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		c.Next()
	}
}

// authenticate resolves the principal FRESH from the database on every
// request. Permissions are not carried in the token, so revoking a role takes
// effect on the next request rather than in fifteen minutes.
func authenticate(d *app.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			fail(c, apierror.New(apierror.CodeUnauthenticated, "token tidak disertakan"))
			return
		}
		claims, err := d.Tokens.Parse(strings.TrimPrefix(raw, "Bearer "))
		if err != nil {
			fail(c, apierror.New(apierror.CodeUnauthenticated, "token tidak sah atau kedaluwarsa"))
			return
		}
		// Logout revokes the access token for its remaining life.
		denied, err := d.Sessions.IsJTIDenied(c.Request.Context(), claims.JTI)
		if err != nil {
			fail(c, err)
			return
		}
		if denied {
			fail(c, apierror.New(apierror.CodeUnauthenticated, "sesi sudah diakhiri"))
			return
		}
		p, err := d.Users.Principal(c.Request.Context(), claims.UserID)
		if err != nil {
			fail(c, apierror.New(apierror.CodeUnauthenticated, "pengguna tidak aktif"))
			return
		}
		c.Set(ctxPrincipal, *p)
		c.Set(ctxJTI, claims.JTI)
		if claims.ExpiresAt != nil {
			c.Set(ctxJTIExpiry, claims.ExpiresAt.Time)
		}
		c.Next()
	}
}

// require declares the permission a route needs. Deny by default: a route
// without this middleware is not reachable, because the router never mounts
// one without it.
func require(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := principal(c)
		if !p.Can(permission) {
			fail(c, apierror.Newf(apierror.CodeForbidden, "tidak memiliki izin %s", permission))
			return
		}
		c.Next()
	}
}

func clientIP(c *gin.Context) string { return c.ClientIP() }

func parseUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	u, err := id.Parse(c.Param(name))
	if err != nil {
		// An unparsable id is a 404, not a 400: it tells the caller nothing
		// about what does or does not exist.
		fail(c, apierror.NotFound("sumber daya"))
		return uuid.Nil, false
	}
	return u, true
}

var _ = security.HashToken
