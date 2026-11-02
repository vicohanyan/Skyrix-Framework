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

func TestExecuteStandaloneMigrationHelpNeedsNoDatabase(t *testing.T) {
	t.Setenv("APP_ENV", "missing-environment")
	t.Setenv("DB_HOST", "203.0.113.1")

	for _, args := range [][]string{
		{"db", "migrate", "--help"},
		{"db", "migrate:status", "--help"},
		{"db", "migrate:baseline", "--help"},
		{"db", "migrate:diff", "--help"},
		{"make", "migration", "--help"},
	} {
		var stdout strings.Builder
		handled, err := executeStandaloneCommand(context.Background(), args, &stdout, &strings.Builder{})
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !handled {
			t.Fatalf("%v was not handled standalone", args)
		}
		if !strings.Contains(stdout.String(), "Usage:") {
			t.Fatalf("%v help output:\n%s", args, stdout.String())
		}
	}
}

func TestExecuteStandaloneLeavesContainerCommandsToApplication(t *testing.T) {
	for _, args := range [][]string{
		{"db"},
		{"db", "automigrate"},
		{"db", "seed"},
		{"make"},
		{"--help"},
	} {
		handled, err := executeStandaloneCommand(context.Background(), args, &strings.Builder{}, &strings.Builder{})
		if err != nil || handled {
			t.Fatalf("%v: handled = %v, err = %v; want application command", args, handled, err)
		}
	}
}
