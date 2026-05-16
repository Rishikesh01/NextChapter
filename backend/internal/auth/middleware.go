package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
)

// ctxKey is unexported so callers must go through [UserFromContext] and
// [RequireUser]. This avoids collisions with other packages' context keys.
type ctxKey int

const (
	ctxUserKey ctxKey = iota + 1
	ctxResolvedKey
)

// UserFromContext returns the authenticated user attached to ctx by the
// middleware, plus a bool indicating presence. Domain code can call this
// without knowing about gin.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxUserKey).(User)
	return u, ok
}

// ResolvedFromContext returns the auth lookup metadata for callers that
// need to know the token id / kind (e.g. /auth/logout).
func ResolvedFromContext(ctx context.Context) (Resolved, bool) {
	r, ok := ctx.Value(ctxResolvedKey).(Resolved)
	return r, ok
}

// Now is the clock the middleware uses. Tests replace this with a fixed
// value via the middleware constructor.
type Now func() time.Time

// Unauthorized is the response writer interface the middleware uses to
// reject a request. The httpapi/render package implements this for gin;
// keeping it as a function pointer means the auth package does not depend
// on render.
type Unauthorized func(c *gin.Context, message string)

// MiddlewareConfig wires the middleware. All fields are required.
type MiddlewareConfig struct {
	Service      *Service
	Now          Now
	Unauthorized Unauthorized
	Logger       *zap.Logger
}

// Middleware returns a gin handler that resolves a session cookie or
// bearer token to a user and attaches it to the request context. Routes
// behind this middleware can assume [UserFromContext] returns ok=true.
func Middleware(cfg MiddlewareConfig) gin.HandlerFunc {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Unauthorized == nil {
		panic("auth: MiddlewareConfig.Unauthorized must not be nil")
	}
	if cfg.Service == nil {
		panic("auth: MiddlewareConfig.Service must not be nil")
	}
	return func(c *gin.Context) {
		raw, source, ok := extractToken(c)
		if !ok {
			cfg.Unauthorized(c, "missing or invalid credentials")
			return
		}
		resolved, err := cfg.Service.Resolve(c.Request.Context(), raw, cfg.Now())
		if err != nil {
			if !errors.Is(err, ErrTokenNotFound) {
				cfg.Logger.Warn("auth lookup failed", zap.Error(err))
			}
			cfg.Unauthorized(c, "missing or invalid credentials")
			return
		}
		// Source must match kind: session tokens only authenticate
		// over cookies, api tokens only over Authorization: Bearer.
		// Prevents a leaked session cookie from being replayed as a
		// long-lived Bearer credential.
		if !sourceMatchesKind(source, resolved.Kind) {
			cfg.Unauthorized(c, "missing or invalid credentials")
			return
		}
		if err := cfg.Service.Touch(c.Request.Context(), resolved, cfg.Now()); err != nil {
			cfg.Logger.Warn("auth touch failed", zap.Error(err))
			// Don't fail the request — touching is best-effort.
		}
		ctx := context.WithValue(c.Request.Context(), ctxUserKey, resolved.User)
		ctx = context.WithValue(ctx, ctxResolvedKey, resolved)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// tokenSource records where a credential came from on the wire so
// downstream code can enforce kind/source pairing.
type tokenSource int

const (
	tokenSourceNone tokenSource = iota
	tokenSourceCookie
	tokenSourceBearer
)

// extractToken pulls the raw token out of the cookie or Authorization
// header in that order, per ADR-0001. The returned source records
// which channel the token came in on.
func extractToken(c *gin.Context) (string, tokenSource, bool) {
	if cookie, err := c.Cookie(constants.SessionCookieName); err == nil && cookie != "" {
		return cookie, tokenSourceCookie, true
	}
	authz := c.GetHeader("Authorization")
	if authz == "" {
		return "", tokenSourceNone, false
	}
	const prefix = "Bearer "
	if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
		return "", tokenSourceNone, false
	}
	tok := strings.TrimSpace(authz[len(prefix):])
	if tok == "" {
		return "", tokenSourceNone, false
	}
	return tok, tokenSourceBearer, true
}

// sourceMatchesKind enforces token-kind / channel separation:
// session tokens may only authenticate over cookies, API tokens only
// over Authorization: Bearer.
func sourceMatchesKind(src tokenSource, kind string) bool {
	switch kind {
	case constants.TokenKindSession:
		return src == tokenSourceCookie
	case constants.TokenKindAPI:
		return src == tokenSourceBearer
	default:
		return false
	}
}
