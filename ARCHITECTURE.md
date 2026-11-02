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
extensions            extensions/auth · extensions/tenant
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
   (the tenant middleware does this for every request);
2. `gormdb.Database.WithContext(ctx)` reads `scope.SchemaFrom(ctx)` and sets
   `search_path` to `<schema>, <main>, public`, or `<main>, public` when the
   context carries no schema;
3. `gormdb.Plugin`, when registered with `db.Use`, qualifies the table of
   every query/create/update/delete callback: `scope.Main` models use the main
   schema, all other models the request schema (main when there is none).
   Statements that bypass these callbacks (`Raw`, `Row`, `Scan`) rely on
   `search_path` only.

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

- `extensions/auth`: `contracts` (store interfaces), `context` (claims in
  context), `middleware` (bearer-token middleware, roles from
  `JWT_ALLOWED_ROLES`), `security` (claims, password hashing), `service`
  (`Config`, JWT signing and verification), `storage` (passport cache, token
  blacklist, sessions, refresh tokens on Redis under the `auth` key
  namespace).
- `extensions/tenant`: `entity.Tenant` (a `scope.Main` model), `repository`
  and `service` (in-memory and Redis cache), `schemaResolver` (by header or
  domain), `middleware` (`TenantMiddleware` resolves the schema and stores it
  with `scope.WithSchema`; `CorsMiddleware`), `context` (how the tenant was
  resolved). The application owns the `tenants` table and its migrations; see
  [extensions/tenant/README.md](extensions/tenant/README.md).

EventBus (NATS) is a platform driver in a separate module,
`gitlab.com/skyrix-lib/eventbus`; the template does not include it and
connects it as a platform module (see [Platform modules](#platform-modules)).

### support packages

`logger` (`logger.Interface`, with independent slog-backed instances and owner-managed file cleanup),
`validation` (go-playground validator), `scaffold` and `scaffold/makecmd`
(`make domain|repository|service|migration`), `cmd/platformgen` (generator of
the platform composition), `internal/depcheck` (dependency tests).

## Platform modules

A platform module (platform driver, an optional core such as EventBus)
implements `kernel.Module` and may implement `cobra.CommandProvider`. The
application declares its modules in `providers/platform.go`, for example:

```go
//go:generate go run gitlab.com/skyrix-lib/framework/cmd/platformgen
//skyrix:platform gitlab.com/skyrix-lib/eventbus/platform config=EventBus
```

`go generate` runs `cmd/platformgen`, which writes `providers/platform_gen.go`
with `PlatformSet` (Wire), `ProvidePlatform` and `PlatformCommands`. The
module's components join the binary's runtime through
`kernel.Platform.Components()`. Without directives the generated
`PlatformSet` is empty and every binary gets a platform without modules; the
template ships this way, with the EventBus directive commented out.

Connecting a driver takes the same steps for every driver: add its module
(`go get`), configure it, enable its directive, then run `go generate` and
`go tool wire gen`. How a driver is configured is described in the driver's
documentation. The configuration mechanism of platform drivers is
**planned** to be revised; until then the driver documentation is the
reference for the current mechanism.

## Binaries and Wire graphs (template)

Each binary has its own `wire.go` and includes only the provider sets it uses.

| Binary       | Provider sets                                                                                                                   | Runtime components (start order)        |
|--------------|---------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|
| `cmd/http`   | `ConfigSet`, `PostgresSet`, `PlatformSet`, `DomainProviderSet`, handlers, global middleware, `AuthVerifierSet`, router, `HTTPProviderSet` | postgres → platform modules → HTTP host |
| `cmd/cobra`  | `buildApp`: `ConfigSet`, `ConsoleJobProviderSet`, `commands.ProviderSet`; `buildDatabase`: `ConfigSet`, `PostgresSet`, `DatabaseCommandSet` | — (runs one command)                    |
| `cmd/worker` | `ConfigSet`, `PlatformSet`                                                                                                      | platform modules (none by default)      |

`RedisSet`, `AuthProviderSet` (auth with Redis stores) and
`TenantProviderSet` exist in `providers` but no binary includes them by
default. The HTTP binary protects `/api/v1` with the framework auth
middleware through `AuthVerifierSet` (token verification without Redis);
`/health` and `/metrics` stay open.

Assembly order rule: from the root of the dependency graph to the leaves, both
in `wire.Build` and in the runtime: engine infrastructure (config, logger,
PostgreSQL, Redis) → platform modules (platform drivers such as EventBus,
tenant, auth) → domains and the hosts that serve them. `kernel.Runtime` stops in reverse, so a host never
runs without the infrastructure it uses.

Every console command is described once, as a `console.Command`: metadata
for help and completion, and a runner that gets its dependencies only when the
command runs. The framework commands are described by `cobra/corecmd`, the
application commands by `internal/commands.All`, and platform module commands
are added as Cobra commands (`PlatformCommands`). The descriptions are cheap
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
  binary and `atlas.hcl`), `make migration <name>`. Migrations are applied by
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
