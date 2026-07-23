package v1

import "github.com/google/wire"

// ProviderSet keeps the NATS transport lifecycle wired while no concrete
// consumers or publishers are registered.
var ProviderSet = wire.NewSet(
	NewSubscriberGroup,
)
