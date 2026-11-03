package main

import (
	"os/exec"
	"testing"
)

func TestHelp(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "-h")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
}
