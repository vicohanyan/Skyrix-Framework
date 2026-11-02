package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerGraphsHaveIndependentLoggers(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "missing-environment")
	t.Setenv("LOG_OUTPUT", "file")
	dir := t.TempDir()
	t.Setenv("LOG_FILE", filepath.Join(dir, "first.json"))
	t.Setenv("LOG_TYPE", "json")
	t.Setenv("LOG_LEVEL", "debug")
	first, closeFirst, err := buildWorkerApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeFirst)
	t.Setenv("LOG_FILE", filepath.Join(dir, "second.txt"))
	t.Setenv("LOG_TYPE", "text")
	t.Setenv("LOG_LEVEL", "error")
	second, closeSecond, err := buildWorkerApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeSecond)
	if first.Logger == second.Logger {
		t.Fatal("worker graph logger configurations are coupled")
	}
	first.Logger.Debug("first graph")
	closeFirst()
	before, err := os.ReadFile(filepath.Join(dir, "first.json"))
	if err != nil {
		t.Fatal(err)
	}
	first.Logger.Error("closed graph")
	after, err := os.ReadFile(filepath.Join(dir, "first.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("worker graph cleanup did not close logger")
	}
	second.Logger.Info("filtered")
	second.Logger.Error("second graph")
	text, err := os.ReadFile(filepath.Join(dir, "second.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(text, []byte("filtered")) || !bytes.Contains(before, []byte("first graph")) || !bytes.Contains(text, []byte("level=ERROR")) || !bytes.Contains(text, []byte("second graph")) {
		t.Fatalf("second graph output = %s", text)
	}
}
