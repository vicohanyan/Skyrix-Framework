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

`providers` also defines `RedisSet`, `AuthProviderSet` and
`TenantProviderSet`; no binary includes them by default. Add a set to a
binary's `wire.go` when it needs Redis, the auth stores or the tenant
middleware, and add its lifecycle component to the binary's runtime.

## Authentication

The HTTP binary protects the API with the framework bearer-token middleware
(`extensions/auth/middleware`), wired by `providers.AuthVerifierSet`:

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

The handlers read the claims with `extensions/auth/context`.
`AuthVerifierSet` checks only the token itself; to also reject blacklisted
tokens and tokens without a session, use `providers.AuthProviderSet` instead
(it needs `RedisSet`).

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
| `DATABASE`     | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_MAIN_SCHEMA`               |
| `REDIS`        | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASS`, `REDIS_DB`, `REDIS_KEY_PREFIX`              |
| `JWT`          | `JWT_PUBLIC_KEY_PATH`, `JWT_ALGORITHM`, `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ALLOWED_ROLES`, ... |
| `TENANT_CACHE` | `TENANT_CACHE_TTL`                                                                    |

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
cobra make domain|repository|service <Name>
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
`database`, `redis`, `auth` and `tenant`; unknown keys return an error. Secret
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

`migrations/` holds forward-only SQL migrations in the Atlas directory format
(`<YYYYMMDDHHMMSS>_<name>.sql`, `.notx.sql` for statements that must run
outside a transaction), embedded into the console binary. They are applied by
`cobra db migrate`, never by `atlas migrate apply`. `atlas.hcl` and
`tools/schemaloader` let Atlas diff the GORM models (list every model managed
by migrations in `tools/schemaloader/main.go`).

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
go generate ./...                                      # Wire injectors and providers/platform_gen.go
go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker  # Wire only
go test ./...
```

Platform modules (platform drivers) are declared with `//skyrix:platform`
comments in `providers/platform.go`; `go generate` regenerates
`providers/platform_gen.go` with `framework/cmd/platformgen`. The template
ships without platform modules, so `PlatformSet` is empty. EventBus (NATS) is
connected as a platform driver: `providers/platform.go` lists the steps.

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
