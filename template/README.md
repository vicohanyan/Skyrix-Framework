# Skyrix application template

Starting point for a new service on the Skyrix framework
(`gitlab.com/skyrix-lib/framework`). The template is **copied**, not imported:
a new project takes this directory, renames the module and owns the code from
then on. The framework itself is a Go module the project imports; see
[../README.md](../README.md) and [../ARCHITECTURE.md](../ARCHITECTURE.md).

The template contains one example domain (`internal/domain/example`, an
"example task" CRUD over HTTP) to show how domains, handlers, repositories,
units of work and migrations fit together. Delete it once the first real
domain exists.

## Starting a new project

1. Copy `template/` into a new repository.
2. Rename the module in `go.mod` (`gitlab.com/skyrix-lib/template`) and in all
   imports.
3. In `go.mod`, replace the local `replace` directive for
   `gitlab.com/skyrix-lib/framework` with a released version
   (`go get gitlab.com/skyrix-lib/framework@<version>`).
4. Rename the service: `SERVICE_NAME`, `DB_NAME`, `REDIS_KEY_PREFIX` in
   `config/*.yaml`, and the `app` names in
   `docker-compose.yaml` (project name, network aliases, volumes).
5. Regenerate code (`go generate ./...`, see below), then
   `go build ./... && go test ./...`.

## Binaries

Each binary has its own Wire graph (`cmd/<name>/wire.go`) and takes only the
providers it needs. Long-running binaries run their components with
`kernel.Run` (ordered start, reverse stop, shutdown on SIGINT/SIGTERM).

| Binary        | Graph                                                                                          | Purpose                                                |
|---------------|------------------------------------------------------------------------------------------------|--------------------------------------------------------|
| `cmd/http`    | config, logger, PostgreSQL (GORM), platform modules, example domain, handlers, middleware, router, HTTP host | HTTP API                                               |
| `cmd/cobra`   | two lazily built graphs: application (config, logger, jobs, application command deps) and database (config, logger, PostgreSQL (GORM), models, seeders) | Console: database, jobs, scaffolding, platform commands |
| `cmd/worker`  | config, logger, platform modules                                                               | Background work, no HTTP; an empty runtime until the application adds components |

`providers` also defines `RedisSet` and `AuthProviderSet`; no binary includes
them by default. Add a set to a binary's `wire.go` when it needs Redis or the
auth stores, and add its lifecycle component to the binary's runtime.

The `/api/v1` routes run behind `providers.APIMiddleware`, an ordered
middleware chain (the first element runs first). `providers.ProvideAPIMiddleware`
in `cmd/http/wire.go` chooses it; by default it holds only authentication. To
change the chain, replace that provider in the binary's graph with one that
returns the complete chain in the wanted order. The router only applies the
chain; `/health` and `/metrics` are outside it. Tenant resolution is optional
and disabled by default (see [Tenants](#tenants)).

## Authentication

The HTTP binary protects the API with the external Auth library middleware
(`gitlab.com/skyrix-lib/auth/middleware`), wired by `providers.AuthVerifierSet`:

- every route under `/api/v1` (the example routes) requires
  `Authorization: Bearer <JWT>` (the scheme is case-insensitive: `bearer`
  works too); a missing, malformed, expired or wrongly signed token, another
  scheme, or a token without a user ID gets 401;
- `GET /health` and `GET /metrics` stay open;
- the token is verified with the public key at `JWT_PUBLIC_KEY_PATH` (needed
  on start), `JWT_ALGORITHM`, and `JWT_ISSUER`/`JWT_AUDIENCE` when set;
- `JWT_ALLOWED_ROLES` limits the token roles: an empty list (the default)
  accepts every role of a valid token, otherwise other roles get 403:

  ```yaml
  JWT:
    JWT_ALLOWED_ROLES: [customer, admin]   # or JWT_ALLOWED_ROLES=customer,admin
  ```

The handlers read the claims with `gitlab.com/skyrix-lib/auth/context`.
`AuthVerifierSet` constructs `service.NewValidator` and checks only the token:
no Redis or private key is required. `AuthSignerSet` constructs `NewSigner`
with only the private key. All constructors return errors; Wire preserves
cleanup of already constructed resources. Both use `logger.Interface`.

For access-token JTI revocation, replace `AuthVerifierSet` with
`AuthRevocationSet` and include `RedisSet`. Add its Redis lifecycle component
to the binary's runtime. The application owns the Redis client and its cleanup.
`REDIS_KEY_PREFIX` must explicitly identify the application/revocation domain;
issuer and JTI are separately hashed under `<prefix>:auth:revoked`. Issuers and
validators sharing a revocation domain must use the same Redis database/prefix.
Revocation storage is called only after signature and claims verification.
Storage errors reject the request. Repeated revocation cannot shorten the expiry.

The alternative `AuthProviderSet` explicitly selects **legacy** full-token
blacklist and live-JTI sessions (requires `RedisSet`). A nonempty JTI must exist
in the session allowlist; an empty JTI skips that old check. This is distinct
from JTI revocation, where a missing entry means the token is allowed and a
missing JTI is rejected. Choose one validation set in a graph. The signer always
assigns a fresh access-token JTI, replacing a caller-supplied ID; when using the
legacy allowlist, save the actual signed token's JTI, not `NewClaims`' input ID.

Refresh tokens retain their existing Redis layout and atomic owner-checked Lua
rotation. Access revocation does not consume, rotate or revoke refresh tokens.
Claims, roles, YAML/env names and `config.Section{Key:"auth", YAMLPath:[]string{"JWT"},
LegacyEnv:true}` integration are unchanged. No platform module is registered.

Local development uses `/var/www/skyrix-auth-lib` and the external workspace
shown in the [framework README](../README.md#local-auth-library). Auth and
framework `v0.0.0` requirements are local placeholders resolved by workspace or
local replacements; they are not published releases. The existing framework
`go.work` is unchanged. Docker's current framework-only build context does not
include the sibling Auth directory; building that image requires a separately
adapted context or real published dependencies. The extraction checks Go builds,
not a Docker deployment.

## Running with Docker

The Compose stack starts the HTTP binary, PostgreSQL 18 and Redis on the
external network `skyrix-network`:

```bash
docker network create skyrix-network   # once
docker compose up -d --build
curl http://localhost:8080/health
```

Requirements: an RSA public key at `secret/jwt_public.pem`.

Debug build under Delve (port 2345):

```bash
docker compose -f docker-compose.yaml -f docker-compose.debug.yaml up -d --build
```

The image contains `/app/http`, `/app/cobra` and `/app/worker`.

## Native development

```bash
go build -o bin/http ./cmd/http
go build -o bin/cobra ./cmd/cobra
go build -o bin/worker ./cmd/worker
APP_ENV=local go run ./cmd/http
```

## Configuration

Configuration is read from `/config/<APP_ENV>.yaml`, then
`config/<APP_ENV>.yaml` (`APP_ENV` defaults to `local`); environment variables
with the same leaf names override the file (`APP_PORT`, `DB_HOST`,
`REDIS_HOST`, `JWT_PUBLIC_KEY_PATH`, ...). The structure is
in `internal/config/config.go`.

| Section        | Keys                                                                                  |
|----------------|---------------------------------------------------------------------------------------|
| root           | `APP_ENV`, `SERVICE_NAME`, `APP_VERSION` (recorded with applied migrations)           |
| `LOGGER`       | `LOG_OUTPUT`, `LOG_TYPE`, `LOG_LEVEL`, `LOG_DIR`, `LOG_FILE`                          |
| `HTTP_SERVER`  | `APP_ADDRESS`, `APP_PORT` (8080), `APP_REQUEST_TIMEOUT`, `APP_READ_HEADER_TIMEOUT` (10s) |
| `DATABASE`     | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_MAIN_SCHEMA`, `DB_SSLMODE` (disable) |
| `REDIS`        | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASS`, `REDIS_DB`, `REDIS_KEY_PREFIX`              |
| `JWT`          | `JWT_PUBLIC_KEY_PATH`, `JWT_ALGORITHM`, `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ALLOWED_ROLES`, ... |

`DB_SSLMODE` is the PostgreSQL (libpq) sslmode. The default `disable`, also
used when the value is empty, turns PostgreSQL TLS off; it fits a database on
the same host or reached over a VPN. TLS is enabled only by choosing another
mode explicitly: `allow`, `prefer`, `require`, `verify-ca` or `verify-full`.
For `verify-ca`/`verify-full`, point the standard libpq `PGSSLROOTCERT` (and,
for client certificates, `PGSSLCERT`/`PGSSLKEY`) environment variables at the
files. The HTTP binary, `db automigrate`/`db seed` and the `db migrate*`
commands use the same value. Connection errors never include the password or
the connection string.

`REDIS_KEY_PREFIX` has no default and is required by every binary that uses
Redis: it is the root of every key the service writes, so services sharing a
Redis database do not collide.

The checked-in values are development defaults. Do not commit production
credentials or private keys.

## Console commands

```bash
cobra --help
cobra config:env                  # env names, types and safe defaults
cobra config:env --section redis  # exact section key
cobra db migrate                 # apply pending SQL migrations
cobra db migrate:status          # state of the migrations
cobra db migrate:baseline        # mark migrations up to a version as applied
cobra db migrate:diff <name>     # generate a migration from the GORM models (needs atlas)
cobra make migration <name>      # empty migration for hand-written SQL
cobra db automigrate             # DEV ONLY: GORM AutoMigrate + seeders
cobra db seed                    # run the seeders
cobra jobs:run --name <job>      # run a registered job synchronously
cobra make <generator> <name>    # see Scaffolding generators below
cobra hello                      # example application command
```

Every command is described once, as a `console.Command`
(`gitlab.com/skyrix-lib/framework/cobra/console`): its metadata (use, help
texts, arguments, flags) and a handler that receives its dependencies only
when the command runs. Help and completion (`cobra`, `--help` of any command,
`help ...`, `completion`) are built from the metadata: they build no
dependency graph and connect to nothing. A command builds only the graph it
needs, right before it runs (`cmd/cobra/console.go`):

- `config:env`, `make`, `db migrate*` and platform-module commands are standalone;
  `db migrate*` connect only to PostgreSQL;
- `db automigrate` and `db seed` build the database graph (`buildDatabase`:
  config, logger, PostgreSQL, models, seeders);
- `jobs:run` and the application commands build the application graph
  (`buildApp`).

Framework commands come from the framework (`cobra/corecmd`); the
application supplies only its data: GORM models and seeders for
`db automigrate` / `db seed` in `internal/database`.

`config:env` lists all fields sorted by section key and env name, with their Go
types and declared defaults. `--section` accepts the keys `app`, `logger`, `http`,
`database`, `migrations`, `redis` and `auth` (plus `tenant` once enabled, see [Tenants](#tenants)); unknown keys return an error. Secret
defaults are `[REDACTED]`. Empty strings and zeroes represent unset defaults,
not required settings supplied at runtime. YAML development values are not defaults
in this reference. The command reads neither YAML nor environment values and
calls neither configuration loading/validation nor the application Wire graph.
The application explicitly supplies fresh section descriptions through
`internal/config.Sections()` in `cmd/cobra/console.go`; add available sections
there when extending application configuration. Logger and infrastructure
providers keep their existing loading behavior.

### Adding an application command

Application commands live in `internal/commands`. A command is added in one
place: write its description and list it in `commands.All`.

```go
// internal/commands/greet.go
func Greet(deps *console.Graph[*Deps]) *console.Command {
	return &console.Command{
		Use:   "greet <name>",
		Short: "Greet somebody",
		Args:  cobra.ExactArgs(1),
		Flags: func(flags *pflag.FlagSet) {
			flags.Bool("shout", false, "Shout the greeting")
		},
		Run: console.With(deps, func(cmd *cobra.Command, args []string, deps *Deps) error {
			greeting := "Hello, " + args[0] + "!"
			if shout, _ := cmd.Flags().GetBool("shout"); shout {
				greeting = strings.ToUpper(greeting)
			}
			deps.Logger.Info("greet", "name", args[0]) // deps are built only now
			_, err := fmt.Fprintln(cmd.OutOrStdout(), greeting)
			return err
		}),
	}
}

// internal/commands/commands.go
func All(deps *console.Graph[*Deps]) []*console.Command {
	return []*console.Command{Hello(deps), Greet(deps)}
}
```

Read flag values from `cmd.Flags()` inside the handler. When the command
needs another dependency (a repository, a domain service), add a field to
`commands.Deps` and, if its provider set is not in `buildApp` yet, add the set
to `cmd/cobra/wire.go`, then run `go tool wire gen ./cmd/cobra`. A command
that needs nothing from the application uses `console.Standalone(handler)`
instead of `console.With`.

## Migrations

Use SQL migrations in production: generate them during development with
`cobra db migrate:diff <name>`, review and commit the SQL, then apply the
prepared migrations during deployment with `cobra db migrate`.
`cobra db automigrate` is intended only for local development and is not
recommended in production: it changes the schema directly without recording
SQL migration history. Avoid mixing both approaches on the same database.

`migrations/` holds forward-only SQL migrations in the Atlas directory format
(`<YYYYMMDDHHMMSS>_<name>.sql`, `.notx.sql` for statements that must run
outside a transaction), embedded into the console binary. They are applied by
`cobra db migrate`, never by `atlas migrate apply`. `cobra db migrate:diff <name>`
loads the GORM models through `tools/schemaloader` (list every model managed by
migrations in `tools/schemaloader/main.go`). The command creates a private temporary
Atlas config and removes it on completion; no project `atlas.hcl` is needed.

The Framework default scratch database is disposable PostgreSQL 18 in Docker.
Override it through the existing application configuration when needed:

```yaml
MIGRATIONS:
  MIGRATIONS_DEV_URL: "docker://postgres/18/dev?search_path=public"
```

The `MIGRATIONS_DEV_URL` environment variable overrides YAML. Leave it unset to use
the Framework default; it is independent of DATABASE connection credentials and
must identify disposable scratch storage, never an application database. APP_ENV
(and the optional `--env` flag) labels the generated environment; separate HCL
environments are unnecessary. Atlas CLI and Docker (for the default dev URL) must
be available when running diff. Existing custom atlas.hcl files are not read or
modified by this command; transfer their dev setting to MIGRATIONS_DEV_URL.
Advanced custom Atlas projects can still be run directly through the Atlas CLI.

## Tenants

Schema-per-tenant resolution is the separate local library
`gitlab.com/skyrix-lib/tenant` (`/var/www/skyrix-tenant-lib`). The Template
does not enable it: no binary requires or imports the module, loads
`TENANT_CACHE`, or builds its cache or repository, and the default API chain is
authentication only. That is intentional. Tenant needs PostgreSQL (the
`tenants` table in the main schema) and Redis (the shared tenant cache).

The working example is kept in the framework repository, not in the Template:
`integration/testdata/tenant` (paths below are relative to it). The framework
integration harness (`integration/tenant_fixture_test.go`, `TestTenantFixture`)
overlays it on a copy of the Template, generates Wire, builds it and checks the
middleware. To enable tenants in a project made from the Template:

1. `go.mod`: require `gitlab.com/skyrix-lib/tenant` (for local development
   `v0.0.0` with `replace gitlab.com/skyrix-lib/tenant => ../../skyrix-tenant-lib`;
   no release is published yet).
2. Copy `providers/tenant.go` (`TenantSet` and `ProvideTenantAPIMiddleware`)
   and `internal/config/tenant.go` from the example into the same places; list the
   section in `config.Sections()`:
   `{Key: "tenant", Value: &TenantCache{}, YAMLPath: []string{"TENANT_CACHE"}, LegacyEnv: true}`.
   Optionally add `TENANT_CACHE: {TENANT_CACHE_TTL: 3m}` to the YAML files.
3. `cmd/http/wire.go`: add `providers.RedisSet` and `providers.TenantSet`, and
   replace `providers.ProvideAPIMiddleware` with
   `providers.ProvideTenantAPIMiddleware` (auth first, then tenant), as in
   the example's `cmd/http/wire.go`. Regenerate with `go tool wire gen ./cmd/http`.
4. The `tenants` table: add `&tenantEntity.Tenant{}` to
   `tools/schemaloader/main.go` and run `cobra db migrate:diff create_tenants`,
   then `cobra db migrate`; for development also to `internal/database.Models`
   (see the example's `internal/database/database.go` and
   `tools/schemaloader/main.go`). Create every tenant schema and its tables yourself:
   `db migrate` migrates only the main schema.

Behaviour: `X-Tenant` (a tenant identifier for the lookup, never a schema
name) is tried first, then `Host`. An unknown or inactive tenant is 404, a host
without tenant is 404, a failed lookup is 503, and a cancelled request stops.
Serving requests without any tenant identifier in the main schema is an
explicit policy (`middleware.Options{MainFallback: true}`). `X-Forwarded-Host`
is read only with `schemaResolver.Options{TrustForwardedHost: true}`, which
requires a proxy that overwrites the header. Resolving a tenant does not
authorize the caller for it: check membership in the application. CRUD runs in
the tenant schema through the gormdb plugin; Raw/Row/Scan and joins must be
schema-qualified or run in `WithinTransaction`. See the Tenant README.

## HTTP endpoints

- `GET /health` — health check of the registered dependencies (PostgreSQL)
- `GET /metrics` — Prometheus metrics
- `/api/v1/examples/tasks` — example domain (`POST /`, `GET /`,
  `GET /{task_id}`, `PATCH /{task_id}/complete`), requires a bearer token
  (see [Authentication](#authentication))

OpenAPI files are generated into `docs/` (Swagger UI is not mounted):

```bash
swag init -g cmd/http/main.go -o docs --parseDependency --parseInternal
```

## Code generation and tests

```bash
GOWORK=off go generate ./...                           # Wire injectors and providers/platform_gen.go
go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker  # Wire only
go test ./...
```

Optional `//skyrix:platform` directives in `providers/platform.go` generate
independent sets in `providers/platform_gen.go`, such as `EventbusplatformSet`.
Each driver supplies a concrete `Config`, `NewConfig() *Config` defaults and a
three-field `manifest.yaml` (`name`, `config_key`, `config_example`). Generated
config providers load only selected `config.Section` values through Template,
using the manifest key. Select only the required driver
sets in each binary's `wire.Build`. In `providers/platform.go`, add typed module
parameters to `ProvideHTTPPlatform` or `ProvideWorkerPlatform` and pass them to
`kernel.NewPlatform` in lifecycle order. No generated file needs editing.

Console commands are selected separately in `ProvideConsolePlatformCommands`
(`providers/console.go`), using zero-value driver command factories that must not
load config or construct/start infrastructure for help and completion. The
template ships with empty platforms and no driver commands. Factories return
`*console.Command`, registered with `Console.Add`; use `console.With` for lazy DI
and close the command graph in its owner. `config:env` includes declared driver
section defaults from `providers.PlatformSections()` without loading settings.

```sh
go run ./cmd/cobra platform list
go run ./cmd/cobra platform add <driver-package>
go run ./cmd/cobra platform remove <driver-package>
```

These standalone commands accept `--dir` (default `providers`). They update only
driver declarations and generated independent sets. Repeat adds/missing removals
print `No changes.` Invalid input leaves files unchanged. Add does not select a
driver for any application. Graphs, module providers and commands must be edited
explicitly; remove preserves user config, domains and Wire files. Add dependencies
with go get yourself, and regenerate Wire using your installed tool. Commands
never run go get/tidy or install tools. External EventBus needs the new driver
contract before it can be added; required changes are in the framework
[ARCHITECTURE.md](../ARCHITECTURE.md#platform-modules).

External Auth Signer/Validator constructors return errors. Generated Wire
propagates key-loading failures and cleans up acquired resources. Signer assigns
a fresh JTI; access revocation is opt-in.

The existing providers and domain layout is unchanged: several sets can share
one provider file, and domains keep their own `ProviderSet`. A driver imported
by the shared providers package can appear in `go list -deps` without being
constructed or started. Independent driver selection and generation repeatability
are covered by the framework integration harness
(`integration/platform_selection_test.go` in the framework repository).

## Layout

```text
cmd/http, cmd/cobra, cmd/worker   binaries and their Wire graphs
config/                           YAML configuration per APP_ENV
docs/                             generated OpenAPI files
entrypoints/                      container entrypoint
infra/go/                         Dockerfile and Dockerfile.debug
internal/commands/                application console commands
internal/config/                  configuration structure and loading
internal/database/                models and seeders for the db commands
internal/domain/example/          example domain
internal/handlers/                HTTP handlers
internal/jobs/                    background jobs
internal/middleware/              application HTTP middleware (rate limit stub)
internal/router/                  route composition
migrations/                       SQL migrations
providers/                        Wire provider sets
tools/schemaloader/               GORM schema for Atlas
```

## Scaffolding generators

Run `cobra make ...` from the project root (the directory containing go.mod).
Generators use only source files and stubs: no working configuration, connections
or application Wire graph is loaded. Module imports are read from go.mod.

```sh
cobra make domain sales/order_flow
cobra make entity sales/order_flow/line_item
cobra make repository sales/order_flow/line_item
cobra make service sales/order_flow/reconcile_batch
cobra make uow sales/order_flow/CreateOrder
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
| handler | BaseHandler-based HTTP skeleton in `internal/handlers`; adds constructor and Handlers field in `providers/handlers.go`; no route is added |
| command | console.Command factory in `internal/commands`; registers in `commands.All`, receiving the existing lazy application graph |
| command --db | Registers in `commands.DatabaseAll`, receiving the existing lazy database graph; help/completion never build it |
| job | Job implementation in `internal/jobs`; constructor in both JobProviderSet and ConsoleJobProviderSet, field in Jobs, and registration in ProvideRegisteredJobsRegistry |
| seeder | Idempotent no-op Seeder in `internal/database/seeders`; constructor in database.ProviderSet and parameter/entry in Seeders |
| migration | Versioned SQL file; preserves --notx and installed Atlas hashing behavior |

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

### Domain transactions

Example CreateTaskUOW uses gormdb.WithinTransaction and explicit repository
WithTx(*gormdb.Database). Services keep their business-only UoW interfaces.
`make uow order/CreateOrder` creates a domain constructor and Execute(ctx)
skeleton using the same mechanism; add repositories and business contracts
explicitly. The stub is exported/overridden as stubs/skyrix/uow.stub.

WithContext qualifies primary CRUD tables through the plugin; it does not set
search_path on pooled connections. Run unqualified Raw/Row/Scan and joins inside
WithinTransaction (checked SET LOCAL), or explicitly qualify SQL. Nested calls on
tx use savepoints, scope changes fail, and transaction contexts must be preserved.
See the Framework README for migration and raw TxManager limitations.
