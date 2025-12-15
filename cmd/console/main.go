package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	consoleApp, cleanup, err := buildConsoleApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build console app: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := consoleApp.Execute(ctx); err != nil {
		consoleApp.Kernel.Logger.Error("console command failed", "error", err)
		os.Exit(1)
	}
}
