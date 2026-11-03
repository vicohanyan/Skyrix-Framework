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

### Atlas is optional: hand-written SQL migrations without the Atlas CLI

- `make migration` no longer needs the Atlas CLI, an Atlas account or a
  database: it verifies and updates `migrations/atlas.sum` with the official
  Atlas Go library (`ariga.io/atlas/sql/migrate` v1.3.0, the format of
  `atlas migrate hash`). The rules are unchanged: an empty directory gets
  `atlas.sum` with the first migration; with existing SQL files a missing or
  mismatching `atlas.sum` is an error before anything is written.
- Add `db migrate:hash`: recomputes `atlas.sum` after editing SQL by hand
  (`atlas.Workspace.Rehash`). It needs no Atlas CLI, database settings or Wire
  graph, never changes SQL files, replaces `atlas.sum` atomically, rejects
  invalid migration file names and concurrent changes, and leaves an empty
  directory without `atlas.sum`. Checksum errors now point to it.
- `db migrate:diff` stays the optional Atlas CLI integration (temporary
  config, private copy of `migrations/`, checksum verification, all-or-nothing
  publication). It now runs only `atlas migrate diff`; the checksum before
  and after is computed by the library. Without the CLI it fails before
  loading settings and names the manual process.
- The runner (`db migrate`, `migrate:status`, `migrate:baseline`) is
  unchanged and does not depend on Atlas.
- **Breaking:** `makecmd.MakeCommand.LookupAtlas`, `atlas.Hash` and the
  `binary`/writer parameters of `Workspace.Verify` are removed;
  `Workspace.Hash` is replaced by `Workspace.Sum`, and `atlas.HashCommand` is
  now `"db migrate:hash"`. `atlastest` fakes only `migrate diff`.
- Template resolves `ariga.io/atlas` v1.3.0 (previously a v0.36.2 pseudo
  version through `atlas-provider-gorm`).

### Empty Template migrations and atlas.sum handling

- Template ships without SQL migrations and without `atlas.sum`; the example
  model is no longer listed in `tools/schemaloader` or `Models`, so no
  command creates the example table on its own. `migrations` embeds `*` with
  a `README.md` placeholder and exposes only `*.sql` files, so it compiles
  with an empty directory. The README documents the explicit opt-in for the
  example table.
- `db migrate:diff` and `make migration` work on a private copy of
  `migrations/` (`atlas.Workspace`, `atlas.DiffOptions.Dir`): they verify the
  existing `atlas.sum` first (`atlas.ErrSumMissing`, `atlas.ErrSumMismatch`,
  with the explicit hash command), create it with the first
  migration, update it afterwards and publish SQL and checksum together. No
  schema changes or any error leave the project untouched.
- `make migration` maintains `atlas.sum` as well (previously it created the
  file and skipped `atlas.sum`); see above for how it is computed.
- The migration runner is unchanged and still ignores `atlas.sum`.

### Integration harness outside Template

- Add `integration/`, a separate test-only module (in `go.work`) for fixtures
  that copy the Template. Move `TestPlatformDriverSelection` (platformgen,
  platform/config commands, Template Auth providers) and the Tenant fixture
  (`TestTenantFixture`, `TestDefaultTemplateWithoutTenant`,
  `integration/testdata/tenant`) out of `template/providers` and
  `template/testdata`. Add `TestTemplateHasNoFrameworkFixtures`.
- Missing fixtures or external libraries (`SKYRIX_AUTH_LIB`,
  `SKYRIX_TENANT_LIB`) fail the harness instead of skipping it.
- Template keeps only application tests; its providers, Wire graphs and
  behaviour are unchanged. Template documentation points to the moved example.

### Local Tenant extraction and API middleware chain

- Move `extensions/tenant` with its tests to the separate local module
  `gitlab.com/skyrix-lib/tenant` (`/var/www/skyrix-tenant-lib`); no copy
  remains in framework, and framework must not import it
  (`TestFrameworkDoesNotImportTenant`). Tenant builds on the neutral
  `engine/scope` and `gormdb`. No repository, release or tag is published;
  `v0.0.0` with a relative `replace` is a local placeholder.
- Template: Tenant is disabled by default. Remove `providers/tenant.go`,
  `TenantProviderSet`, the `TENANT_CACHE` section and the tenant model from
  `db automigrate`; default binaries neither require nor import the module.
  Existing tables and migrations are untouched.
- Template router: replace the ignored `TenantMiddleware`/Noop parameter (and
  the explicit auth parameter) with `providers.APIMiddleware`, an ordered
  middleware chain for `/api/v1` chosen by `ProvideAPIMiddleware` (default:
  auth only). `/health` and `/metrics` stay outside it. A fixture
  (`integration/testdata/tenant`, `TestTenantFixture`) generates and runs the
  opt-in tenant graph.
- Tenant fixes: an unknown or inactive tenant header is 404 without domain or
  main fallback; an unknown host is 404 unless the application opts into
  `middleware.Options.MainFallback`; lookup failures are 503 and never become
  not found or main; a cancelled request never reaches the next handler;
  `ActiveTo` is honoured; the in-memory cache is bounded by the cache TTL
  counted from the database load (also across Redis); stored schema case is
  preserved; `X-Forwarded-Host` is read only with `TrustForwardedHost`; `Vary`
  names the configured tenant header on every response. Tenant queries
  qualify the main schema explicitly, including `ListDomains`.

### PostgreSQL sslmode and safe connection errors

- Template: add `DB_SSLMODE` to the `DATABASE` section (YAML and environment,
  default `disable`; an empty value also means `disable`). `disable` turns
  PostgreSQL TLS off; `allow`, `prefer`, `require`, `verify-ca` and
  `verify-full` are chosen explicitly. The HTTP binary, `db automigrate`/`db seed`
  and the `db migrate*` commands receive the same value. TLS is not enforced for
  production; certificates use the standard libpq `PGSSL*` variables.
- `postgres.Config.Validate` requires a port in 1-65535 (port 0 used to reach
  pgx) and an sslmode of disable, allow, prefer, require, verify-ca or
  verify-full; its messages contain no connection values.
- Fix a partial password leak: pgx's best-effort masking of unparsable
  connection strings missed passwords with quotes followed by spaces. Add
  `postgres.Config.ConnConfig` and `postgres.OpenPool`; `OpenSQL` and
  `gormdb.Open` parse the connection string once and return
  `*postgres.ParseError` (pgx's reason only, no connection string, no wrapped
  pgx error). GORM now receives an opened pool and never parses the connection
  string, so neither its errors nor its logger can contain it.

### Migration diff configuration

- Generate a private temporary Atlas configuration for db migrate:diff, with
  explicit project paths and cleanup on success, failure and cancellation.
- Remove Template's required atlas.hcl. Load optional MIGRATIONS_DEV_URL from
  application YAML/environment; keep the disposable PostgreSQL Docker default in
  Framework. Application database credentials are never used as diff scratch.
- Preserve APP_ENV/--env as the generated environment label. Existing custom HCL
  files remain untouched and are ignored by the wrapper; transfer dev URL settings
  or run Atlas directly for custom projects. Add config/CLI/cleanup regression tests.

### Scoped transactions and domain UoW scaffolding

- Add gormdb.Database.WithinTransaction(ctx, func(*Database) error): checked
  SET LOCAL before callback, GORM commit/rollback/panic and same-transaction
  savepoints for nested calls on tx; no automatic retries.
- Remove pooled session-level SET from WithContext. Preserve literal schema names,
  reject incompatible transaction scope and return schema setup errors. Raw/join
  SQL requires explicit qualification or WithinTransaction. SetSchema is restricted
  to managed matching scope; deprecated TxManager delegates with documented raw
  context limitations. Migrate example UoW and retain explicit repository WithTx.
- Add make uow with an embedded/project-overridable stub, constructor registration
  in the domain ProviderSet, conflict/path safety and existing atomic rollback.
  Repositories, business interfaces/services and binary Wire graphs remain explicit.
- Add driver-level lifecycle tests, opt-in dedicated PostgreSQL pool/schema tests,
  UoW golden/override/conflict tests and explicit repeatable Wire/build validation.

### Local Auth extraction

- Move `extensions/auth` and its tests to the local `gitlab.com/skyrix-lib/auth`
  module at `/var/www/skyrix-auth-lib`; no forwarding package or framework Auth
  dependency remains. Template owns providers, config sections and Wire graphs.
- Replace the combined JWT service with independent Signer/Validator contracts.
  Signing loads only a private RSA key; stateless validation needs only a public
  key. Keep constructor errors, logger.Interface, Wire cleanup and existing claims.
- Add opt-in JTI revocation with a separate Redis adapter, application/issuer
  isolation, authenticated lookup and atomic non-shortening expiry. Each signed
  access token receives a fresh JTI. Legacy blacklist/sessions are explicitly
  composed and keep their previous semantics; refresh rotation is unchanged.
- Use an external workspace and documented local v0.0.0 placeholders, not
  fabricated published versions. Existing services and EventBus are unchanged.

### Scaffolding generators

- Complete standalone make domain/entity/repository/service/handler/command
  [--db]/job/seeder/stubs commands adapted to Template's existing providers,
  console graphs, job registry and seeder contracts. Preserve make migration.
- Shared embedded templates with project overrides in stubs/skyrix, explicit
  export conflicts, module imports from go.mod and documented nested domain names.
- AST-directed registration preserves comments and user code, deduplicates new
  imports/providers and rejects unsupported or ambiguous composition shapes.
  Binary Wire sources and generated injectors remain explicitly user-managed.
- Plan files before writing; reject path/symbol/file conflicts and symlinks.
  Roll back failed writes with original contents/modes, including failed Atlas
  hash changes. No infrastructure, working config or tool installation is needed.
- Add golden/AST tests plus generation, explicit Wire and compilation of a full
  Template fixture under another module path; verify lazy DB commands, job wiring,
  project stubs, conflict handling, rollback and help without configuration.

### Requirements

- Go 1.27 or newer.
- Dependency injection uses [`github.com/goforj/wire`](https://github.com/goforj/wire).
  `github.com/google/wire` is not supported.
- `gitlab.com/skyrix-lib/eventbus` v1.1.0 or newer. EventBus v1.0.x is built on
  `google/wire`, ships no platform module, and does not work with this release.

### Driver contract and tooling changes

- JWT constructors return `(*JWTService, error)` instead of panicking on RSA key
  loading errors; regenerated Template Wire preserves reverse cleanup on failure.
  Token signing and verification behavior is unchanged.
- Minimal tooling-only driver manifest: `name`, `config_key`, `config_example`.
  No runtime metadata, version field, global registry or startup discovery.
- Driver command factories return `*console.Command`; explicit lazy graphs defer
  configuration/connections until execution. Help/completion build no graphs.
- platformgen replaces `config=Field` with independent typed Config/NewConfig
  adapters using Template's shared config.Section loader and manifest keys.
  Unselected drivers do not load settings or construct/start components.
- Standalone platform add/remove/list manage declarations and independent
  generated sets; repeated operations are no-ops, invalid input preserves files.
  Graphs, command choices and user config/domains remain application-owned.
  Dependencies and external tools must be managed explicitly by the user.
- Contract tests cover two real Wire graphs, JWT errors/cleanup, lazy commands,
  selected sections, file preservation and repeatable generation. External
  EventBus migration and live integration checks require its own source/module.

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
  Optional `//skyrix:platform` directives generate independent Wire adapters;
  the application explicitly composes modules and selects console commands.
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
- `cmd/platformgen` emits independent per-driver Wire sets, preserving
  typed config.Section adapters. It no longer generates an aggregate `PlatformSet`,
  `ProvidePlatform` or `PlatformCommands`; no directives produces an empty
  declaration file. Template explicitly selects modules per Wire graph and
  console commands through application providers. Existing providers/domain
  structure is retained. Tests exercise real Wire generation, independent driver
  config/construction/lifecycle, help/completion without graphs and repeatability.

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
  and platform modules are selected explicitly in each binary's Wire graph.
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
