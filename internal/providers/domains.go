package providers

import (
	"skyrix/internal/domain/order"
	"skyrix/internal/domain/subscriber"

	"github.com/google/wire"
)

var DomainProviderSet = wire.NewSet(
	order.ProviderSet,
	subscriber.ProviderSet,
)
