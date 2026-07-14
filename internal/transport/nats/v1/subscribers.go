package v1

import (
	"context"

	"gitlab.com/skyrix-lib/eventbus"
)

// SubscriberGroup is the lifecycle placeholder for future NATS subscribers.
type SubscriberGroup struct {
	bus eventbus.Bus
}

// NewSubscriberGroup keeps eventbus.Bus in the application dependency graph.
func NewSubscriberGroup(bus eventbus.Bus) *SubscriberGroup {
	return &SubscriberGroup{bus: bus}
}

// Start is a no-op until concrete subscribers are added.
func (g *SubscriberGroup) Start(context.Context) error {
	return nil
}

// Stop is a no-op until concrete subscribers are added.
func (g *SubscriberGroup) Stop(context.Context) error {
	return nil
}
