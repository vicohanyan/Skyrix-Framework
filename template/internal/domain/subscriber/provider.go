package subscriber

import (
	"gitlab.com/skyrix-lib/template/internal/domain/subscriber/repository"
	"gitlab.com/skyrix-lib/template/internal/domain/subscriber/services"

	"github.com/google/wire"
)

// ProviderSet exposes the public components of the subscriber domain (the service)
// and includes its internal dependencies (the repository) for wire to assemble.
var ProviderSet = wire.NewSet(
	services.NewSubscriberService,
	repository.NewSubscriberRepository,
)
