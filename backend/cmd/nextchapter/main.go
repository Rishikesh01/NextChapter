// Package main is the entry point for the nextchapter HTTP server. It
// resolves config from the environment, installs a SIGINT/SIGTERM
// signal handler, and delegates to internal/server.Run.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/enable-it/nextchapter/backend/internal/config"
	"github.com/enable-it/nextchapter/backend/internal/server"
)

// @title           NextChapter API
// @version         1.0
// @description     NextChapter is a self-hosted progress tracker for manhwa, manhua, and web novels.
// @description     Open registration — anyone can create an account via /auth/register.
// @description
// @description     Auth: session cookie ("nc_session") OR Authorization: Bearer <api token>.
//
// @contact.name    NextChapter
//
// @license.name    See LICENSE
//
// @BasePath        /
// @schemes         http https
//
// @securityDefinitions.apikey  CookieAuth
// @in                          cookie
// @name                        nc_session
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run is split out from main so deferred cleanup (signal.NotifyContext's
// stop, etc.) actually executes before os.Exit.
func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}
