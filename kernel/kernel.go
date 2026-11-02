// Package kernel is the microkernel of the framework: transport- and
// implementation-neutral contracts and the assembly of a binary (runtime,
// module registry, run loop). It depends only on the standard library.
package kernel

import "context"

// Component is a lifecycle unit managed by the runtime. Start must not block:
// long-running work (serving, consuming) runs in the background until Stop.
type Component interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// Kernel is the lifecycle of a whole binary, implemented by Runtime.
type Kernel interface {
	Component
}

// FailureNotifier is implemented by components whose background work can fail
// after Start, such as servers and consumers. The channel receives the
// failure; Run then stops the binary.
type FailureNotifier interface {
	Failed() <-chan error
}

// HealthChecker is implemented by components that can report whether they
// are usable, such as database connections. Check returns nil when healthy
// and must respect ctx.
type HealthChecker interface {
	Check(context.Context) error
}
