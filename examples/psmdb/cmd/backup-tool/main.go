package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// We must notify our context for SIGINT and SIGTERM signals.
	// This is required so that the backup tool can shutdown gracefully
	// and clean up any resources it has created.
	// Note that this works only because the Job starts the importer process as PID 1.
	pCtx := context.Background()

	ctx, stop := signal.NotifyContext(pCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Execute the root command with context
	if err := Root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
