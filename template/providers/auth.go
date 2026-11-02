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

// AuthMiddlewareSet provides the framework bearer-token middleware
// (extensions/auth/middleware) with the roles of JWT_ALLOWED_ROLES; an empty
// list accepts every role of a valid token. It needs a *service.JWTService.
var AuthMiddlewareSet = wire.NewSet(
	middleware.OptionsFromConfig,
	middleware.NewAuthMiddleware,
)

// AuthVerifierSet verifies tokens by signature, issuer, audience and expiry
// only, without Redis. It needs ConfigSet (*config.JWT, the public key at
// JWT_PUBLIC_KEY_PATH). The HTTP binary uses it.
var AuthVerifierSet = wire.NewSet(
	service.NewJWTVerifier,
	AuthMiddlewareSet,
)

// AuthProviderSet wires the auth package with its Redis stores: the JWT
// service also rejects blacklisted tokens and tokens without a session. It
// needs ConfigSet (*config.JWT) and RedisSet. Use it instead of
// AuthVerifierSet, not together with it.
var AuthProviderSet = wire.NewSet(
	AuthStoreSet,
	service.NewJWTService,
	AuthMiddlewareSet,
)

// ProvidePassportOpts returns the passport cache options.
func ProvidePassportOpts() contracts.PassportOpts {
	return contracts.PassportOpts{StatusTTL: storage.DefaultPassportTTL}
}
