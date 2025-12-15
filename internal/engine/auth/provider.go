package auth

import (
	"skyrix/internal/config"
	"skyrix/internal/engine/auth/contracts" // Added import for contracts
	"skyrix/internal/engine/auth/middleware"
	"skyrix/internal/engine/auth/service"
	"skyrix/internal/engine/auth/storage"
	"skyrix/internal/logger"

	"github.com/google/wire"
)

// Service is an aggregator for auth-related services.
type Service struct {
	AuthMiddleware *middleware.AuthMiddleware
}

// ProvideAuthService constructs the AuthService aggregator.
func ProvideAuthService(log logger.Interface, jwtService *service.JWTService) *Service {
	return &Service{
		AuthMiddleware: middleware.NewAuthMiddleware(jwtService, log),
	}
}

// provideAuthStoreOpts creates contracts.StoreOpts.
// NewRedisAuthStore will use its internal defaults if these are empty.
func provideAuthStoreOpts() contracts.StoreOpts {
	return contracts.StoreOpts{}
}

// ProviderSet provides all components related to the auth domain.
var ProviderSet = wire.NewSet(
	storage.NewRedisAuthStore,
	service.NewJWTService,
	ProvideAuthService,
	middleware.NewAuthMiddleware,
	wire.FieldsOf(new(*config.Config), "JWT"),
	provideAuthStoreOpts,                                          // Added provider for contracts.StoreOpts
	wire.Bind(new(contracts.Store), new(*storage.RedisAuthStore)), // Bind *storage.RedisAuthStore to contracts.Store
	// We can also provide individual services if needed elsewhere
	// wire.FieldsOf(new(*AuthService), "JWT", "Session"),
)
