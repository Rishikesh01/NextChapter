// Package config parses NextChapter's runtime configuration from the
// environment. It is intentionally trivial: no flags, no config files, no
// hot-reload. A single Config struct is built at startup and passed by value
// down the wiring.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap/zapcore"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	// ListenAddr is the address the HTTP server binds to (host:port).
	ListenAddr string
	// DatabaseURL points at the persistence layer. Supported schemes:
	// "sqlite://" (modernc.org/sqlite) and "postgres://" (pgx/v5).
	DatabaseURL string
	// BootstrapUsername / BootstrapPassword create the first user on a
	// fresh DB. Both must be set or both must be empty.
	BootstrapUsername string
	BootstrapPassword string
	// LogLevel is one of "debug", "info", "warn", "error".
	LogLevel zapcore.Level
	// AllowedOrigins is the CORS allow-list. Empty = same-origin only.
	AllowedOrigins []string
	// Version is the build version; surfaced from /healthz.
	Version string
}

// Default returns the zero-value defaults applied before env vars are read.
func Default() Config {
	return Config{
		ListenAddr:  ":8080",
		DatabaseURL: "sqlite://./nextchapter.db",
		LogLevel:    zapcore.InfoLevel,
		Version:     "dev",
	}
}

// FromEnv reads NEXTCHAPTER_* variables, layered on top of Default().
func FromEnv() (Config, error) {
	cfg := Default()

	if v := os.Getenv("NEXTCHAPTER_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("NEXTCHAPTER_DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	cfg.BootstrapUsername = os.Getenv("NEXTCHAPTER_BOOTSTRAP_USERNAME")
	cfg.BootstrapPassword = os.Getenv("NEXTCHAPTER_BOOTSTRAP_PASSWORD")
	if v := os.Getenv("NEXTCHAPTER_VERSION"); v != "" {
		cfg.Version = v
	}

	if v := os.Getenv("NEXTCHAPTER_LOG_LEVEL"); v != "" {
		lvl, err := parseLevel(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		cfg.LogLevel = lvl
	}

	if v := os.Getenv("NEXTCHAPTER_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, p)
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate enforces the cross-field invariants the type system can't catch.
func (c Config) Validate() error {
	if c.ListenAddr == "" {
		return errors.New("config: NEXTCHAPTER_LISTEN_ADDR must not be empty")
	}
	if c.DatabaseURL == "" {
		return errors.New("config: NEXTCHAPTER_DATABASE_URL must not be empty")
	}
	switch {
	case c.BootstrapUsername == "" && c.BootstrapPassword == "":
		// fine: open registration is always available.
	case c.BootstrapUsername != "" && c.BootstrapPassword != "":
		// fine: env-var bootstrap.
		if len(c.BootstrapPassword) < 8 {
			return errors.New("config: NEXTCHAPTER_BOOTSTRAP_PASSWORD must be at least 8 characters")
		}
	default:
		return errors.New("config: NEXTCHAPTER_BOOTSTRAP_USERNAME and NEXTCHAPTER_BOOTSTRAP_PASSWORD must both be set or both be empty")
	}
	return nil
}

// HasBootstrap reports whether env-var bootstrap is configured.
func (c Config) HasBootstrap() bool {
	return c.BootstrapUsername != "" && c.BootstrapPassword != ""
}

func parseLevel(s string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info", "":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid NEXTCHAPTER_LOG_LEVEL %q", s)
	}
}
