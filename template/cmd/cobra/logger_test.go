package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/skyrix-lib/template/internal/jobs"
)

func TestApplicationGraphOwnsOneLogger(t *testing.T) {
	setConsoleEnv(t, "127.0.0.1:1", "127.0.0.1:1")
	t.Chdir(t.TempDir())
	path := filepath.Join(t.TempDir(), "console.json")
	t.Setenv("LOG_OUTPUT", "file")
	t.Setenv("LOG_FILE", path)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_TYPE", "json")
	graphs := newGraphs()
	t.Cleanup(graphs.Close)
	app, err := graphs.App.Get()
	if err != nil {
		t.Fatal(err)
	}
	again, err := graphs.App.Get()
	if err != nil || app != again {
		t.Fatal("graph did not retain its application instance")
	}
	job, ok := app.Jobs.Get("system.ping")
	if !ok {
		t.Fatal("system.ping is not registered")
	}
	if job.(*jobs.SystemPingJob).Log != app.Commands.Logger {
		t.Fatal("consumers do not share the graph logger")
	}
	app.Commands.Logger.Debug("before close")
	graphs.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte("before close")) {
		t.Fatalf("missing log: %s", before)
	}
	app.Commands.Logger.Error("after close")
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("graph cleanup did not close logger file")
	}
}
