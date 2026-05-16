package middleware

import (
	"context"
	"errors"
	"net/http"
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
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Service == nil {
		panic("middleware: AuthMiddlewareConfig.Service must not be nil")
	}
	return func(c *gin.Context) {
		raw, source, ok := extractAuthToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, handlers.ErrorBody{Error: handlers.ErrorDetail{
				Code:    handlers.CodeUnauthorized,
				Message: "missing or invalid credentials",
			}})
			return
		}
		resolved, err := cfg.Service.Resolve(c.Request.Context(), raw, time.Now())
		if err != nil {
			if !errors.Is(err, auth.ErrTokenNotFound) {
				cfg.Logger.Warn("auth lookup failed", zap.Error(err))
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, handlers.ErrorBody{Error: handlers.ErrorDetail{
				Code:    handlers.CodeUnauthorized,
				Message: "missing or invalid credentials",
			}})
			return
		}
		// Source must match kind: session tokens only authenticate
		// over cookies, api tokens only over Authorization: Bearer.
		// Prevents a leaked session cookie from being replayed as a
		// long-lived Bearer credential.
		if !authSourceMatchesKind(source, resolved.Kind) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, handlers.ErrorBody{Error: handlers.ErrorDetail{
				Code:    handlers.CodeUnauthorized,
				Message: "missing or invalid credentials",
			}})
			return
		}
		if err := cfg.Service.Touch(c.Request.Context(), resolved, time.Now()); err != nil {
			cfg.Logger.Warn("auth touch failed", zap.Error(err))
			// Don't fail the request — touching is best-effort.
		}
		ctx := models.WithUser(c.Request.Context(), resolved.User)
		ctx = context.WithValue(ctx, ctxAuthResolvedKey, resolved)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ctxAuthResolvedKey is the unexported context-key used to stash the
// auth lookup metadata. Callers that need the token id/kind use
// [AuthResolvedFromContext]; the authenticated user is read via
// [models.UserFromContext].
var ctxAuthResolvedKey = &struct{ name string }{name: "auth.resolved"}

// AuthResolvedFromContext returns the auth lookup metadata stashed by
// the Auth middleware. Currently unused by handlers (the authenticated
// user is read via models.UserFromContext); kept available for future
// flows that need to know the token id/kind.
func AuthResolvedFromContext(ctx context.Context) (auth.Resolved, bool) {
	r, ok := ctx.Value(ctxAuthResolvedKey).(auth.Resolved)
	return r, ok
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
// Authorization header in that order, per ADR-0001. The returned
// source records which channel the token came in on.
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
