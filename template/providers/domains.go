package providers

import (
	"gitlab.com/skyrix-lib/template/internal/domain/example"

	"github.com/goforj/wire"
)

var DomainProviderSet = wire.NewSet(
	example.ProviderSet,
)

var ConsoleDomainProviderSet = wire.NewSet(
	example.ProviderSet,
)
