package main

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteStandaloneEventBusHelp(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder

	handled, err := executeStandaloneCommand(
		context.Background(),
		[]string{"eventbus", "consumer", "create", "--help"},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("execute standalone EventBus help: %v", err)
	}
	if !handled {
		t.Fatal("EventBus command was not handled standalone")
	}
	if !strings.Contains(stdout.String(), "does not subscribe the consumer") {
		t.Fatalf("unexpected EventBus help:\n%s", stdout.String())
	}
}

func TestExecuteStandaloneIgnoresApplicationCommand(t *testing.T) {
	handled, err := executeStandaloneCommand(
		context.Background(),
		[]string{"jobs", "run", "example"},
		&strings.Builder{},
		&strings.Builder{},
	)
	if err != nil {
		t.Fatalf("execute application command: %v", err)
	}
	if handled {
		t.Fatal("application command was handled as standalone")
	}
}
