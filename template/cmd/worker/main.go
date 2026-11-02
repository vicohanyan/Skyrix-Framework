package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/skyrix-lib/framework/kernel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, cleanup, err := buildWorkerApp()
	if err != nil {
		log.Fatalf("Failed to build worker app: %v", err)
	}
	defer cleanup()

	app.Logger.Info("worker started")
	if err := kernel.Run(ctx, app.Runtime); err != nil {
		app.Logger.Error("worker run failed", "error", err)
		cleanup()
		os.Exit(1)
	}
	app.Logger.Info("worker stopped")
}
