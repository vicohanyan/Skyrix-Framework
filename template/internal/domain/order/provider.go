package order

import (
	"github.com/google/wire"
)

// ProviderSet exposes the public components of the order domain (the service)
// and includes its internal dependencies (the repository) for wire to assemble.
var ProviderSet = wire.NewSet(
// service.NewOrderService,
// repository.NewOrderRepository,
)
