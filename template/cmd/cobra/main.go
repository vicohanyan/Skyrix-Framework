package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "console command failed: %v\n", err)
		os.Exit(1)
	}
}

// run executes one console command. Help and completion are answered from
// the command descriptions; a command builds only the graph it needs, right
// before it runs (see newConsole).
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	g := newGraphs()
	defer g.Close()
	return newConsole(g).Execute(ctx, args, stdout, stderr)
}
