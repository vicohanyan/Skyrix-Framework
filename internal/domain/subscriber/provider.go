package subscriber

import (
	"skyrix/internal/domain/subscriber/repository"
	"skyrix/internal/domain/subscriber/services"

	"github.com/google/wire"
)

// ProviderSet exposes the public components of the subscriber domain (the service)
// and includes its internal dependencies (the repository) for wire to assemble.
var ProviderSet = wire.NewSet(
	services.NewSubscriberService,
	repository.NewSubscriberRepository,
)
