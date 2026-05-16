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
	"github.com/enable-it/nextchapter/backend/internal/store"
	gen "github.com/enable-it/nextchapter/backend/internal/store/generated"
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
		// Best-effort flush. zap returns EINVAL on stdout on some
		// platforms — ignore those.
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

	queries := gen.New(db)
	now := time.Now

	// Each domain package follows the same pattern: build the
	// repository (the only thing that touches sqlc-generated code),
	// then build the service on top of it.
	userRepo := users.NewRepository(queries)
	userSvc := users.NewService(userRepo, now)

	authRepo := auth.NewRepository(queries)
	authSvc := auth.NewService(authRepo, now)

	entryRepo := entries.NewRepository(queries)
	entrySvc := entries.NewService(entryRepo, now)

	seriesRepo := series.NewRepository(queries)
	seriesSvc := series.NewService(seriesRepo, entrySvc, now)

	if cfg.HasBootstrap() {
		if err := bootstrapFirstUser(ctx, logger, userSvc, cfg.BootstrapUsername, cfg.BootstrapPassword); err != nil {
			return fmt.Errorf("server: bootstrap: %w", err)
		}
	}

	engine := httpapi.New(httpapi.Deps{
		Users:          userSvc,
		Auth:           authSvc,
		Series:         seriesSvc,
		Entries:        entrySvc,
		Logger:         logger,
		HasEnvBoot:     cfg.HasBootstrap(),
		Version:        cfg.Version,
		AllowedOrigins: cfg.AllowedOrigins,
		CookieSecure:   isHTTPS(cfg.AllowedOrigins),
		CookieDomain:   "",
		Now:            now,
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

func bootstrapFirstUser(ctx context.Context, logger *zap.Logger, svc *users.Service, username, password string) error {
	n, err := svc.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		logger.Debug("bootstrap: users already exist, skipping")
		return nil
	}
	if _, err := svc.Create(ctx, models.Registration{Username: username, Password: password}); err != nil {
		return err
	}
	logger.Warn("bootstrap: created first user from env vars", zap.String("username", username))
	return nil
}

func newLogger(level zapcore.Level) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
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
