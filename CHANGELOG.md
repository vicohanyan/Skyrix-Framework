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

### Added

- Explicit foundation boundaries for `kernel/...`, `config` and `logger`:
  retain kernel's standard-library-only rule and extend config's existing
  transitive isolation check to logger, external EventBus and concrete driver
  libraries. Import-chain diagnostics and regression tests distinguish
  production dependencies from internal and external test tooling.

- Standalone `config:env` via `console.Command`: explicit application sections,
  stable section/env ordering, exact `--section` filtering, types and safe default
  snapshots. Secret defaults are masked; YAML/environment, validation and Wire
  infrastructure graphs are never loaded. Template supplies its available sections.

- Consumer-owned `config.Section` and instance-local `config.Loader`: explicit
  composition, defaults → YAML → env, validation, derived env metadata and
  secret-safe diagnostic trees. Conversion/validation errors suppress values.
  Dependency tests enforce isolation from engine, extensions, hosts and Template.
- Template selects configuration sections through Wire providers. Disabled
  Redis/Auth/Tenant/platform sections require no configuration; existing YAML
  keys and env names remain supported. See README for naming, secrets, supported
  types and compatibility rules. Logger DI and cleanup remain unchanged.

### Breaking changes

- `logger.NewLogger(cfg)` now returns `(logger.Interface, func())` and creates
  an independent instance on every call; the process-wide singleton is removed.
  Consumers retain `logger.Interface`; `SlogWrapper` adapts the slog backend.
  `Interface.With("key", value)` returns a derived logger for contextual fields;
  custom implementations and test doubles must implement this method too.
  Existing constructor calls must capture cleanup and run it after all consumers
  stop. Template Wire graphs own cleanup on shutdown and graph construction
  errors, allowing HTTP, Console and Worker configurations to remain independent.
  Levels, JSON/text, output selection and stdout fallback are preserved;
  cleanup closes owned files only. The migration runner retains its narrow
  `migrate.Logger` contract. `migratecmd.SettingsLoader` now returns
  `(Settings, func(), error)` with a non-nil cleanup on success, released on
  connection/runner errors, command completion and Atlas env lookup.

- The module path changed from `skyrix` to `gitlab.com/skyrix-lib/framework`.
  The framework is an importable library; the runnable application lives in the
  separate `gitlab.com/skyrix-lib/template` module.
- `kernel` is the microkernel: neutral contracts and the assembly of a binary,
  standard library only. The `kernel.Kernel` service container and
  `kernel.NewKernel` are removed; `kernel.Kernel` is now the lifecycle
  interface.
- `kernel/jobs` contains only the `Job` and `Registry` contracts. The in-memory
  registry moved to `engine/jobs` as `RuntimeRegistry` (`engine/jobs.NewRegistry`).
- PostgreSQL is infrastructure in `engine/postgres`, on `database/sql` without
  GORM: `Config` (with quoted DSN, `SSLMode` and pool settings,
  `Config.ConfigurePool`), `OpenSQL` (`*sql.DB`), and the `kernel.Component`
  lifecycle (`NewComponent(*sql.DB)`: Start pings, Stop closes, `Check` is the
  health check). Removed: `kernel`/`engine.PostgresOptions`,
  `ProvidePostgres`, `engine.OpenPostgresSQL` and `internal/postgres`.
- GORM lives in `engine/postgres/gormdb`: `Open`/`Provide` (`*gorm.DB`; `Open`
  no longer pings, add the component to the runtime), `NewComponent(*gorm.DB)`
  (returns `*postgres.Component`), the schema-aware `Database`/`NewDatabase`
  (was `engine.Database`/`engine.NewDatabaseService`), the `DB` interface
  (was `engine.DB`), the schema-routing `Plugin` (was `scope.Plugin`) and the
  deprecated `TxManager` (was `engine.TxManager`). The `engine` root package
  holds no code; `engine`, `engine/postgres`, `engine/scope`, `engine/redis`,
  `engine/jobs` and `kernel` do not import `gorm.io/...` (checked by
  `internal/depcheck`).
- The `buildinfo` package is removed. The application passes its version to
  the migration commands with `migratecmd.Settings.AppVersion`.
- Auth moved from the template to `extensions/auth` (`contracts`, `context`,
  `middleware`, `security`, `service`, `storage`). `service.Config` replaces
  the template `config.JWT`; `middleware.NewAuthMiddleware` takes
  `middleware.Options{Roles}` (empty allows every role) and stores the claims
  with `auth/context.WithClaims` instead of template context keys. The
  middleware matches the `Bearer` scheme case-insensitively (RFC 7235;
  `middleware.BearerToken` parses the header); another scheme or an empty
  token is rejected with 401.
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
  and `dbcmd.Seeders`. `cobra/corecmd.Commands` describes the framework
  commands (`db`, `jobs:run`, `make`).
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
- `engine/scope` carries the request schema without knowing about tenants:
  `scope.WithSchema`/`scope.SchemaFrom` replace `scope.WithTenant`/
  `scope.TenantFrom`, and `scope.Tenant`/`scope.TenantModel` are renamed
  `scope.Scoped`/`scope.ScopedModel`. `extensions/tenant/context.WithSchema`
  and `SchemaFrom` are removed: the tenant middleware stores the resolved
  schema with `scope.WithSchema`, and the database layer reads it from there.
  `engine/...` and `kernel/...` no longer import `extensions/...` (checked by
  `internal/depcheck`).
- `engine.Runtime` and `engine.NewRuntime` moved to `kernel.Runtime` and
  `kernel.NewRuntime`; the `engine.Platform` alias is removed.
- `httpserver.HTTPApp` and `httpserver.NewHTTPApp` are removed. The HTTP server
  is a component (`httpserver.NewHost`) added to a `kernel.Runtime`, and every
  binary runs its runtime with `kernel.Run`.
- The `platform` package is removed. Modules implement `kernel.Module`
  (`Name`, `Components`) and are registered with `kernel.NewPlatform`; console
  commands are an optional capability, `cobra.CommandProvider`, collected with
  `cobra.ModuleCommands`.
- The console host moved to `cobra/console`. Removed: `cobra.ConsoleApp`,
  `cobra.NewConsoleApp`, `cobra.CommandGroups` and `cobra.NewRootCommand`;
  `cobra` keeps only `CommandProvider` and `ModuleCommands`. Framework
  commands are `console.Command` descriptions: `dbcmd.Command` (with
  `dbcmd.Deps`/`NewDeps` in place of `dbcmd.Commands`/`New`),
  `jobscmd.Command` (replaces `JobsRunCommand`), `makecmd.MakeCommand.Command`
  and `migratecmd.Commands.Command`/`Subcommands` (replace `ToCobraCommand`
  and `CobraCommands`; `NewDBGroup` takes the subcommands).
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
  migration`), `engine/jobs/jobscmd` (`jobs:run`) and `httpserver/handler`
  (`BaseHandler`, JSON decoding, structured errors).

### Changed

- Console commands are described once (`cobra/console`): a `console.Command`
  holds the metadata (use, help texts, arguments, flags) and a runner that
  gets its dependencies only when the command runs — `console.Standalone`
  or `console.With(graph, handler)`, where `console.Graph` wraps a Wire
  injector and builds it on first use (`Derive` shares one graph between
  commands). What a command needs (`console.Needs`: standalone, database,
  application) follows from its runner. Help and completion are answered
  from the metadata and never build a graph; a command builds only its own
  graph, right before it runs. The template's `cmd/cobra` has an application
  graph (`buildApp`) and a database graph (`buildDatabase`, used by
  `db automigrate` and `db seed`); application commands are listed once in
  `internal/commands.All`.
- `httpserver.Options.ReadHeaderTimeout`: `NewServer` always sets
  `http.Server.ReadHeaderTimeout` (Slowloris protection); zero or negative
  uses `httpserver.DefaultReadHeaderTimeout` (10s). The template sets
  `APP_READ_HEADER_TIMEOUT`.
- `cmd/platformgen` accepts a `platform.go` without `//skyrix:platform`
  directives and generates an empty but valid `PlatformSet`
  (`ProvidePlatform` returns a platform without modules).

### Fixed

- `httpserver.NewRouter` with `RouterOptions.Timeout` zero or negative no
  longer installs `chi` `Timeout(0)`, which cancelled every request context
  immediately; a non-positive timeout now disables the request timeout.
- `extensions/tenant/entity.Tenant`: the unique indexes no longer carry
  `WHERE deleted_at IS NULL` — the model has no `deleted_at` column, so
  `db automigrate` failed. The index names are unchanged, so AutoMigrate
  leaves existing indexes alone.

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
- `/health` checks PostgreSQL (`providers.ProvideHealthChecks`).
  `JWT_ALLOWED_ROLES` is empty by default (the framework auth middleware then
  accepts every role of a valid token).
- `internal/auth`, `internal/utils`, `internal/scaffold` and the
  `DBMigrator`/`DBSeeder` are removed; `internal/database` lists only the
  models and seeders for the framework db commands.
- SQL migrations: `db migrate` (with `--steps` and `--to`), `db migrate:status`,
  `db migrate:baseline`, `db migrate:diff` (Atlas) and `make migration`. The
  former `db migrate` alias of `db automigrate` is removed; AutoMigrate is a
  development-only command.
- Delivery leftovers are removed: Compose, Dockerfiles, configuration and the
  README use neutral names (`app`, `app-postgres`, `app-redis`, binaries
  `/app/http`, `/app/cobra`, `/app/worker`, port 8080); `StoreIDFromIdentity`
  and the `store:` subject are removed from the auth middleware. PostGIS is
  removed from the template: PostgreSQL runs from `postgres:18-alpine`, there
  is no PostGIS image (`infra/postgres`) or PostGIS Atlas dev database, and
  the `enable_postgis` migration is replaced by the example-domain table
  migration.
- Unused leftovers are removed: `internal/helpers`
  (`GetSubscriberNameFromContext`), `internal/contextkeys` (the auth
  middleware no longer stores claims under `user_claims`/`id`; use
  `AuthIdentityFromContext`) and the `OAUTH` configuration section. The
  Dockerfiles build with Go 1.27.3, and `template/.gitignore` keeps
  `secret/`, `.env*`, `logs/` and `bin/` out of the repository.
- EventBus is no longer part of the template: the `//skyrix:platform`
  directive is commented out with instructions for connecting it as a
  platform driver, `internal/transport/nats` and the `QUEUE` configuration
  section are removed, and `gitlab.com/skyrix-lib/eventbus` is no longer a
  dependency. `cmd/worker` is an empty runtime until the application adds
  components.
- The template's own auth middleware (`internal/middleware/auth.go`,
  `AuthIdentity`, `RequireAnyRole`) is removed. The HTTP binary protects
  `/api/v1` with `extensions/auth/middleware` through
  `providers.AuthVerifierSet` (roles from `JWT_ALLOWED_ROLES`); `/health` and
  `/metrics` stay open. `providers.AuthMiddlewareSet` is shared by
  `AuthVerifierSet` and `AuthProviderSet`.
