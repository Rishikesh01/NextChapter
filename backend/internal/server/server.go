// Package server wires every other package together. It is the only
// place that knows the order of operations: open DB, run migrations,
// run env-var bootstrap, build the http engine, listen.
//
// cmd/nextchapter/main.go is a thin shell around Run.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/config"
	"github.com/enable-it/nextchapter/backend/internal/entries"
	"github.com/enable-it/nextchapter/backend/internal/httpapi"
	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/series"
	"github.com/enable-it/nextchapter/backend/internal/sites"
	"github.com/enable-it/nextchapter/backend/internal/store"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// Run starts NextChapter. It blocks until ctx is canceled or the HTTP
// server stops with an error, then performs a graceful shutdown.
func Run(ctx context.Context, cfg config.Config) error {
	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("server: build logger: %w", err)
	}
	defer func() {
		// zap.Logger.Sync() returns EBADF on stderr in containers (and
		// EINVAL on some stdout configurations); the underlying file
		// descriptor isn't seekable so the flush call surfaces a
		// benign error every time. We intentionally discard it per
		// upstream zap guidance — there's nothing actionable to do
		// with a flush-on-shutdown error and surfacing it would just
		// add noise.
		_ = logger.Sync()
	}()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("server: open db: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("close db", zap.Error(err))
		}
	}()

	if err := store.Migrate(ctx, db, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("server: migrate: %w", err)
	}

	dialect, err := store.DialectFor(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("server: dialect: %w", err)
	}

	// Each domain package follows the same pattern: build the
	// repository (the only thing that touches sqlc-generated code)
	// for the active dialect, then build the service on top of it.
	// The auth service also consumes the users repository so
	// [auth.Service.Authenticate] can read the stored password hash
	// — that's why userRepo is built first and threaded into
	// auth.NewService.
	userRepo, err := users.NewRepository(dialect, db)
	if err != nil {
		return fmt.Errorf("server: users repo: %w", err)
	}
	userSvc := users.NewService(userRepo, logger)

	authRepo, err := auth.NewRepository(dialect, db)
	if err != nil {
		return fmt.Errorf("server: auth repo: %w", err)
	}
	authSvc := auth.NewService(authRepo, userRepo, logger)

	entryRepo, err := entries.NewRepository(dialect, db)
	if err != nil {
		return fmt.Errorf("server: entries repo: %w", err)
	}
	entrySvc := entries.NewService(entryRepo, logger)

	seriesRepo, err := series.NewRepository(dialect, db)
	if err != nil {
		return fmt.Errorf("server: series repo: %w", err)
	}
	seriesSvc := series.NewService(seriesRepo, entrySvc, logger)

	sitesRepo, err := sites.NewRepository(dialect, db)
	if err != nil {
		return fmt.Errorf("server: sites repo: %w", err)
	}
	sitesSvc := sites.NewService(sitesRepo, logger)

	if cfg.HasBootstrap() {
		if err := bootstrapFirstUser(ctx, logger, userSvc, sitesSvc, cfg.BootstrapUsername, cfg.BootstrapPassword); err != nil {
			return fmt.Errorf("server: bootstrap: %w", err)
		}
	}

	engine := httpapi.New(httpapi.Deps{
		Users:          userSvc,
		Auth:           authSvc,
		Series:         seriesSvc,
		Entries:        entrySvc,
		Sites:          sitesSvc,
		Logger:         logger,
		Version:        cfg.Version,
		AllowedOrigins: cfg.AllowedOrigins,
		CookieSecure:   isHTTPS(cfg.AllowedOrigins),
		CookieDomain:   "",
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", zap.String("addr", cfg.ListenAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server: shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func bootstrapFirstUser(ctx context.Context, logger *zap.Logger, svc *users.Service, sitesSvc *sites.Service, username, password string) error {
	u, err := svc.Register(ctx, models.Registration{Username: username, Password: password})
	switch {
	case err == nil:
		logger.Warn("bootstrap: created first user from env vars", zap.String("username", username))
		// Best-effort seed: same shape as the /auth/register handler.
		// Failures don't roll the user back — there is no admin
		// re-seed endpoint today, so a bootstrap-time seed failure
		// leaves the operator with manual rule entry via
		// POST /sites/rules.
		if err := sitesSvc.SeedSiteRulesForUser(ctx, u.ID); err != nil {
			logger.Warn("bootstrap: seed site rules",
				zap.Int64("user_id", u.ID),
				zap.Error(err),
			)
		}
		return nil
	case errors.Is(err, models.ErrUsernameTaken):
		// Re-run with the same env vars after a successful first boot.
		logger.Debug("bootstrap: user already exists, skipping", zap.String("username", username))
		return nil
	default:
		return err
	}
}

func newLogger(level zapcore.Level) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg.Build()
}

// isHTTPS reports whether every configured origin is https://; we use
// that as a proxy for "Secure cookies are appropriate". An operator
// behind a TLS-terminating proxy can override via env if needed in a
// later milestone.
func isHTTPS(origins []string) bool {
	if len(origins) == 0 {
		return false
	}
	for _, o := range origins {
		if !strings.HasPrefix(o, "https://") {
			return false
		}
	}
	return true
}
