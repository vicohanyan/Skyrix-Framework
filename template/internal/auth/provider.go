package auth

import (
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/auth/contracts"
	"gitlab.com/skyrix-lib/template/internal/auth/middleware"
	"gitlab.com/skyrix-lib/template/internal/auth/service"
	"gitlab.com/skyrix-lib/template/internal/auth/storage"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

type Service struct {
	AuthMiddleware *middleware.AuthMiddleware
}

func ProvideAuthService(log logger.Interface, jwtService *service.JWTService) *Service {
	return &Service{
		AuthMiddleware: middleware.NewAuthMiddleware(jwtService, log),
	}
}

func provideAuthStoreOpts() contracts.StoreOpts {
	return contracts.StoreOpts{}
}

var ProviderSet = wire.NewSet(
	storage.NewRedisAuthStore,
	service.NewJWTService,
	ProvideAuthService,
	middleware.NewAuthMiddleware,
	wire.FieldsOf(new(*config.Config), "JWT"),
	provideAuthStoreOpts,
	wire.Bind(new(contracts.Store), new(*storage.RedisAuthStore)),
)
