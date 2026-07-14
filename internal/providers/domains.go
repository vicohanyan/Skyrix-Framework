package providers

import (
	"skyrix/internal/domain/example"

	"github.com/google/wire"
)

var DomainProviderSet = wire.NewSet(
	example.ProviderSet,
)

var ConsoleDomainProviderSet = wire.NewSet(
	example.ProviderSet,
)
