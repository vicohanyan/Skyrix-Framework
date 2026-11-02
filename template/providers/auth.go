package providers

import (
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"
	"gitlab.com/skyrix-lib/framework/extensions/auth/middleware"
	"gitlab.com/skyrix-lib/framework/extensions/auth/service"
	"gitlab.com/skyrix-lib/framework/extensions/auth/storage"

	"github.com/goforj/wire"
)

// AuthStoreSet provides the auth stores on Redis (passports, blacklist,
// sessions, refresh tokens). It needs RedisSet.
var AuthStoreSet = wire.NewSet(
	ProvidePassportOpts,
	storage.NewPassportStore,
	storage.NewTokenBlacklist,
	storage.NewSessionStore,
	storage.NewRefreshTokenStore,
	wire.Bind(new(contracts.PassportStore), new(*storage.PassportStore)),
	wire.Bind(new(contracts.TokenBlacklist), new(*storage.TokenBlacklist)),
	wire.Bind(new(contracts.SessionStore), new(*storage.SessionStore)),
	wire.Bind(new(contracts.RefreshTokenStore), new(*storage.RefreshTokenStore)),
)

// AuthProviderSet wires the auth platform package: stores, the JWT service
// and the bearer-token middleware. It needs ConfigSet (*config.JWT) and
// RedisSet.
var AuthProviderSet = wire.NewSet(
	AuthStoreSet,
	service.NewJWTService,
	middleware.OptionsFromConfig,
	middleware.NewAuthMiddleware,
)

// ProvidePassportOpts returns the passport cache options.
func ProvidePassportOpts() contracts.PassportOpts {
	return contracts.PassportOpts{StatusTTL: storage.DefaultPassportTTL}
}
