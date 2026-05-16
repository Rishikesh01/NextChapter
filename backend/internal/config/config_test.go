package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/enable-it/nextchapter/backend/internal/config"
)

func TestFromEnvDefaults(t *testing.T) {
	r := require.New(t)
	t.Setenv("NEXTCHAPTER_LISTEN_ADDR", "")
	t.Setenv("NEXTCHAPTER_DATABASE_URL", "")
	t.Setenv("NEXTCHAPTER_BOOTSTRAP_USERNAME", "")
	t.Setenv("NEXTCHAPTER_BOOTSTRAP_PASSWORD", "")
	t.Setenv("NEXTCHAPTER_LOG_LEVEL", "")
	t.Setenv("NEXTCHAPTER_ALLOWED_ORIGINS", "")

	cfg, err := config.FromEnv()
	r.NoError(err)
	r.Equal(":8080", cfg.ListenAddr)
	r.Equal("sqlite://./nextchapter.db", cfg.DatabaseURL)
	r.Equal(zapcore.InfoLevel, cfg.LogLevel)
	r.False(cfg.HasBootstrap())
}

func TestFromEnvBootstrapPair(t *testing.T) {
	r := require.New(t)
	t.Setenv("NEXTCHAPTER_BOOTSTRAP_USERNAME", "alice")
	t.Setenv("NEXTCHAPTER_BOOTSTRAP_PASSWORD", "shortpw")

	_, err := config.FromEnv()
	r.Error(err, "expected error for short password")

	t.Setenv("NEXTCHAPTER_BOOTSTRAP_PASSWORD", "longenoughpassword")
	cfg, err := config.FromEnv()
	r.NoError(err)
	r.True(cfg.HasBootstrap())

	t.Setenv("NEXTCHAPTER_BOOTSTRAP_PASSWORD", "")
	_, err = config.FromEnv()
	r.Error(err, "expected error when only username set")
}

func TestFromEnvLogLevel(t *testing.T) {
	r := require.New(t)
	t.Setenv("NEXTCHAPTER_LOG_LEVEL", "debug")
	cfg, err := config.FromEnv()
	r.NoError(err)
	r.Equal(zapcore.DebugLevel, cfg.LogLevel)

	t.Setenv("NEXTCHAPTER_LOG_LEVEL", "bogus")
	_, err = config.FromEnv()
	r.Error(err, "expected error for invalid log level")
}

func TestFromEnvAllowedOrigins(t *testing.T) {
	r := require.New(t)
	t.Setenv("NEXTCHAPTER_ALLOWED_ORIGINS", "http://localhost:5173, http://example.com")
	cfg, err := config.FromEnv()
	r.NoError(err)
	r.Equal([]string{"http://localhost:5173", "http://example.com"}, cfg.AllowedOrigins)
}
