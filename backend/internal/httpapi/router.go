// Package httpapi builds the gin engine, registers every route, and
// returns an http.Handler the cmd/nextchapter binary can serve. It is
// the only package in the backend that imports gin.
package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/handlers"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/middleware"
	"github.com/enable-it/nextchapter/backend/internal/series"
	"github.com/enable-it/nextchapter/backend/internal/sites"
	_ "github.com/enable-it/nextchapter/backend/internal/swaggerdocs" // registers with gin-swagger
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// Deps is everything the router needs to build itself. The cmd/server
// main wires this struct up once. Service fields are typed against
// the concrete domain packages here because the router is the seam
// where wiring happens; handlers receive them as [models] interfaces.
type Deps struct {
	Users          *users.Service
	Auth           *auth.Service
	Series         *series.Service
	Entries        *entries.Service
	Sites          *sites.Service
	Logger         *zap.Logger
	Version        string
	AllowedOrigins []string
	CookieSecure   bool
	CookieDomain   string
	// WebUI is the embedded SPA build served from / (ADR-0010). Nil
	// (the test harness default) keeps the JSON-only NoRoute behavior.
	WebUI fs.FS
}

// New builds the gin.Engine: sets the run mode, installs the
// middleware stack, registers project-wide custom binding validators,
// and hands off to [registerRoutes] for the path tree. Engine setup
// and routing are deliberately split so the route table is one
// function to read in isolation.
//
// Custom validator registration runs here (not in package init) so a
// test fixture or alternative entry point can spin up an engine with a
// known validator state. A registration failure is fatal — it would
// otherwise leave the engine accepting payloads that should be
// rejected.
func New(d Deps) *gin.Engine {
	if err := handlers.RegisterCustomValidators(); err != nil {
		d.Logger.Fatal("register custom validators", zap.Error(err))
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(d.Logger))
	if len(d.AllowedOrigins) > 0 {
		r.Use(middleware.CORS(middleware.CORSConfig{AllowedOrigins: d.AllowedOrigins}))
	}
	d.registerRoutes(r)
	return r
}

// registerRoutes registers every HTTP path against the engine. Split
// out of [New] so the route table reads top-to-bottom in one place
// without the surrounding engine plumbing.
func (d Deps) registerRoutes(r *gin.Engine) {
	meta := handlers.MetaDeps{Version: d.Version}
	r.GET("/healthz", meta.Health)

	// Swagger UI / spec. Path is hard-coded by gin-swagger; the spec
	// itself is registered via the blank import of internal/swaggerdocs
	// above. Regenerate with `make swagger` after annotation changes.
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authDeps := handlers.AuthDeps{
		Users:        d.Users,
		Auth:         d.Auth,
		Sites:        d.Sites,
		Logger:       d.Logger,
		CookieDomain: d.CookieDomain,
		CookieSecure: d.CookieSecure,
	}
	r.POST("/auth/register", authDeps.Register)
	r.POST("/auth/login", authDeps.Login)

	// Authenticated group.
	authed := r.Group("")
	authed.Use(middleware.Auth(middleware.AuthMiddlewareConfig{
		Service: d.Auth,
		Logger:  d.Logger,
	}))
	authed.POST("/auth/logout", authDeps.Logout)
	authed.GET("/auth/me", authDeps.Me)
	// /auth/tokens: POST mints, DELETE/:id revokes. No list endpoint
	// — the raw token is only ever returned once on mint, so a
	// listing adds attack surface (revealing label/last-used-at
	// metadata) for no product benefit the operator can't get via
	// the DB directly.
	authed.POST("/auth/tokens", authDeps.CreateToken)
	authed.DELETE("/auth/tokens/:id", authDeps.DeleteToken)

	seriesDeps := handlers.SeriesDeps{Series: d.Series, Logger: d.Logger}
	authed.GET("/series", seriesDeps.List)
	authed.POST("/series", seriesDeps.Create)
	authed.GET("/series/:id", seriesDeps.Get)
	authed.PATCH("/series/:id", seriesDeps.Patch)
	authed.DELETE("/series/:id", seriesDeps.Delete)
	// Cover images (ADR-0011). GET returns raw image bytes, not JSON —
	// it is the URL the web UI puts in an <img src>, and same-origin
	// means the session cookie rides along.
	authed.GET("/series/:id/cover", seriesDeps.GetCover)
	authed.PUT("/series/:id/cover", seriesDeps.PutCover)
	authed.DELETE("/series/:id/cover", seriesDeps.DeleteCover)

	entriesDeps := handlers.EntriesDeps{
		Entries:       d.Entries,
		SeriesTracker: d.Series,
		Logger:        d.Logger,
	}
	authed.GET("/entries", entriesDeps.List)
	authed.POST("/entries/capture", entriesDeps.Capture)
	authed.PATCH("/entries/:id", entriesDeps.Patch)
	authed.DELETE("/entries/:id", entriesDeps.Delete)

	sitesDeps := handlers.SitesDeps{
		Sites:   d.Sites,
		Entries: d.Entries,
		Logger:  d.Logger,
	}
	authed.GET("/sites", sitesDeps.List)
	authed.POST("/sites/rules", sitesDeps.AddRule)
	authed.PATCH("/sites/rules/:id", sitesDeps.EditRule)
	authed.DELETE("/sites/rules/:id", sitesDeps.RemoveRule)

	// Unknown-route fallback: the embedded web UI for browser-shaped GET
	// requests, the JSON not_found envelope for everything else. gin's
	// StaticFS cannot mount at "/" (catch-all conflicts with registered
	// routes) and its sub-path variant writes a 404 header before NoRoute
	// runs, so ALL static serving lives here (ADR-0010 §4). NoRoute runs
	// with engine middleware only — unauthenticated, as assets must be.
	r.NoRoute(func(c *gin.Context) {
		method := c.Request.Method
		if d.WebUI == nil ||
			(method != http.MethodGet && method != http.MethodHead) ||
			isAPIPath(c.Request.URL.Path) {
			handlers.WriteNotFound(c, "not found")
			return
		}
		serveWebUI(c, d.WebUI)
	})
}

// apiPrefixes are the first path segments owned by the JSON API. Unknown
// paths under them keep the not_found envelope even with the SPA embedded,
// so API clients never receive HTML for a typo'd endpoint.
var apiPrefixes = map[string]bool{
	"auth":    true,
	"series":  true,
	"entries": true,
	"sites":   true,
	"healthz": true,
	"swagger": true,
}

func isAPIPath(p string) bool {
	seg := strings.TrimPrefix(path.Clean(p), "/")
	if i := strings.IndexByte(seg, '/'); i >= 0 {
		seg = seg[:i]
	}
	return apiPrefixes[seg]
}

// serveWebUI serves the exact embedded file when it exists, else
// index.html with 200 — the client-side-routing fallback. Content-hashed
// Vite assets are immutable; index.html must always revalidate so a new
// deploy takes effect.
func serveWebUI(c *gin.Context, ui fs.FS) {
	name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	if info, err := fs.Stat(ui, name); err != nil || info.IsDir() {
		name = "index.html"
	}
	if strings.HasPrefix(name, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}
	http.ServeFileFS(c.Writer, c.Request, ui, name)
}
