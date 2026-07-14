package auth

import (
	"skyrix/internal/config"
	"skyrix/internal/engine/auth/contracts"
	"skyrix/internal/engine/auth/middleware"
	"skyrix/internal/engine/auth/service"
	"skyrix/internal/engine/auth/storage"
	"skyrix/internal/logger"

	"github.com/google/wire"
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
