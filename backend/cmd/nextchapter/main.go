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

func main() {
	os.Exit(run())
}

// run is split out from main so deferred cleanup (signal.NotifyContext's
// stop, etc.) actually executes before os.Exit. Returns the process
// exit code.
func run() int {
	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		return 1
	}
	return 0
}
