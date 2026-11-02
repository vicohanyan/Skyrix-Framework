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
- extensions — `extensions/auth` (JWT, sessions, token stores) and
  `extensions/tenant` (schema-per-tenant resolution);
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
network provider. ConfigSet alone does not enable infrastructure. Existing
`config=Field` platformgen generation is unchanged; future driver sections must
be selected explicitly by their composition/provider until platformgen migration.
No global registry, config:env command, driver manifest or platform management
command is introduced.
