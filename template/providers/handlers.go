package providers

import (
	"github.com/google/wire"

	"gitlab.com/skyrix-lib/template/internal/handlers"
)

type Handlers struct {
	ExampleTask *handlers.ExampleTaskHandler
}

var HandlerProviderSet = wire.NewSet(
	handlers.NewExampleTaskHandler,
	wire.Struct(new(Handlers), "*"),
)
