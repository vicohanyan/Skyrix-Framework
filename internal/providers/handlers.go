package providers

import (
	"skyrix/internal/handlers"

	"github.com/google/wire"
)

type Handlers struct {
	Subscriber *handlers.SubscriberHandler
	// Order *handlers.OrderHandler
}

var HandlerProviderSet = wire.NewSet(
	handlers.NewSubscriberHandler,
	// handlers.NewOrderHandler,

	wire.Struct(new(Handlers), "*"),
)
