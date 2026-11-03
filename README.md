# Skyrix Framework

Go framework for Skyrix backend services (`gitlab.com/skyrix-lib/framework`).
It is a library: services import it. It provides

- `kernel` — the microkernel: lifecycle contracts, `Runtime`, the `Run` loop
  and the platform module registry (standard library only);
- `engine` — core infrastructure: PostgreSQL (`engine/postgres` on
  `database/sql`, GORM in `engine/postgres/gormdb`), Redis (`engine/redis`),
  the request data scope (`engine/scope`) and background jobs (`engine/jobs`);
- hosts — the HTTP server (`httpserver`), the Cobra console (`cobra/console`,
  `cobra/corecmd`, module commands in `cobra`) and the database commands with the SQL migration runner
  (`database/dbcmd`, `database/migrate`);
- extensions — `extensions/tenant` (schema-per-tenant resolution);
- optional authentication — the separate `gitlab.com/skyrix-lib/auth` library,
  composed by Template; framework does not import Auth;
- tooling — `cmd/platformgen`, `scaffold/makecmd`, `logger`, `validation`.

New services start from the application template in [`template/`](template/),
which is copied, not imported (see [template/README.md](template/README.md)).
The architecture, package layers and dependency rules are described in
[ARCHITECTURE.md](ARCHITECTURE.md).

## Installation

```bash
go get gitlab.com/skyrix-lib/framework@<version>
```

v0.3 is not released yet; its pre-releases will be tagged `v0.3.0-beta.N`
(see [CHANGELOG.md](CHANGELOG.md)). The latest tag is `v0.2.2-beta`.

## Compatibility

| Framework | Go | Wire | EventBus |
|---|---|---|---|
| v0.3.x | 1.27+ | `github.com/goforj/wire` | v1.1.0+ |
| v0.2.x | 1.26+ | `github.com/google/wire` | v1.0.x |

v0.3.0 is not backward compatible with v0.2.x, and there is no compatibility
layer. Services and modules must be migrated to v0.3.0 explicitly; modules
released for v0.3.0 do not support v0.2.x. See [CHANGELOG.md](CHANGELOG.md)
for the list of breaking changes.

## Minimal example

An HTTP binary with PostgreSQL, `/health`, `/metrics` and one route. The
template does the same assembly with Wire.

```go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log, cleanup := logger.NewLogger(logger.Config{LogOutput: "stdout", LogType: "text", LogLevel: "info"})

	defer cleanup()

	// PostgreSQL through GORM; the component pings on start and closes on stop.
	db, err := gormdb.Open(postgres.Config{
		Host: "localhost", Port: 5432, User: "app", Password: "secret", Database: "app",
	})
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := db.Use(&gormdb.Plugin{}); err != nil {
		log.Error("register schema plugin", "error", err)
		os.Exit(1)
	}
	database := gormdb.NewDatabase(db, "public")
	dbComponent, err := gormdb.NewComponent(db)
	if err != nil {
		log.Error("database component", "error", err)
		os.Exit(1)
	}

	// HTTP: /health, /metrics and an application route.
	router := httpserver.NewRouter(httpserver.RouterOptions{Timeout: 5 * time.Second})
	httpserver.RegisterSystemRoutes(router, httpserver.HealthChecks{{Name: "postgres", Checker: dbComponent}})
	router.Get("/now", func(w http.ResponseWriter, r *http.Request) {
		var now string
		if err := database.WithContext(r.Context()).Raw("SELECT now()::text").Scan(&now).Error; err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(now))
	})
	host := httpserver.NewHost(httpserver.NewServer(router, httpserver.Options{Address: "0.0.0.0", Port: 8080}), log)

	// Components start in order and stop in reverse: database first, HTTP last.
	if err := kernel.Run(ctx, kernel.NewRuntime(dbComponent, host)); err != nil {
		log.Error("run", "error", err)
		os.Exit(1)
	}
}
```

## Development

```bash
go build ./... && go vet ./... && go test -count=1 ./...
```

`-count=1` matters for the dependency tests (`internal/depcheck`), which read
the import graph with `go list`. Tests against real servers run when
`SKYRIX_TEST_POSTGRES_DSN` or `SKYRIX_TEST_REDIS_ADDR` is set. The template
is a separate module in `template/`; the repository `go.work` includes both.

## Section configuration

`framework/config` has no engine, extension, host or Template dependencies.
Consumers own structs and defaults; composition explicitly supplies sections:

```go
type EventBusConfig struct {
    Enabled bool          `yaml:"enabled"`
    Timeout time.Duration `yaml:"timeout"`
    Servers []string      `yaml:"servers"`
    Credentials struct {
        Password string `yaml:"password" secret:"true"`
    } `yaml:"credentials"`
}
func (c *EventBusConfig) Validate() error {
    if c.Enabled && len(c.Servers) == 0 {
        return errors.New("servers required")
    }
    return nil
}

cfg := &EventBusConfig{Timeout: 5 * time.Second}
loader, err := config.New(config.Section{Key: "platform.eventbus", Value: cfg})
if err != nil { return err }
if err := loader.Load(yamlReader, os.LookupEnv); err != nil { return err }
// Use cfg in DI. Diagnostics must use loader.Safe(), never cfg or Section.Value.
sections := loader.Sections() // keys, prefixes, YAML paths and fields
fields := loader.Fields() // detached metadata: path, env, type, secret, safe default
safeValues := loader.Safe() // detached tree; working values remain unchanged
```

```yaml
platform:
  eventbus:
    enabled: true
    timeout: 2s
    servers: ["nats://localhost:4222"]
    credentials:
      password: ""
```

Loading overlays the supplied struct defaults with present YAML fields, then
present environment variables, including empty strings. It never treats false,
zero, an empty string or an empty list as an absent value. Every selected section
is loaded before its `Validate() error` is called. Unselected sections are neither
decoded nor validated; configuration loading opens no infrastructure connections.
A failed load can partially change targets: discard the targets on error. Create
fresh consumer-owned targets for independent configurations; do not share mutable
default slices between them. Loader and targets are not safe for concurrent mutation.

`cobra/configcmd.Env(sections...)` describes the standalone `config:env` console
command. Pass fresh section targets containing defaults only, explicitly from the
application (or through `corecmd.Options.ConfigSections`); no global registry is
used. The command lists section, env name, Go type and default, sorted by section
key and env name. `--section <key>` selects an exact key; unknown keys are errors.
Strings are quoted, durations use duration notation and secret defaults are always
`[REDACTED]`, including fields inside secret structs. Metadata snapshots defaults
at `config.New`, before any load; legacy `env-default` tags take precedence over
struct defaults. It never prints values from YAML or the environment, calls
`Load`/`Validate`, or builds a dependency graph.

Naming rules are deliberately mechanical:

- Keys match `[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*`. YAML traverses the dotted key.
- Fields use an explicit `yaml` name, or the entire Go field name lowercased
  (`ReadTimeout` becomes `readtimeout`). Prefer explicit snake_case YAML tags.
  Names match `[A-Za-z][A-Za-z0-9_]*`; `yaml:"-"` excludes a field. Unexported
  fields are ignored. YAML tag options (including inline) are unsupported.
- Nested structs add a dot-separated field component. Env names uppercase the
  full key and field path, replacing dots with underscores. Thus
  `platform.eventbus.credentials.password` maps to
  `PLATFORM_EVENTBUS_CREDENTIALS_PASSWORD`; the section prefix is
  `PLATFORM_EVENTBUS_`. No acronym or camel-case splitting is performed.
- Duplicate/ancestor section keys, duplicate YAML field names, and collisions
  after env normalization are errors, including `a_b` versus nested `a.b`.
  Selected YAML mappings reject duplicate keys. Unknown fields and unselected
  sections are ignored, allowing a shared application YAML file.
- Scalars support strings, booleans, signed/unsigned integers, floating-point
  numbers and `time.Duration`; structs nest and lists contain supported scalars
  or lists. Env strings are literal; other env values use YAML scalar/list syntax,
  e.g. `PLATFORM_EVENTBUS_SERVERS='["nats://one", "nats://two"]'`.
  Duration env values use Go duration syntax (`500ms`, `2s`). YAML supports duration
  strings too. Pointers, maps, lists of structs, nulls and aliases are unsupported.
  Supply one YAML document; `Load(nil, lookup)` uses defaults and env only.
- `secret:"true"` masks the entire field, including nested structs and lists.
  `Safe()` also masks secret descendants. Conversion errors contain canonical
  parameter paths but suppress values and decoder details. Validation errors
  report the section key and suppress the validator's error text (and cause),
  since even an unmarked validator error can embed credentials. Validators must
  not log raw configuration themselves. Raw structs remain working values and
  must never be sent to diagnostics.

Template keeps its existing uppercase YAML layout and unprefixed env names:
`APP_ENV`, `LOGGER.LOG_OUTPUT`, `DATABASE.DB_PASS`, `JWT.JWT_ALLOWED_ROLES`, etc.
There is no settings-file migration. Its compatibility adapter uses Section's
explicit `YAMLPath` and `LegacyEnv`: explicit `env` tags override derived names,
`env-default` tags are applied before YAML, and string-list env values retain the
legacy comma-separated format (whitespace trimmed, empty env means empty list).
New sections should use derived names and initialized Go defaults instead.
`YAMLPath: nil` follows the key; an empty non-nil path selects the document root.
Env collisions are checked across the explicit composition, including overrides.

`LoadConfig` now loads application identity and logger only. Wire section providers
load HTTP/Postgres/Auth/Redis/Tenant only when used by the binary; migration
settings explicitly select Postgres. Adding a section requires an explicit typed
provider that loads and returns its consumer-owned configuration; this needs no
network provider. ConfigSet alone does not enable infrastructure. Driver adapters
use the same `config.Section` loader; the former `config=Field` option is removed.
`config:env` describes fresh defaults without reading YAML/env or calling Validate.


## Explicit platform composition

`cmd/platformgen` generates independent driver sets in the application's
`providers/platform_gen.go`, loading typed driver config via `config.Section`. It generates no
shared platform constructor or all-driver command list. Select individual sets
in each binary's `wire.Build`, and pass only the selected typed modules to an
application-owned platform provider. Console command selection is explicit and
independent of runtime composition. Several sets can share one providers file;
domains keep their own ProviderSets. See [Platform modules](ARCHITECTURE.md#platform-modules).

The Template demonstrates this with `ProvideHTTPPlatform`, `ProvideWorkerPlatform`
and `ProvideConsolePlatformCommands`. Unselected drivers' configuration and
components are not built merely because their sets are declared. Shared package
imports can still appear in `go list -deps`; this does not imply construction or
startup. No binary directives, registries or runtime filters are introduced.

Driver packages export `Module`, `ProviderSet`, `Config` (a struct) and
`NewConfig() *Config` (fresh defaults only). Their `manifest.yaml` contains exactly:

```yaml
name: example
config_key: platform.example
config_example: |
  enabled: false
```

`config_example` is the YAML section body, for tooling/documentation only. The
manifest is never read at application startup. Generated providers pass the typed
config and manifest key to Template's `Config.LoadSections`; defaults, YAML, env,
Validate and secret masking remain in the existing config loader.

```sh
go run ./cmd/cobra platform list
go run ./cmd/cobra platform add <driver-package>
go run ./cmd/cobra platform remove <driver-package>
```

Run these from Template (or pass `--dir <providers-directory>`). Add/remove update
only active declarations in `platform.go` and independent sets/config metadata in
`platform_gen.go`. Repeated adds and missing removals print `No changes.` Invalid
input is checked before publishing either file. Remove preserves user config,
domains and Wire sources. Edit Wire sets, module providers and command selection
explicitly before regenerating Wire; removing a selected set requires that edit
before the next Wire generation. No go get/tidy or tool installation is run.
If a driver package is unavailable, add its module dependency explicitly and retry.

Driver `cobra.CommandFactory` now returns `*console.Command`. Metadata factories
must work on zero-value modules and perform no I/O; bind a lazy injector with
`console.With(graph, handler)` and close that graph in the console owner. Graphs
and resources are created only for execution, including cleanup on build errors.
Auth now lives in `gitlab.com/skyrix-lib/auth`. Its separate Signer and Validator
constructors return errors; callers must handle them. Each signed access token
receives a fresh JTI. Revocation is explicitly enabled by the application.

External EventBus sources were not modified or integration-tested. Its platform
package needs the manifest above, a concrete typed Config and fresh NewConfig,
providers accepting that config without reloading it, and console.Command factories
with deferred dependency construction/cleanup. See ARCHITECTURE.md for details.

## Scaffolding generators

Run `cobra make ...` from the project root (the directory containing go.mod).
Generators use only source files and stubs: no working configuration, connections
or application Wire graph is loaded. Module imports are read from go.mod.

```sh
cobra make domain sales/order_flow
cobra make entity sales/order_flow/line_item
cobra make repository sales/order_flow/line_item
cobra make service sales/order_flow/reconcile_batch
cobra make handler sales/order_flow/invoice_view
cobra make command reconcile_accounts
cobra make command scan_invoices --db
cobra make job refresh_balances
cobra make seeder currency_codes
cobra make stubs
cobra make migration custom_trigger [--notx]
```

Names consist of ASCII letter-led segments with digits after the first letter;
`_` and `-` separate compound words. Empty segments, whitespace, Go keywords,
absolute paths, traversal and symlinks below the project root are rejected.
Domain path segments also reject `main` and `init`. Domain paths may be nested:
`sales/order_flow` becomes `internal/domain/sales/orderflow` (package `orderflow`,
type prefix `SalesOrderFlow`). Domain components require `<domain-path>/<name>`.
Commands, jobs and seeders accept a single name, with no `/`. Snake/kebab case
is recommended: `reconcile_batch` becomes type `ReconcileBatch`, file prefix
`reconcileBatch`, CLI name `reconcile-batch`; job names use dots (`refresh.balances`).
Existing CamelCase input is accepted, with its remaining case preserved in Go
identifiers; its CLI name is simply lowercased. Names with the same normalized
file/type conflict rather than overwrite each other.

| Generator | Output and registration |
| --- | --- |
| domain | `provider.go` and package docs under dto/entity/interfaces/repository/services/unitOfWork; adds its ProviderSet to `providers/domains.go`'s DomainProviderSet and ConsoleDomainProviderSet |
| entity | Model and constructor in the domain's entity package; adds a model literal to `internal/database.Models` |
| repository/service | Concrete skeleton and constructor in the domain's repository/services package; adds constructor to its ProviderSet |
| uow | `make uow order/CreateOrder` creates `unitOfWork/createOrderUOW.go`, `CreateOrderUOW`, constructor and Execute(ctx) with WithinTransaction; registers only the domain constructor |
| handler | BaseHandler-based HTTP skeleton in `internal/handlers`; adds constructor and Handlers field in `providers/handlers.go`; no route is added |
| command | console.Command factory in `internal/commands`; registers in `commands.All`, receiving the existing lazy application graph |
| command --db | Registers in `commands.DatabaseAll`, receiving the existing lazy database graph; help/completion never build it |
| job | Job implementation in `internal/jobs`; constructor in both JobProviderSet and ConsoleJobProviderSet, field in Jobs, and registration in ProvideRegisteredJobsRegistry |
| seeder | Idempotent no-op Seeder in `internal/database/seeders`; constructor in database.ProviderSet and parameter/entry in Seeders |
| migration | Versioned SQL file; preserves --notx and installed Atlas hashing behavior |

UoW skeletons leave repositories, operation parameters, domain interfaces and service wiring explicit. `make stubs` includes `uow.stub`, overridable at `stubs/skyrix/uow.stub`.

Domain skeletons contain no invented business operations, HTTP/DB dependencies
or UnitOfWork contract. Repositories/services start as concrete types: introduce
meaningful domain interfaces and Wire bindings when defining actual operations.
Entities are registered for explicit model/schema commands; generation itself
never applies a schema change or creates a migration. Seeders have no storage
dependency until you add one. Command factories defer dependencies with
`console.With`; Template registers DatabaseAll against its existing database graph.

Registrations use Go AST locations, preserving comments and user registrations.
Supported points are explicit `wire.NewSet(...)` variables, Handlers/Jobs structs
with `wire.Struct(new(Bundle), "*")`, direct returned slice literals in
Models/Seeders/All/DatabaseAll, and the existing straight-line jobs registry.
Seeders may already have named parameters. Keyed slice entries, indirect returns,
custom registry control flow, duplicate imports and ambiguous shapes are rejected
before publishing any files. Custom projects must provide these Template points;
the generator does not invent or rewrite arbitrary composition code.

After changing providers, job bundles or seeder signatures, explicitly run:

```sh
go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker
go test ./...
go build ./...
```

Generators never modify binary Wire sources or wire_gen.go, install tools, run
go get/tidy, or select platform drivers. Use an already built console while
scaffolding, or regenerate Wire before rebuilding/running the console from source
when a changed signature makes the old injector stale. Command descriptions alone
need no Wire regeneration. Implement placeholders and add HTTP routes explicitly.

`make stubs` exports the embedded templates to `stubs/skyrix/*.stub`. It refuses
an export if any target already exists, preserving all existing stubs. For every
generator (including migration), a project file takes priority over the embedded
one; missing files fall back individually. All use Go text/template with strict
missing-key errors. Available fields: `Module`, `DomainImport` (domain generators),
`Package` (destination Go package), `Type`, `Var`, `Use` and `JobName`. `doc.stub`
is used for domain package docs; `command_db.stub` is the DB command variant;
`migration.stub` is empty by default and receives Module/Use. Keep each template's
required type/factory names and signatures; generated Go is formatted and parsed.

All outputs and registrations are prepared before application. Existing output
files and symbol conflicts are errors; repeated generation never overwrites.
Write failures roll back only the operation's files, restoring previous contents
and modes and removing only newly created empty directories. External edits
detected while publishing/rolling back are preserved and reported. Failed Atlas
hashing also restores the previous atlas.sum and removes the new migration.

## Local Auth library

Auth lives in the separate local module `gitlab.com/skyrix-lib/auth`, currently
at `/var/www/skyrix-auth-lib`. Framework has no dependency on it; Auth uses the
public `logger` and, in Redis adapters only, `engine/redis` packages.
Template owns configuration sections, provider sets and Wire composition.
See [Template authentication](template/README.md#authentication).

Use an external workspace rather than changing the checked-in `go.work`:

```go
go 1.27

use (
    /var/www/skyrix-framework
    /var/www/skyrix-framework/template
    /var/www/skyrix-auth-lib
)
```

Save it outside the modules, for example `/tmp/skyrix-auth-check/go.work`, and
set `GOWORK` to its absolute path for tests, vet, builds and Wire. Run checks
from each module separately: framework's `./...` does not include Template or
Auth. See the Auth README for its API and Redis adapter.

The local `v0.0.0` requirements for Auth/framework are technical placeholders,
not published releases. Relative `replace` directives support this local layout;
fixture modules rewrite those paths to absolute paths. `GOWORK=off` with local
replacements is not evidence of a standalone build from published dependencies.
No Auth repository or release has been published by this extraction.

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
