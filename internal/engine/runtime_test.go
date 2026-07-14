package engine

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeStartRollsBackStartedComponents(t *testing.T) {
	startErr := errors.New("start failed")

	first := &runtimeComponent{name: "first"}
	second := &runtimeComponent{name: "second", startErr: startErr}
	third := &runtimeComponent{name: "third"}

	runtime := NewRuntime(first, second, third)

	if err := runtime.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want error")
	}

	if first.starts != 1 {
		t.Fatalf("first starts = %d, want 1", first.starts)
	}
	if first.stops != 1 {
		t.Fatalf("first stops = %d, want 1", first.stops)
	}
	if second.stops != 0 {
		t.Fatalf("second stops = %d, want 0", second.stops)
	}
	if third.starts != 0 {
		t.Fatalf("third starts = %d, want 0", third.starts)
	}
}

func TestRuntimeStopAttemptsAllComponents(t *testing.T) {
	firstErr := errors.New("first stop failed")
	secondErr := errors.New("second stop failed")

	first := &runtimeComponent{name: "first", stopErr: firstErr}
	second := &runtimeComponent{name: "second", stopErr: secondErr}

	runtime := NewRuntime(first, second)

	err := runtime.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop() error = nil, want error")
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("Stop() error does not contain first stop error: %v", err)
	}
	if !errors.Is(err, secondErr) {
		t.Fatalf("Stop() error does not contain second stop error: %v", err)
	}
	if first.stops != 1 {
		t.Fatalf("first stops = %d, want 1", first.stops)
	}
	if second.stops != 1 {
		t.Fatalf("second stops = %d, want 1", second.stops)
	}
}

type runtimeComponent struct {
	name     string
	startErr error
	stopErr  error
	starts   int
	stops    int
}

func (c *runtimeComponent) Start(context.Context) error {
	c.starts++
	return c.startErr
}

func (c *runtimeComponent) Stop(context.Context) error {
	c.stops++
	return c.stopErr
}
