package middleware

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/handlers"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// AuthMiddlewareConfig wires the auth middleware. All fields are required.
type AuthMiddlewareConfig struct {
	Service *auth.Service
	Logger  *zap.Logger
}

// Auth returns a gin handler that resolves a session cookie or bearer
// token to a user and attaches it to the request context. Routes
// behind this middleware can assume [models.UserFromContext] returns
// ok=true. Lives in the middleware package (rather than internal/auth)
// so the auth package stays domain-only: this file is HTTP plumbing
// that happens to consume the auth service, not part of the auth
// domain itself.
func Auth(cfg AuthMiddlewareConfig) gin.HandlerFunc {
	if cfg.Service == nil {
		panic("middleware: AuthMiddlewareConfig.Service must not be nil")
	}
	return func(c *gin.Context) {
		raw, source, ok := extractAuthToken(c)
		if !ok {
			handlers.WriteUnauthorized(c, "missing or invalid credentials")
			return
		}
		resolved, err := cfg.Service.Resolve(c.Request.Context(), raw, time.Now())
		if err != nil {
			if !errors.Is(err, auth.ErrTokenNotFound) {
				cfg.Logger.Warn("auth lookup failed", zap.Error(err))
			}
			handlers.WriteUnauthorized(c, "missing or invalid credentials")
			return
		}
		// Source must match kind: session tokens only authenticate
		// over cookies, api tokens only over Authorization: Bearer.
		// Prevents a leaked session cookie from being replayed as a
		// long-lived Bearer credential.
		if !authSourceMatchesKind(source, resolved.Kind) {
			handlers.WriteUnauthorized(c, "missing or invalid credentials")
			return
		}
		ctx := models.WithUser(c.Request.Context(), resolved.User)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// authTokenSource records where a credential came from on the wire so
// downstream code can enforce kind/source pairing.
type authTokenSource int

const (
	authTokenSourceNone authTokenSource = iota
	authTokenSourceCookie
	authTokenSourceBearer
)

// extractAuthToken pulls the raw token out of the cookie or
// Authorization header in that order. The returned source records
// which channel the token came in on.
func extractAuthToken(c *gin.Context) (string, authTokenSource, bool) {
	if cookie, err := c.Cookie(constants.SessionCookieName); err == nil && cookie != "" {
		return cookie, authTokenSourceCookie, true
	}
	authz := c.GetHeader("Authorization")
	if authz == "" {
		return "", authTokenSourceNone, false
	}
	const prefix = "Bearer "
	if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
		return "", authTokenSourceNone, false
	}
	tok := strings.TrimSpace(authz[len(prefix):])
	if tok == "" {
		return "", authTokenSourceNone, false
	}
	return tok, authTokenSourceBearer, true
}

// authSourceMatchesKind enforces token-kind / channel separation:
// session tokens may only authenticate over cookies, API tokens only
// over Authorization: Bearer.
func authSourceMatchesKind(src authTokenSource, kind string) bool {
	switch kind {
	case constants.TokenKindSession:
		return src == authTokenSourceCookie
	case constants.TokenKindAPI:
		return src == authTokenSourceBearer
	default:
		return false
	}
}
