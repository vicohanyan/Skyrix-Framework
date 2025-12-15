package main

import (
	"context"
	"log" // Keep standard log for build errors if custom logger isn't ready
	"os/signal"
	"syscall"
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

	// Use your custom logger for application runtime messages
	app.Kernel.Logger.Info("HTTP application built successfully", "address", app.Server.Addr)

	// 4. Run the application
	if err := app.Run(ctx, app.Kernel.Logger); err != nil {
		// The app's logger is available now
		app.Kernel.Logger.Error("HTTP app run failed", "error", err)
	}
}
