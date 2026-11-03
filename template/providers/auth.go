package providers

import (
	"gitlab.com/skyrix-lib/auth/contracts"
	"gitlab.com/skyrix-lib/auth/middleware"
	revocationredis "gitlab.com/skyrix-lib/auth/revocation/redis"
	"gitlab.com/skyrix-lib/auth/service"
	"gitlab.com/skyrix-lib/auth/storage"

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

// AuthMiddlewareSet provides the gitlab.com/skyrix-lib/auth bearer-token middleware with the roles of JWT_ALLOWED_ROLES; an empty
// list accepts every role of a valid token. It needs a contracts.Validator.
var AuthMiddlewareSet = wire.NewSet(
	middleware.OptionsFromConfig,
	middleware.NewAuthMiddleware,
)

// AuthVerifierSet verifies tokens by signature, issuer, audience and expiry
// only, without Redis. It needs ConfigSet (*config.JWT, the public key at
// JWT_PUBLIC_KEY_PATH). The HTTP binary uses it.
var AuthVerifierSet = wire.NewSet(
	service.NewValidator,
	wire.Bind(new(contracts.Validator), new(*service.Validator)),
	AuthMiddlewareSet,
)

// AuthProviderSet wires the auth package with its Redis stores: the JWT
// validator also rejects blacklisted tokens and nonempty JTIs without a session. It
// needs ConfigSet (*config.JWT) and RedisSet. Use it instead of
// AuthVerifierSet, not together with it.
var AuthProviderSet = wire.NewSet(
	AuthStoreSet,
	service.NewValidator,
	ProvideLegacyValidator,
	wire.Bind(new(contracts.Validator), new(*service.LegacyValidator)),
	AuthMiddlewareSet,
)

// ProvidePassportOpts returns the passport cache options.
func ProvidePassportOpts() contracts.PassportOpts {
	return contracts.PassportOpts{StatusTTL: storage.DefaultPassportTTL}
}

// AuthSignerSet loads only the private key and never constructs Redis stores.
var AuthSignerSet = wire.NewSet(service.NewSigner, wire.Bind(new(contracts.Signer), new(*service.Signer)))

func ProvideLegacyValidator(v *service.Validator, blacklist contracts.TokenBlacklist, sessions contracts.SessionStore) (*service.LegacyValidator, error) {
	return service.NewLegacyValidator(v, blacklist, sessions)
}

// AuthRevocationSet requires RedisSet and ConfigSet. Use instead of AuthVerifierSet.
// Redis's application KeyPrefix plus each token's issuer isolate revocations.
var AuthRevocationSet = wire.NewSet(
	revocationredis.NewStore,
	wire.Bind(new(contracts.RevocationChecker), new(*revocationredis.Store)),
	wire.Bind(new(contracts.RevocationStore), new(*revocationredis.Store)),
	service.NewRevokingValidator,
	wire.Bind(new(contracts.Validator), new(*service.Validator)),
	AuthMiddlewareSet,
)
