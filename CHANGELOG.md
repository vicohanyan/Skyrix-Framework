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
- `kernel.PostgresOptions`, `kernel.RedisOptions`, `kernel.ProvidePostgres` and
  `kernel.ProvideRedis` moved to the `engine` package.
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
  library only. `engine.OpenPostgresSQL` opens a `database/sql` pool without
  GORM, and `buildinfo.Version` carries the application version via ldflags.
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
- SQL migrations: `db migrate` (with `--steps` and `--to`), `db migrate:status`,
  `db migrate:baseline`, `db migrate:diff` (Atlas) and `make migration`. The
  former `db migrate` alias of `db automigrate` is removed; AutoMigrate is a
  development-only command.
