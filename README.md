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

	log := logger.NewLogger(logger.Config{LogOutput: "stdout", LogType: "text", LogLevel: "info"})

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
