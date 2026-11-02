# Changelog

All notable changes to `gitlab.com/skyrix-lib/framework` are documented here.
The project follows [Semantic Versioning](https://semver.org/). While the major
version is 0, a minor release may contain breaking changes.

## [v0.3.0] — unreleased

Pre-releases are tagged `v0.3.0-beta.N`. The public API may still change
between beta releases; `v0.3.0` is tagged once services have migrated.

**This release is not backward compatible with v0.2.x.** There is no
compatibility layer for v0.2.x applications: services and modules built for
v0.2.x must be migrated before upgrading. Modules released for v0.3.0 do not
support v0.2.x.

### Requirements

- Go 1.27 or newer.
- Dependency injection uses [`github.com/goforj/wire`](https://github.com/goforj/wire).
  `github.com/google/wire` is not supported.
- `gitlab.com/skyrix-lib/eventbus` v1.1.0 or newer. EventBus v1.0.x is built on
  `google/wire`, ships no platform module, and does not work with this release.

### Breaking changes

- The module path changed from `skyrix` to `gitlab.com/skyrix-lib/framework`.
  The framework is an importable library; the runnable application lives in the
  separate `gitlab.com/skyrix-lib/template` module.
- `kernel` is the microkernel: neutral contracts and the assembly of a binary,
  standard library only. The `kernel.Kernel` service container and
  `kernel.NewKernel` are removed; `kernel.Kernel` is now the lifecycle
  interface.
- `kernel/jobs` contains only the `Job` and `Registry` contracts. The in-memory
  registry moved to `engine/jobs` as `RuntimeRegistry` (`engine/jobs.NewRegistry`).
- PostgreSQL is infrastructure in `engine/postgres`: `Config` (with quoted
  DSN, `SSLMode` and pool settings), `Open`/`Provide` (`*gorm.DB`), `OpenSQL`
  (`*sql.DB`), and the `kernel.Component` lifecycle (`NewComponent`: Start
  pings, Stop closes, `Check` is the health check). `Open` no longer pings;
  add the component to the runtime. Removed: `kernel`/`engine.PostgresOptions`,
  `ProvidePostgres`, `engine.OpenPostgresSQL` and `internal/postgres`.
- The `buildinfo` package is removed. The application passes its version to
  the migration commands with `migratecmd.Settings.AppVersion`.
- Auth moved from the template to `extensions/auth` (`contracts`, `context`,
  `middleware`, `security`, `service`, `storage`). `service.Config` replaces
  the template `config.JWT`; `middleware.NewAuthMiddleware` takes
  `middleware.Options{Roles}` (empty allows every role) and stores the claims
  with `auth/context.WithClaims` instead of template context keys.
- `httpserver.RegisterSystemRoutes(r, checks)` takes `httpserver.HealthChecks`.
  `GET /health` no longer always answers `ok`: it runs every check
  (`kernel.HealthChecker`, implemented by the PostgreSQL and Redis
  components) in parallel within 2 seconds and returns 200 or 503 with the
  status of each check as JSON; check errors are not exposed.
- `extensions/auth/security` defines no role constants. Roles come from the
  configuration: `service.Config.AllowedRoles` (`JWT_ALLOWED_ROLES`) and
  `middleware.OptionsFromConfig`.
- `httpserver/gzip`: `GzipBytes` and `GunzipBytesWithLimit` (moved from the
  template's `internal/utils/gzip`). `FastCompressionLevel` is now
  `gzip.BestSpeed` (was 5, the same as `OptimalCompressionLevel`), and
  exceeding the limit returns `gzip.ErrTooLarge` instead of
  `io.ErrUnexpectedEOF`.
- The `db` console group (`migrate*`, `automigrate`, `seed`) is built into the
  framework (`database/dbcmd`); the application supplies only `dbcmd.Models`
  and `dbcmd.Seeders`. `cobra/corecmd` assembles the framework commands
  (`db`, `jobs:run`, `make`) and the standalone ones.
- Redis is rebuilt as infrastructure in `engine/redis`. Removed:
  `engine.Redis`, `engine.NewRedisService`, `engine.RedisOpts`,
  `engine.RedisOptions`, `engine.ProvideRedis`, the `engine.Cache` interface
  and `internal/redis`. Use `engine/redis.Config`, `Open`/`Provide`
  (`*redis.Client` from go-redis), `NewComponent` and `Keyspace` instead.
  `Config.KeyPrefix` is required and has no default; the hardcoded
- `extensions/tenant/service.NewTenantService(log, repo, cache)` takes a
  `service.Cache` (implemented by `extensions/tenant/cache.RedisCache`);
  `service.CacheOpts` is removed. Cache keys are unchanged below the prefix:
  `<prefix>:tenant:namespace:<ns>`, `<prefix>:tenant:domain:<domain>`.
- `engine.Runtime` and `engine.NewRuntime` moved to `kernel.Runtime` and
  `kernel.NewRuntime`; the `engine.Platform` alias is removed.
- `httpserver.HTTPApp` and `httpserver.NewHTTPApp` are removed. The HTTP server
  is a component (`httpserver.NewHost`) added to a `kernel.Runtime`, and every
  binary runs its runtime with `kernel.Run`.
- The `platform` package is removed. Modules implement `kernel.Module`
  (`Name`, `Components`) and are registered with `kernel.NewPlatform`; console
  commands are an optional capability, `cobra.CommandProvider`, collected with
  `cobra.ModuleCommands`.
- `cobra.NewConsoleApp(logger, groups)` takes a logger. `ConsoleApp.Kernel` is
  replaced by `ConsoleApp.Logger`.
- `gitlab.com/skyrix-lib/framework/cobra/eventbus` is removed. The EventBus CLI
  lives in `gitlab.com/skyrix-lib/eventbus/cobra` and is registered through the
  EventBus platform module.

### Added

- Compile-time platform modules (`kernel.Module`, `kernel.Platform`) contribute
  lifecycle components and, through `cobra.CommandProvider`, console commands.
  Applications declare them with `//skyrix:platform` directives and generate
  the composition with `template/internal/scaffold/platformgen`.
- `kernel.Run` runs a binary: it starts the runtime, waits for the context or
  a component failure (`kernel.FailureNotifier`), and stops the runtime in
  reverse order within a timeout. `httpserver.Host` binds its listener during
  start and reports serving errors as failures.
- `database/migrate`: forward-only SQL migrations in the Atlas directory format
  with run numbers, checksums, baselines and a Postgres advisory lock; standard
  library only. `engine/postgres.OpenSQL` opens a `database/sql` pool without
  GORM.
- `engine/redis`: Redis connection (`Config` with address, credentials, DB,
  TLS, pool and timeouts, and the required `KeyPrefix`), the
  `kernel.Component` lifecycle with a `Check` health check, and `Keyspace`
  (`Sub`, `Key`) so the services and consumers sharing a Redis database do not
  collide. A dependency test keeps the package free of its consumers.
- Framework code moved out of the template: `cmd/platformgen` (run with
  `go run gitlab.com/skyrix-lib/framework/cmd/platformgen`, `-config` sets the
  config package), `database/migrate/migratecmd` and `database/migrate/atlas`
  (migration commands), `scaffold/makecmd` (`make domain|repository|service|
  migration`), `engine/jobs/jobscmd` (`jobs:run`), `cobra.ExecuteStandalone`
  (commands that run without the container) and `httpserver/handler`
  (`BaseHandler`, JSON decoding, structured errors).

### Deprecated

- `engine/jobs.Job` and `engine/jobs.Registry` (aliases of the `kernel/jobs`
  contracts) remain only to ease the move inside this repository and will be
  removed in a later release. New code must use the `kernel/jobs` types.

### Template

- The application template targets v0.3.0 only. The `Queue` configuration
  section is renamed to `EventBus`, the kernel container provider is removed,
  and platform modules are wired through the generated `PlatformSet`.
- The template builds three binaries with separate Wire graphs: `cmd/http`,
  `cmd/cobra` and `cmd/worker` (events and background work, no HTTP and no
  database). Infrastructure is split into `ConfigSet`, `PostgresSet` and
  `RedisSet` so each binary includes only what it uses.
- Redis: `REDIS_KEY_PREFIX` is required (no default) and
  `TENANT_CACHE_KEY_PREFIX` is removed. `RedisSet` provides `*redis.Client`,
  the `engineredis.Keyspace` and `*engineredis.Component`; no template binary
  includes it yet. `AuthProviderSet` wires `extensions/auth`. Auth storage is
  split into `PassportStore`,
  `TokenBlacklist`, `SessionStore` and `RefreshTokenStore`;
  `RotateRefreshToken` is atomic and rejects unknown, expired, already used
  or foreign tokens.
- `PostgresSet` provides `*enginepostgres.Component`; runtimes start
  components from the root of the dependency graph to the leaves (engine
  infrastructure, platform modules, then hosts serving the domains) and stop
  in reverse. `APP_VERSION` is recorded with applied migrations.
- `/health` checks PostgreSQL (`providers.ProvideHealthChecks`); the auth
  middleware accepts the roles in `JWT_ALLOWED_ROLES` (`[customer]`).
- `internal/auth`, `internal/utils`, `internal/scaffold` and the
  `DBMigrator`/`DBSeeder` are removed; `internal/database` lists only the
  models and seeders for the framework db commands.
- SQL migrations: `db migrate` (with `--steps` and `--to`), `db migrate:status`,
  `db migrate:baseline`, `db migrate:diff` (Atlas) and `make migration`. The
  former `db migrate` alias of `db automigrate` is removed; AutoMigrate is a
  development-only command.
