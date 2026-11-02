package kernel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// failingComponent signals Start through started and fails on demand.
type failingComponent struct {
	started  chan struct{}
	failures chan error
	mu       sync.Mutex
	stops    int
}

func newFailingComponent() *failingComponent {
	return &failingComponent{started: make(chan struct{}), failures: make(chan error, 1)}
}

func (c *failingComponent) Start(context.Context) error { close(c.started); return nil }
func (c *failingComponent) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stops++
	return nil
}
func (c *failingComponent) Failed() <-chan error { return c.failures }

func TestRunStopsInReverseOrderWhenContextIsCancelled(t *testing.T) {
	log := &orderLog{}
	first := &orderedComponent{name: "first", log: log}
	second := &orderedComponent{name: "second", log: log}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- Run(ctx, NewRuntime(first, second)) }()
	waitFor(t, func() bool { return len(log.entries()) == 2 })
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	order := log.entries()
	want := []string{"start first", "start second", "stop second", "stop first"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestRunStopsAndReturnsComponentFailure(t *testing.T) {
	failure := errors.New("listener closed")
	healthy := newFailingComponent()
	failing := newFailingComponent()

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), NewRuntime(healthy, failing)) }()
	<-failing.started
	failing.failures <- failure

	select {
	case err := <-done:
		if !errors.Is(err, failure) {
			t.Fatalf("Run() error = %v, want component failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after a component failure")
	}
	if healthy.stops != 1 || failing.stops != 1 {
		t.Fatalf("stops = %d/%d, want 1/1", healthy.stops, failing.stops)
	}
}

func TestRunReturnsStartError(t *testing.T) {
	startErr := errors.New("port in use")
	err := Run(context.Background(), NewRuntime(&runtimeComponent{startErr: startErr}))
	if !errors.Is(err, startErr) {
		t.Fatalf("Run() error = %v, want start error", err)
	}
}

func TestRunBoundsStopWithTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, NewRuntime(&blockingStopComponent{}), WithStopTimeout(20*time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want stop deadline", err)
	}
}

type orderLog struct {
	mu    sync.Mutex
	items []string
}

func (l *orderLog) add(item string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = append(l.items, item)
}

func (l *orderLog) entries() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.items...)
}

type orderedComponent struct {
	name string
	log  *orderLog
}

func (c *orderedComponent) Start(context.Context) error {
	c.log.add("start " + c.name)
	return nil
}

func (c *orderedComponent) Stop(context.Context) error {
	c.log.add("stop " + c.name)
	return nil
}

type blockingStopComponent struct{}

func (blockingStopComponent) Start(context.Context) error { return nil }
func (blockingStopComponent) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}
