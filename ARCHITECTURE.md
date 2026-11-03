# Skyrix Framework — Architecture (v0.3)

This document describes the framework as it is in the code of this repository.
Items that are planned but not implemented are marked **planned**.

## Two modules

| Module                            | Directory   | How a service uses it                                                        |
|-----------------------------------|-------------|------------------------------------------------------------------------------|
| `gitlab.com/skyrix-lib/framework` | `/`         | **Imported** (`go get`). Reusable kernel, engine, hosts and extensions.      |
| `gitlab.com/skyrix-lib/template`  | `template/` | **Copied** into a new repository. The application scaffold that imports the framework. |

The framework never imports the template. The template owns everything that is
specific to an application: configuration structure, Wire graphs, domains,
routes, migrations, Compose and Dockerfiles. During development the template
uses the framework through `replace gitlab.com/skyrix-lib/framework => ..`.

## Layers

```text
template (copied)     cmd/http · cmd/cobra · cmd/worker · providers · internal/domain/...
                          │ imports
extensions            external Auth · external Tenant (application composition)
                          │
hosts                 httpserver · cobra/console (+ corecmd) · database/dbcmd, migrate
                          │
engine                engine/postgres · engine/postgres/gormdb · engine/redis · engine/scope · engine/jobs
                          │
kernel                kernel · kernel/jobs                     (standard library only)
```

Dependencies point down. The rules that are enforced by tests are listed in
[Dependency rules](#dependency-rules).

### kernel — contracts and the execution core

`kernel` is the microkernel. It depends only on the standard library.

- Contracts: `Component` (`Start`, `Stop`; `Start` must not block),
  `Kernel` (the lifecycle of a binary), `FailureNotifier` (background work that
  can fail after `Start`), `HealthChecker` (`Check(ctx)`).
- `Runtime` (`NewRuntime(components...)`): starts components in order; if one
  fails to start, the ones already started are stopped in reverse. `Stop`
  stops all components in reverse order and joins the errors. `Failed`
  forwards the first failure of a component that implements `FailureNotifier`.
- `Run(ctx, kernel, options...)`: the run loop of every long-running binary.
  It starts the kernel, waits until `ctx` is cancelled (signal) or a component
  fails, then stops the kernel within `DefaultStopTimeout` (10 s,
  `WithStopTimeout`).
- Module registry: `Module` (`Name`, `Components`) and `Platform`
  (`NewPlatform(modules...)` validates names and keeps registration order).
  Further capabilities are optional interfaces declared by the hosts that use
  them, for example `cobra.CommandProvider`.
- `kernel/jobs`: the `Job` and `Registry` contracts.

### engine — core infrastructure

The `engine` root package contains no code; the modules are subpackages.

| Package                  | Contents                                                                                                                                                                                                 |
|--------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `engine/postgres`        | `database/sql` only, no GORM: `Config` (`Validate`, quoted `DSN`, `ConfigurePool`), `OpenSQL` (pgx driver, pings), `Component` (`NewComponent(*sql.DB)`: `Start` pings, `Stop` closes, `Check` health check). |
| `engine/postgres/gormdb` | GORM on top of `engine/postgres`: `Open`/`Provide` (`*gorm.DB`, lazy, no ping), `NewComponent(*gorm.DB)` → `*postgres.Component`, `Database`/`NewDatabase` (schema switching), the `DB` interface, the schema-routing `Plugin`, `TxManager` (deprecated). |
| `engine/redis`           | `Config` (with required `KeyPrefix`), `Open`/`Provide` (`*redis.Client` from go-redis, not wrapped), `Component`, `Keyspace` (`Sub`, `Key`). Consumers build their own stores on the client.             |
| `engine/scope`           | The request data scope in `context.Context`: `WithSchema`/`SchemaFrom`, model scopes `Scoped` (default) and `Main` (`ScopedModel`, `MainModel`), `TableBasable`. Standard library only.               |
| `engine/jobs`            | `RuntimeRegistry` (`NewRegistry`), `ExecuteJob`/`ExecuteJobAsync` (retries, panic protection); `engine/jobs/jobscmd` is the `jobs:run` console command.                                                  |

#### Request schema

`engine/scope` is the only channel through which the database layer learns the
schema of a request, and it knows nothing about where the schema comes from:

1. an extension resolves the schema and stores it with `scope.WithSchema`
   (for example the middleware of the separate Tenant library, when the
   application adds it to its API middleware chain);
2. `gormdb.Database.WithContext(ctx)` binds context and schema metadata without
   issuing SQL or changing pooled connection settings;
3. the registered `gormdb.Plugin` qualifies primary CRUD tables, using main or
   request scope. Raw/Row/Scan, joins and explicit table expressions require
   explicit SQL qualification or a pinned `WithinTransaction` operation.

### Scoped GORM transactions

`Database.WithContext(ctx)` binds context and metadata for the registered schema
plugin; it executes no SQL and does not change a pooled connection's search_path.
The plugin qualifies the primary CRUD table (main models use the main schema;
scoped models use `scope.SchemaFrom(ctx)`, falling back to main). It does not
rewrite joins, Raw/Row/Scan, explicit table expressions or arbitrary SQL.

Use a domain UoW and `WithinTransaction` for unqualified SQL and multi-repository
operations:

```go
err := database.WithinTransaction(ctx, func(tx *gormdb.Database) error {
    // Both repository copies must use this tx; preserve logger/other dependencies.
    orders := orderRepo.WithTx(tx)
    history := historyRepo.WithTx(tx)
    if err := orders.Save(ctx, order); err != nil { return err }
    return history.Append(ctx, entry)
})
```

Before the callback, GORM begins a transaction with ctx and gormdb checks
`SET LOCAL search_path`. The wrapper retains the same SQL transaction, main
schema and selected scope. WithContext on that wrapper accepts the same effective
schema, including derived contexts with new deadlines; incompatible scope returns
`ErrScopeMismatch` in the GORM session's Error. The original transaction context
still controls the maximum lifetime. An empty main schema means `public`.
Schema names are literal identifiers: case and whitespace are preserved, embedded
quotes escaped, and NUL rejected. No silent lowercasing is performed.

Callback nil commits; an error or panic rolls back through GORM, with panic
propagated. Begin/SET LOCAL/commit errors are returned; errors retain wrapping for
errors.Is/As. Rollback errors follow GORM's behavior and do not replace the
callback error. No automatic retries: a failed commit can have an unknown result.

Nested calls **on tx** use GORM savepoints in the same SQL transaction. Inner
success is provisional until outer commit; inner failure rolls back to its
savepoint. Scope changes and unmanaged raw GORM transactions are rejected, as is
nesting with DisableNestedTransaction enabled. Calling the original root database
starts independent work; context does not propagate a transaction automatically.
Do not share a tx concurrently, commit it manually, or retain it after callback.

Migration: replace `db.WithContext(ctx).Transaction(...)` with
`db.WithinTransaction(ctx, func(tx *gormdb.Database) error { ... })`; adapt explicit
repository WithTx methods to keep this wrapper and copy other dependencies.
Ordinary WithContext no longer guarantees search_path for arbitrary SQL. Use
WithinTransaction or explicitly qualify every relevant SQL identifier. Do not
replace SET with SET LOCAL outside a transaction. Deprecated SetSchema only
accepts the matching scope of an already managed transaction and never switches
schema. TxManager delegates to WithinTransaction, but its raw *gorm.DB callback
cannot enforce scope if a caller replaces context; migrate to the wrapper API.

Integration tests use only `SKYRIX_TEST_POSTGRES_DSN`, which must identify a
**dedicated test database** with CREATE/DROP SCHEMA permission. Without it they
skip; recording-driver tests do not establish real PostgreSQL pool isolation.


### hosts

| Package                                  | Contents                                                                                                                                                                                                                 |
|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `httpserver`                             | `NewServer` (`ReadHeaderTimeout` defaults to 10s), `NewRouter` (chi: request ID, client IP, optional recover, request log, timeout, optional request decompression, response compression), `RegisterSystemRoutes` (`GET /health` runs the `HealthChecks` in parallel, `GET /metrics` Prometheus), `Host` — the server as a `kernel.Component` that binds on `Start` and reports serving errors through `Failed`. `httpserver/handler`: JSON decoding, validation and structured errors; `httpserver/middleware`: recover, gzip decompression; `httpserver/gzip`. |
| `cobra`                                  | Console capability of platform modules: `CommandProvider` + `ModuleCommands`.                                                                                                                                            |
| `cobra/console`                          | Cobra console host: `Command` (a command described once: metadata, flags and a `Runner`), `Standalone`/`With` (runners without dependencies or with a lazily built `Graph`), `Needs` (`NeedsNothing`, `NeedsDatabase`, `NeedsApp`), `Graph` (`NewGraph` around a Wire injector, `Derive`, `Value`) and `Console` (`New`, `Add`, `AddCobra`, `Execute`). |
| `cobra/corecmd`                          | Describes the framework commands: `db` (`database/dbcmd`), `jobs:run`, `make` (`Commands(Options)`).                                                                                                                       |
| `database/migrate`                       | Forward-only SQL migration runner (see [Migrations](#migrations)), standard library only. `migratecmd` — the `db migrate*` commands; `atlas` — runs the Atlas CLI.                                                       |
| `database/dbcmd`                         | The `db` command group: the migration commands plus `db automigrate` (GORM AutoMigrate, development only) and `db seed`. The application supplies only `dbcmd.Models` and `dbcmd.Seeders`.                              |

There is no worker host package: the template's `cmd/worker` is a
`kernel.Runtime` of platform components (empty in the template: it starts,
waits for a signal and stops). A job scheduler and a gRPC host are
**planned**.

### extensions

Optional packages plugged in by the application; the engine and the kernel do
not know them.

- Authentication is the separate `gitlab.com/skyrix-lib/auth` module. Its
  `service.Signer` needs a private RSA key, `service.Validator` needs a public
  RSA key, and `revocation/redis` optionally implements access-token JTI
  revocation. Legacy full-token blacklist, live sessions, passport and refresh
  stores remain separate. Template composes it; framework must not import it.
- Schema-per-tenant resolution is the separate `gitlab.com/skyrix-lib/tenant`
  module (`/var/www/skyrix-tenant-lib`, formerly `extensions/tenant`). It
  depends on the public `engine/scope`, `engine/postgres/gormdb`,
  `engine/redis` and `logger` packages; framework never imports it
  (`TestFrameworkDoesNotImportTenant`). Its middleware resolves a tenant by
  header or host and stores the tenant's schema with `scope.WithSchema`;
  `gormdb` then routes CRUD to that schema. Template does not enable it by
  default; an application adds its providers to the HTTP Wire graph and its
  `Handle` to the API middleware chain. The application owns the `tenants`
  table and its migrations.

EventBus (NATS) is a platform driver in a separate module,
`gitlab.com/skyrix-lib/eventbus`; the template does not include it and
connects it as a platform module (see [Platform modules](#platform-modules)).

### support packages

`logger` (`logger.Interface`, with independent slog-backed instances and owner-managed file cleanup),
`validation` (go-playground validator), `scaffold` and `scaffold/makecmd`
(`make domain|repository|service|migration`), `cmd/platformgen` (generator of
independent platform Wire adapters), `internal/depcheck` (dependency tests).

## Platform modules

A platform module (platform driver, an optional core such as EventBus)
implements `kernel.Module` and may implement `cobra.CommandProvider`. The
application declares its modules in `providers/platform.go`, for example:

```go
//go:generate go run gitlab.com/skyrix-lib/framework/cmd/platformgen
//skyrix:platform gitlab.com/skyrix-lib/eventbus/platform
```

`go generate` runs `cmd/platformgen`, which writes `providers/platform_gen.go`
with an independent Wire set for every directive, for example
`EventbusplatformSet`. Set names capitalize the existing import alias and append
`Set`. Each set includes only that driver's `ProviderSet` and a typed config
provider. The driver exports a struct `Config` and `NewConfig() *Config` returning
fresh defaults without I/O. The generated provider calls Template's
`Config.LoadSections(config.Section{Key: manifest.ConfigKey, Value: value})`.
There are no interface bindings to fields on the application's Config. Unselected
sets load no settings and construct no modules or components. Without declarations,
only an empty `PlatformSections()` metadata function is generated. It supplies
fresh section defaults for config:env and never loads settings or calls Validate.
No shared `PlatformSet`, platform constructor or command list is generated.

The application owns `ProvideHTTPPlatform` and `ProvideWorkerPlatform` in
`providers/platform.go`. Each binary's `wire.Build` explicitly selects its driver
sets and corresponding platform provider. Add typed module parameters to that
provider and pass them to `kernel.NewPlatform` in lifecycle order. The returned
components join the runtime through `kernel.Platform.Components()`. The template
ships with empty platforms and the EventBus directive commented out.

Console selects commands separately in the application-owned
`providers.ProvideConsolePlatformCommands`, for example with
`frameworkcobra.ModuleCommands(new(selecteddriver.Module))`. Factories must work
on zero-value modules without loading settings, building a graph or starting
components just to list commands or show help. `CommandFactory` returns
`*console.Command`, registered with `Console.Add`. A command that needs DI uses
`console.With` with its explicitly selected lazy graph; the application closes it.
Completion and help use descriptions only. Constructors, config loading and
connections belong to execution, including cleanup when a constructor fails.

Several independent sets remain in the same providers package; domains retain
their own `ProviderSet`. Importing a shared package may put all its driver imports
in `go list -deps`, but Wire calls only the constructors required by the selected
graph. Adding a driver directive does not load its configuration, construct its
module, start its components or register its commands by itself. No binary
registry, runtime filtering or startup discovery is involved.

The driver package ships `manifest.yaml` with exactly `name`, `config_key`, and
`config_example` (a YAML string containing a section-body mapping). Tooling rejects
unknown fields, invalid names/section keys, duplicate declarations/names,
overlapping keys and colliding env prefixes. It reads manifests from explicitly
declared packages using Go package metadata; no package initialization or discovery
runs. Manifest data controls tooling only. Versions live in go.mod.

`scaffold/platform` parses declarations, validates package contracts and manifests,
and renders independent sets. `cobra/platformcmd.Command()` exposes standalone
`platform add <import-path>`, `remove <import-path>`, and `list`, with `--dir`
(default `providers`). List prints name, config key and import path. Add/remove
plan and validate all output before writing, stage both files, and restore the
original declaration on a failed second rename. Repeated adds/missing removes are
no-ops. They preserve user code, config and domains; they only edit declaration
lines and the generated adapters. They never modify graph or entrypoint files,
run go get/tidy, or install tools. Add requires the package dependency to be
available; missing-dependency errors tell users to add it explicitly.

Connecting a driver: explicitly add its dependency, run platform add (or write a
declaration and run platformgen), configure the manifest's dotted section key,
select its set in the chosen Wire graphs and typed modules in application
providers, select command descriptions separately, then regenerate Wire with an
installed tool. Remove only removes declarations/adapters: first edit any graphs
that reference the set before regenerating Wire. The application retains the
structure Wire graph → application providers → domain/infrastructure ProviderSets.

External EventBus must migrate before using this adapter: ship manifest.yaml;
replace the config interface with a concrete driver-owned Config and pure
NewConfig defaults; accept the typed config in ProviderSet without a second
YAML/env load; keep validation in Config.Validate and mark secrets; return
console.Command descriptions with deferred DI/cleanup. Check any EventBus callers
of JWT constructors for the new error result. Its sources are unavailable here,
so no external changes or live NATS checks were made. Two local test drivers
exercise the contract through generated independent Wire graphs instead.

External Auth `NewSigner`, `NewValidator` and `NewRevokingValidator` return
errors for invalid configuration/key loading, preserving causes. Wire cleans up
already acquired resources. Signer assigns a fresh JTI; the revoking validator
checks it only after signature/claims validation. Legacy policies are explicit.

## Binaries and Wire graphs (template)

Each binary has its own `wire.go` and includes only the provider sets it uses.

| Binary       | Provider sets                                                                                                                   | Runtime components (start order)        |
|--------------|---------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|
| `cmd/http`   | `ConfigSet`, `PostgresSet`, `ProvideHTTPPlatform`, `DomainProviderSet`, handlers, global middleware, `AuthVerifierSet`, `ProvideAPIMiddleware`, router, `HTTPProviderSet` | postgres → platform modules → HTTP host |
| `cmd/cobra`  | `buildApp`: `ConfigSet`, `ConsoleJobProviderSet`, `commands.ProviderSet`; `buildDatabase`: `ConfigSet`, `PostgresSet`, `DatabaseCommandSet` | — (runs one command)                    |
| `cmd/worker` | `ConfigSet`, `ProvideWorkerPlatform`                                                                                                      | platform modules (none by default)      |

`RedisSet` and `AuthProviderSet` (auth with Redis stores) exist in
`providers` but no binary includes them by default. The router applies
`providers.APIMiddleware`, an ordered chain chosen by a provider of the
binary's graph, to `/api/v1`; it knows nothing about the elements. The default
`ProvideAPIMiddleware` returns only bearer-token authentication
(`AuthVerifierSet`, token verification without Redis). `/health` and
`/metrics` are outside the chain and stay open. Tenant resolution is not part
of the default chain; see [template/README.md](template/README.md#tenants).

Assembly order rule: from the root of the dependency graph to the leaves, both
in `wire.Build` and in the runtime: engine infrastructure (config, logger,
PostgreSQL, Redis) → platform modules (platform drivers such as EventBus)
→ domains and the hosts that serve them. `kernel.Runtime` stops in reverse, so a host never
runs without the infrastructure it uses.

Every console command is described once, as a `console.Command`: metadata
for help and completion, and a runner that gets its dependencies only when the
command runs. The framework commands are described by `cobra/corecmd`, the
application commands by `internal/commands.All`, and platform module commands
are added as explicitly selected Cobra commands (`ProvideConsolePlatformCommands`). The descriptions are cheap
to build; the dependency graphs are Wire injectors wrapped in a
`console.Graph` and called lazily, right before the command that needs them:

1. help and completion use only the metadata: nothing is configured or
   connected;
2. standalone commands (`console.NeedsNothing`: `make`, `db migrate*`,
   platform commands) build no graph; `db migrate*` connect only to
   PostgreSQL when they run;
3. `db automigrate` and `db seed` (`console.NeedsDatabase`) build only the
   database graph (`buildDatabase`);
4. `jobs:run` and the application commands (`console.NeedsApp`) build the
   application graph (`buildApp`).

What a command needs follows from its runner (`console.Standalone` or
`console.With(graph, ...)`), so it cannot drift from what the handler uses.

## Migrations

- Forward-only SQL files in the Atlas directory format:
  `<YYYYMMDDHHMMSS>_<name>.sql`, or `.notx.sql` for statements that must run
  outside a transaction. The application embeds its `migrations/` directory.
- `database/migrate.Runner` applies them through `database/sql` (no GORM):
  records each migration with a run number, checksum and application version
  in `skyrix_migrations` (configurable), refuses modified applied files
  unless explicitly allowed, and serialises runners with a PostgreSQL
  advisory lock.
- Commands: `db migrate` (`--steps`, `--to`), `db migrate:status`,
  `db migrate:baseline <version>`, `db migrate:diff <name>` (generates a
  migration from the GORM models with `atlas migrate diff`, needs the `atlas`
  binary), `make migration <name>`. Diff writes a private temporary HCL config,
  invokes Atlas with an explicit --config and removes the file afterwards.
  Application settings supply AtlasEnv and optional AtlasDevURL; Template loads
  MIGRATIONS_DEV_URL from the MIGRATIONS section/environment. The default dev URL
  lives once in Framework and uses disposable PostgreSQL 18 in Docker. Main DB
  credentials are not copied into the Atlas config. Model loader and migration
  paths follow project conventions; custom project HCL is ignored and preserved.
  Migrations are applied by
  `db migrate`, never by `atlas migrate apply`.
- `db automigrate` (GORM AutoMigrate + seeders) is for development only and is
  not recorded in `skyrix_migrations`.

## Dependency rules

The foundation is explicitly `kernel/...` (currently `kernel` and
`kernel/jobs`), `config` (consumer-owned sections and loader), and `logger`
(logging contract and independent slog instances). Hosts, CLI commands,
generators, `validation` and migration utilities are not foundation merely
because they are reusable or have their own dependency rules.

Foundation production code must not reach `engine/...`, `extensions/...`,
concrete platform drivers (including the external
`gitlab.com/skyrix-lib/eventbus` module), or Template, directly or transitively.
The config/logger rule also rejects hosts (`httpserver`, `cobra`, `database`),
scaffolding and generators (`scaffold`, `cmd`), and the current concrete driver
libraries (`github.com/jackc/pgx`, `github.com/redis/go-redis`, `gorm.io`,
`github.com/nats-io`, including their subpackages). Add new concrete driver
module paths to this rule when introducing them. `kernel/...` retains the
stricter standard-library-only rule; config may use YAML and logger keeps its
slog implementation. The config/logger checks also cover their subpackages.
These rules do not change `config.Section` or `config:env`.

The rules are checked by tests built on `internal/depcheck`, which reads the
production import graph with `go list -deps` (without `-test`):
`TestImports` and `XTestImports`, including depcheck tooling, are excluded.
Failures name the source package and the import chain to the forbidden package.
`go test` does not see changes of the import
graph as test inputs, so run these tests with `-count=1`.

| Rule                                                                                             | Test                                                |
|--------------------------------------------------------------------------------------------------|-----------------------------------------------------|
| `config` and `logger` do not reach engine, extensions, hosts, generators, concrete drivers or Template, directly or transitively | `internal/depcheck/boundaries_test.go` |
| `kernel/...` imports only the standard library                                                   | `kernel/deps_test.go`                               |
| `engine/...` and `kernel/...` do not reach `extensions/...`, directly or transitively            | `internal/depcheck/boundaries_test.go`              |
| framework does not reach the external `gitlab.com/skyrix-lib/auth` and `gitlab.com/skyrix-lib/tenant` modules | `internal/depcheck/boundaries_test.go` |
| `kernel/...`, `engine`, `engine/postgres`, `engine/scope/...`, `engine/redis/...`, `engine/jobs/...` do not reach `gorm.io/...`; GORM stays in `engine/postgres/gormdb` | `internal/depcheck/boundaries_test.go` |
| `engine/postgres` imports only the standard library, the pgx `database/sql` driver and `kernel`  | `engine/postgres/postgres_test.go`                  |
| `engine/redis/...` imports only the standard library, go-redis, `kernel` and `logger`            | `engine/redis/deps_test.go`                         |
| `database/migrate` (the runner) imports only the standard library                                | `database/migrate/deps_test.go`                     |

## Template versus framework

| | Framework | Template |
|---|---|---|
| Distribution | Go module, imported with `go get` | copied into a new repository |
| Contents | kernel, engine, hosts, extensions, generators | `cmd/*` binaries and Wire graphs, `providers`, `internal/config`, domains, handlers, router, migrations, Compose, Dockerfiles |
| Changes | versioned (v0.3 is not compatible with v0.2.x, see CHANGELOG.md) | owned by the service after copying |
| Example code | — | `internal/domain/example`, `internal/commands/HelloCommand.go`, `internal/jobs/systemPingJob.go` |
| Tests | unit tests per package; `integration/` (separate test-only module): Template copies for generators, platform commands and the opt-in Auth/Tenant wiring | tests of the application, its providers, Wire graphs and example domain; no framework fixtures or `testdata` |

The `make uow <domain-path>/<name>` generator uses the shared embedded renderer,
project `stubs/skyrix/uow.stub` override and atomic AST registration. It creates a
concrete UOW, constructor and Execute(ctx) transaction skeleton in the domain's
unitOfWork package. The domain owns participants, business parameters/interfaces
and service composition; no repository registry, kernel UoW contract or binary
Wire graph changes are generated. `make stubs` exports the new template too.
