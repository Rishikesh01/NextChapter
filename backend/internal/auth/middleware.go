package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/handlers"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// ctxKey is unexported so callers must go through [ResolvedFromContext].
// The authenticated user is exposed via [models.UserFromContext].
type ctxKey int

const (
	ctxResolvedKey ctxKey = iota + 1
)

// ResolvedFromContext returns the auth lookup metadata for callers
// inside this package that need to know the token id / kind. The
// authenticated user is exposed via [models.UserFromContext] to keep
// handlers off the auth-package import.
func ResolvedFromContext(ctx context.Context) (Resolved, bool) {
	r, ok := ctx.Value(ctxResolvedKey).(Resolved)
	return r, ok
}

// MiddlewareConfig wires the middleware. All fields are required.
type MiddlewareConfig struct {
	Service *Service
	Logger  *zap.Logger
}

// Middleware returns a gin handler that resolves a session cookie or
// bearer token to a user and attaches it to the request context.
// Routes behind this middleware can assume [models.UserFromContext]
// returns ok=true.
func Middleware(cfg MiddlewareConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Service == nil {
		panic("auth: MiddlewareConfig.Service must not be nil")
	}
	return func(c *gin.Context) {
		raw, source, ok := extractToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, handlers.ErrorBody{Error: handlers.ErrorDetail{
				Code:    handlers.CodeUnauthorized,
				Message: "missing or invalid credentials",
			}})
			return
		}
		resolved, err := cfg.Service.Resolve(c.Request.Context(), raw, time.Now())
		if err != nil {
			if !errors.Is(err, ErrTokenNotFound) {
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
		if !sourceMatchesKind(source, resolved.Kind) {
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
