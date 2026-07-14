# Skyrix Delivery

Delivery service for the Skyrix platform. The service exposes customer, store,
delivery-admin, and super-admin HTTP APIs, consumes delivery events from NATS
JetStream, and manages orders, couriers, route zones, batches, offers, and
delivery jobs.

## Stack

- Go 1.26
- Chi HTTP router
- PostgreSQL 18 with PostGIS and GORM
- Redis
- NATS JetStream
- Google Wire for compile-time dependency injection
- Cobra CLI
- Prometheus metrics
- Swagger/OpenAPI

## Requirements

- Go 1.26+ for native development
- Docker and Docker Compose for the containerized environment
- An existing Docker network named `skyrix-network`
- A NATS server reachable as `nats:4222` on that network
- An RSA public key at `secret/jwt_public.pem` for local JWT validation

The local Compose file starts the application, PostgreSQL, and Redis. NATS is
part of the shared Skyrix infrastructure and is not defined in this repository.

## Quick start with Docker

Create the shared network once, if it does not already exist:

```bash
docker network create skyrix-network
```

Build and start the local stack:

```bash
docker compose up -d --build
docker compose ps
curl http://localhost:7540/health
```

The service is available at `http://localhost:7540`. PostgreSQL is exposed on
`127.0.0.1:5433`; Redis is available only inside `skyrix-network`.

Stop the stack with:

```bash
docker compose down
```

To also remove local PostgreSQL and Redis data:

```bash
docker compose down -v
```

## Debugging with Delve

The debug override builds the application with optimizations disabled and
starts it under Delve:

```bash
docker compose -f docker-compose.yaml -f docker-compose.debug.yaml up -d --build
```

Attach a debugger to `127.0.0.1:2345`. The HTTP API remains available on port
7540.

## Native development

Download dependencies and build both binaries:

```bash
go mod download
go build -o delivery ./cmd/http
go build -o cobra ./cmd/console
```

Configuration is loaded from `/config/<APP_ENV>.yaml` first and then from
`config/<APP_ENV>.yaml`. `APP_ENV` defaults to `local`. Environment variables
override values from the YAML file.

Running the service natively requires reachable PostgreSQL, Redis, NATS, and a
valid path in `JWT_PUBLIC_KEY_PATH`:

```bash
APP_ENV=local go run ./cmd/http
```

## Configuration

- `config/local.yaml` contains local-development defaults.
- `config/prod.yaml` contains the production configuration template.
- Environment variables use the same leaf names as the YAML settings, such as
  `APP_PORT`, `DB_HOST`, `REDIS_HOST`, `QUEUE_HOST`, and
  `JWT_PUBLIC_KEY_PATH`.

Do not commit production credentials or private keys. The checked-in local
values are development defaults only.

## Database setup and CLI

The Docker image contains the CLI at `/app/cobra`. Show all available commands:

```bash
docker compose exec app /app/cobra --help
```

Common commands:

```bash
# Run schema migrations and PostgreSQL-specific indexes
docker compose exec app /app/cobra db automigrate

# Alias for db automigrate
docker compose exec app /app/cobra db migrate

# Seed required dictionary data
docker compose exec app /app/cobra db seed

# List or run registered background jobs
docker compose exec app /app/cobra jobs:run --help

# Import delivery zones from GeoJSON/uMap FeatureCollection
docker compose exec app /app/cobra route-zones:import \
  /app/data/route-zones/ijevan_route_zones.geojson

# Initialize EventBus scaffolding and manage transport modules
./cobra eventbus init
./cobra eventbus create Payment
./cobra eventbus list
./cobra eventbus remove Payment

# Create business-domain scaffolding
./cobra make domain Payment
./cobra make repository Payment
./cobra make service Payment
```

The migration command applies GORM migrations, required PostgreSQL extensions,
and partial indexes. Delivery-zone import requires PostGIS.

## HTTP endpoints

System endpoints:

- `GET /health` — health check
- `GET /metrics` — Prometheus metrics

Business APIs are mounted under `/api/v1`:

- `/customer/delivery`
- `/store/delivery`
- `/delivery/admin`
- `/superadmin/delivery`

Most business routes require JWT authentication and the appropriate role or
tenant context. Generated OpenAPI files are stored in `docs/`.

Regenerate them with:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/http/main.go -d ./ \
  -o docs --parseDependency
```

The application currently does not mount Swagger UI; use `docs/swagger.yaml` or
`docs/swagger.json` with an OpenAPI viewer.

## Code generation and tests

Regenerate Wire injectors after changing providers:

```bash
go install github.com/google/wire/cmd/wire@latest
wire ./cmd/http
wire ./cmd/console
```

Run the test suite:

```bash
go test ./...
```

NATS protobuf bindings can be regenerated when `protoc` and
`protoc-gen-go` are installed:

```bash
./scripts/gen-nats-proto.sh
```

## Production Compose

`docker-compose.prod.yaml` expects:

- `IMAGE_DELIVERY` with the published application image
- `JWT_PUBLIC_KEY_PATH` with the host path to the public key
- database and service values in `.env`
- the external `skyrix-network`

Start it with:

```bash
docker compose -f docker-compose.prod.yaml up -d
```

Run one-off CLI commands through the tools profile:

```bash
docker compose -f docker-compose.prod.yaml --profile tools run --rm \
  delivery-console db migrate
```

## Repository layout

```text
cmd/http/                 HTTP service entrypoint and Wire injector
cmd/console/              Cobra CLI entrypoint and Wire injector
config/                   environment-specific YAML configuration
data/route-zones/         route-zone import data
docs/                     generated OpenAPI files
entrypoints/              container entrypoint scripts
infra/go/                 local, debug, and production Dockerfiles
infra/postgres/           PostgreSQL/PostGIS image
internal/adapter/         external service adapters
internal/domain/          delivery domain modules
internal/engine/          shared runtime infrastructure
internal/handlers/        HTTP handlers
internal/jobs/            background jobs
internal/router/          HTTP route composition
internal/transport/       event transport and generated contracts
scripts/                  development and generation scripts
```

## Delivery coverage and dispatch

- Customer coordinates come from the Catalog event address snapshot; the
  service does not geocode textual delivery addresses during order creation.
- Missing or invalid coordinates produce `ADDRESS_UNRESOLVED`; valid coordinates
  outside all active zones produce `OUT_OF_DELIVERY_ZONE`; matches produce
  `IN_DELIVERY_ZONE` and persist the resolved zone and coordinates.
- Automatic dispatch only batches orders with `IN_DELIVERY_ZONE` coverage and
  groups them by delivery zone and delivery method.
- Courier assignment belongs to a delivery batch, not directly to an order.


## API DOCUMENTATION

The API is documented using Swagger.

Swagger UI:
```text
http://localhost:7070/swagger/index.html
```

Regenerate Swagger documentation:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
swag init -g cmd/http/main.go -d ./ -o docs --parseDependency
```

Note:
- this project uses `cmd/http/main.go` as the HTTP entrypoint
- install the Swagger generator first if `swag` is not available:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

==================================================

## CONFIGURATION

Environment configuration lives in the `config/` directory.

Current local configuration:
```text
config/local.yaml
```

Database connection settings can be modified in the configuration files.
Add environment-specific files such as `config/prod.yaml` when production deployment configuration is introduced.

==================================================

## REBUILDING COMPONENTS

Common rebuild/regeneration commands:

```bash
wire ./cmd/http
wire ./cmd/console
```

```bash
export PATH=$PATH:$(go env GOPATH)/bin
swag init -g cmd/http/main.go -d ./ -o docs --parseDependency
```

==================================================
