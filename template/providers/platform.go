package providers

import (
	"gitlab.com/skyrix-lib/framework/engine"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"
	natsv1 "gitlab.com/skyrix-lib/template/internal/transport/nats/v1"

	"github.com/google/wire"
	"gitlab.com/skyrix-lib/eventbus"
)

func ProvidePlatformRuntime(natsSubscribers *natsv1.SubscriberGroup) *engine.Runtime {
	return engine.NewRuntime(natsSubscribers)
}

func ProvideQueueConfig(cfg *config.Config) *config.Queue {
	return &cfg.Queue
}

func ProvideEventBusConfig(cfg *config.Queue) eventbus.Config {
	return cfg
}

func ProvideEventBusLogger(log logger.Interface) eventbus.Logger {
	return log
}

var PlatformProviderSet = wire.NewSet(
	ProvideQueueConfig,
	ProvideEventBusConfig,
	ProvideEventBusLogger,
	eventbus.ProviderSet,
	TenantProviderSet,
	natsv1.ProviderSet,
	ProvidePlatformRuntime,
	wire.Bind(new(engine.Platform), new(*engine.Runtime)),
	// auth.ProviderSet, // later
)
