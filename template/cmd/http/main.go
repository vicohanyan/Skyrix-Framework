package main

import (
	"context"
	"log" // Keep standard log for build errors if custom logger isn't ready
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/skyrix-lib/framework/kernel"
)

func main() {
	// 1. Create a context that is canceled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. Build the application using wire
	app, cleanup, err := buildHTTPApp()
	if err != nil {
		// Use a standard logger here because the app's logger might not be initialized yet
		log.Fatalf("Failed to build http app: %v", err)
	}

	// 3. Defer the cleanup function that wire provides
	defer cleanup()

	// 4. Run until a signal arrives or a component fails
	if err := kernel.Run(ctx, app.Runtime); err != nil {
		app.Logger.Error("HTTP app run failed", "error", err)
		cleanup()
		os.Exit(1)
	}
	app.Logger.Info("HTTP application stopped")
}
