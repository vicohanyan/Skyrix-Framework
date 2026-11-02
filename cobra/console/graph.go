package console

import (
	"errors"
	"fmt"
	"sync"
)

// Needs is what a command needs from the application when it runs.
type Needs uint8

const (
	// NeedsNothing marks standalone commands: they need no dependency
	// graph and read their configuration and open their connections
	// themselves when they run (make, db migrate*, platform module commands).
	NeedsNothing Needs = iota
	// NeedsDatabase marks commands that need only the database graph
	// (configuration, logger, PostgreSQL), such as db automigrate and db seed.
	NeedsDatabase
	// NeedsApp marks commands that need the full application graph.
	NeedsApp
)

// String returns "standalone", "database" or "application".
func (n Needs) String() string {
	switch n {
	case NeedsNothing:
		return "standalone"
	case NeedsDatabase:
		return "database"
	case NeedsApp:
		return "application"
	default:
		return fmt.Sprintf("Needs(%d)", uint8(n))
	}
}

// Graph is a dependency graph that is built on first use. Its build function
// is typically a Wire injector (func() (T, func(), error)): the graph stays
// compile-time, only the call is deferred until a command that needs it runs.
// Help, completion and commands that do not use the graph never build it.
//
// Get builds the graph once and returns the same value (or build error) on
// every call; Close runs the injector's cleanup. A Graph is safe for
// concurrent use.
type Graph[T any] struct {
	needs Needs
	build func() (T, func(), error)

	mu      sync.Mutex
	built   bool
	value   T
	cleanup func()
	err     error
}

// NewGraph returns a graph of what needs describes (NeedsDatabase or
// NeedsApp), built by build when first used.
func NewGraph[T any](needs Needs, build func() (T, func(), error)) *Graph[T] {
	return &Graph[T]{needs: needs, build: build}
}

// Value returns an already built graph holding value, for tests and for
// dependencies that cost nothing to create. Close does nothing.
func Value[T any](needs Needs, value T) *Graph[T] {
	return &Graph[T]{needs: needs, built: true, value: value}
}

// Derive returns a graph of a part of parent, such as one field of the
// application graph. It shares parent's build: getting it builds parent
// once, and closing parent releases it. Closing the derived graph does
// nothing.
func Derive[T, U any](parent *Graph[T], part func(T) U) *Graph[U] {
	needs := NeedsApp
	if parent != nil {
		needs = parent.needs
	}
	return &Graph[U]{needs: needs, build: func() (U, func(), error) {
		var zero U
		value, err := parent.Get()
		if err != nil {
			return zero, nil, err
		}
		return part(value), nil, nil
	}}
}

// Needs reports what the graph provides.
func (g *Graph[T]) Needs() Needs {
	if g == nil {
		return NeedsNothing
	}
	return g.needs
}

// Get builds the graph on the first call and returns its value. A failed
// build is not retried: later calls return the same error.
func (g *Graph[T]) Get() (T, error) {
	var zero T
	if g == nil || (g.build == nil && !g.built) {
		return zero, errors.New("console: dependency graph is not configured")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.built {
		g.built = true
		g.value, g.cleanup, g.err = g.build()
		if g.err != nil {
			g.value = zero
			if g.cleanup != nil {
				g.cleanup()
				g.cleanup = nil
			}
		}
	}
	return g.value, g.err
}

// Built reports whether the graph has been built (or its build attempted).
func (g *Graph[T]) Built() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.built
}

// Close releases a built graph by running the injector's cleanup once. It
// does nothing when the graph was never built.
func (g *Graph[T]) Close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	cleanup := g.cleanup
	g.cleanup = nil
	g.mu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}
