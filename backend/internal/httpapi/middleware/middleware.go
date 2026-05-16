// Package middleware contains the gin middlewares NextChapter wires
// into every request: request id, structured logging, CORS, and the
// auth gate. The auth middleware lives here (rather than under
// [internal/auth]) so the auth package stays domain code; this
// package is HTTP plumbing.
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// HeaderRequestID is the HTTP header we both accept (if the caller
// supplied one) and echo back to the client. Matches the convention
// used by most proxies.
const HeaderRequestID = "X-Request-Id"

type ctxKey int

const (
	ctxRequestIDKey ctxKey = iota + 1
)

// RequestID assigns an opaque request id to every request, stores it on
// the gin context, the request context, and the response header. Logs
// downstream pick it up via [RequestIDFromContext].
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Writer.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(c.Request.Context(), ctxRequestIDKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequestIDFromContext returns the request id attached by [RequestID].
// The empty-string return covers two contracted cases:
//   - the request did not pass through [RequestID] (e.g. a deeply nested
//     internal call with a background context);
//   - the value at the key isn't a string (cannot happen given the
//     unexported key — kept for type-safety).
//
// Both branches collapse to "" because callers (the access-log
// middleware, downstream log lines) skip the field when it's empty.
// The bool from the type assertion is therefore discarded by design.
func RequestIDFromContext(ctx context.Context) string {
	v, ok := ctx.Value(ctxRequestIDKey).(string)
	if !ok {
		return ""
	}
	return v
}

// Logger is a structured access-log middleware that emits one zap line
// per request after the handler returns.
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}
		c.Next()
		latency := time.Since(start)

		fields := make([]zap.Field, 0, 6)
		fields = append(fields,
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		)
		if rid := RequestIDFromContext(c.Request.Context()); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		}
		level := zapcore.InfoLevel
		switch {
		case c.Writer.Status() >= 500:
			level = zapcore.ErrorLevel
		case c.Writer.Status() >= 400:
			level = zapcore.WarnLevel
		}
		if ce := log.Check(level, "http request"); ce != nil {
			ce.Write(fields...)
		}
	}
}

// CORSConfig configures the CORS middleware. An empty AllowedOrigins
// disables CORS entirely (same-origin only).
type CORSConfig struct {
	AllowedOrigins []string
}

// CORS returns a gin middleware that responds to preflights and adds
// Access-Control-Allow-* headers for whitelisted origins. It is
// intentionally minimal; the bootstrap milestone has no third-party
// callers besides the extension and the dev SPA.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[strings.TrimRight(o, "/")] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			// Not allow-listed: do not emit CORS headers; the
			// browser will fail the request. We still serve the
			// request so curl/extension callers without an Origin
			// header are unaffected.
			c.Next()
			return
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id")
			h.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
