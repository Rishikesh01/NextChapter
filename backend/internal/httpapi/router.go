// Package httpapi builds the gin engine, registers every route, and
// returns an http.Handler the cmd/nextchapter binary can serve. It is
// the only package in the backend that imports gin.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/api"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/handlers"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/middleware"
	"github.com/enable-it/nextchapter/backend/internal/series"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// Deps is everything the router needs to build itself. The cmd/server
// main wires this struct up once.
type Deps struct {
	Users          *users.Service
	Auth           *auth.Service
	Series         *series.Service
	Entries        *entries.Service
	Logger         *zap.Logger
	HasEnvBoot     bool
	Version        string
	AllowedOrigins []string
	CookieSecure   bool
	CookieDomain   string
	Now            func() time.Time
}

// New builds the gin.Engine. It uses gin.New() (not gin.Default) so we
// own the middleware stack and produce JSON logs via zap.
func New(d Deps) *gin.Engine {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = zap.NewNop()
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(d.Logger))
	if len(d.AllowedOrigins) > 0 {
		r.Use(middleware.CORS(middleware.CORSConfig{AllowedOrigins: d.AllowedOrigins}))
	}

	meta := handlers.MetaDeps{Version: d.Version}
	r.GET("/", meta.Root)
	r.GET("/healthz", meta.Health)

	authDeps := handlers.AuthDeps{
		Users:        d.Users,
		Auth:         d.Auth,
		Logger:       d.Logger,
		HasEnvBoot:   d.HasEnvBoot,
		CookieDomain: d.CookieDomain,
		CookieSecure: d.CookieSecure,
		Now:          d.Now,
	}
	r.POST("/auth/register", authDeps.Register)
	r.POST("/auth/login", authDeps.Login)

	// Authenticated group.
	authed := r.Group("")
	authed.Use(auth.Middleware(auth.MiddlewareConfig{
		Service: d.Auth,
		Logger:  d.Logger,
	}))
	authed.POST("/auth/logout", authDeps.Logout)
	authed.GET("/auth/me", authDeps.Me)
	// /auth/tokens: POST mints, DELETE/:id revokes. No list endpoint —
	// the raw token is only ever returned once on mint, so a listing
	// adds attack surface (revealing label/last-used-at metadata) for
	// no product benefit the operator can't get via the DB directly.
	authed.POST("/auth/tokens", authDeps.CreateToken)
	authed.DELETE("/auth/tokens/:id", authDeps.DeleteToken)

	seriesDeps := handlers.SeriesDeps{Series: d.Series, Logger: d.Logger}
	authed.GET("/series", seriesDeps.List)
	authed.POST("/series", seriesDeps.Create)
	authed.GET("/series/:id", seriesDeps.Get)
	authed.PATCH("/series/:id", seriesDeps.Patch)
	authed.DELETE("/series/:id", seriesDeps.Delete)

	entriesDeps := handlers.EntriesDeps{
		Entries:       d.Entries,
		SeriesCreator: seriesCreatorAdapter{s: d.Series},
		Logger:        d.Logger,
	}
	authed.GET("/entries", entriesDeps.List)
	authed.POST("/entries/capture", entriesDeps.Capture)
	authed.PATCH("/entries/:id", entriesDeps.Patch)
	authed.DELETE("/entries/:id", entriesDeps.Delete)

	// Method-mismatch / unknown-route fallback. We do not advertise the
	// /auth/register endpoint to closed-window callers.
	r.NoRoute(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeNotFound,
			Message: "not found",
		}})
	})

	return r
}

// seriesCreatorAdapter adapts *series.Service to entries.SeriesCreator.
type seriesCreatorAdapter struct{ s *series.Service }

func (a seriesCreatorAdapter) Create(ctx context.Context, userID int64, title string) (int64, error) {
	return a.s.CreateImplicit(ctx, userID, title)
}
