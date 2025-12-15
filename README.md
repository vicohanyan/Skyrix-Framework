# Skyrix Framework

Skyrix Framework is a pragmatic, modular backend framework written in Golang for building scalable APIs and multi-tenant systems.

Primary development is hosted on GitLab.
GitHub is used as a read-only mirror and template source.

==================================================

## WHAT YOU GET

- HTTP API foundation (Chi router, middleware, structured handlers)
- Configuration loading via Cleanenv
- Request validation (go-playground/validator)
- JWT-based authentication foundation
- PostgreSQL database layer (GORM)
- Redis caching layer
- ULID-based identifiers and versioning
- CLI console (Cobra)
- Compile-time dependency injection (Google Wire)
- Production-oriented defaults with minimal magic

==================================================

## TECH STACK

- Go 1.25
- Chi (github.com/go-chi/chi/v5)
- Google Wire
- Cobra
- Cleanenv
- go-playground/validator
- JWT (github.com/golang-jwt/jwt/v5)
- PostgreSQL + GORM
- Redis
- golang.org/x/crypto
- ULID

==================================================

## REPOSITORY LAYOUT
```
cmd/            # entrypoints (http, console, workers)
internal/
app/          # application composition (core / kernel)
config/       # config schemas and loading
engine/       # reusable engine modules
router/       # HTTP router composition
handlers/     # application layer
providers/    # Wire providers
config/         # yaml / env configs
docs/           # architecture and ADRs
LICENSE
NOTICE
```
==================================================

## QUICK START

Requirements:
- Go 1.25+
- Docker & Docker Compose (optional)

Install dependencies:
```bash
go mod download
```

Generate dependency injection:
```bash
wire ./cmd/http
wire ./cmd/console
```
==================================================

## CLI CONSOLE

Skyrix Framework provides a CLI console pattern (similar to Laravel / Symfony) for:

- database migrations
- background jobs
- maintenance tasks
- development helpers

Example:
```bash
./cobra --help
./cobra hello
```
==================================================

## DEPENDENCY INJECTION

This framework uses compile-time dependency injection via Google Wire.

Install Wire:
```bash
go install github.com/google/wire/cmd/wire@latest
```

Generate DI code:
```bash
wire ./cmd/http
wire ./cmd/console
```

Guidelines:
- Providers live in internal/providers
- Injectors live in cmd/*/wire.go
- Avoid runtime service locators

==================================================

## DOCKER (LOCAL + DEBUG)

This repository provides two Docker build modes:

- Local/Run mode: build and run the HTTP service normally
- Debug mode: run the HTTP service under Delve (dlv) with port 2345 exposed

Files:
- Dockerfile — local/run build (builds /app/delivery and /app/cobra)
- Dockerfile.debug — debug build with -N -l flags and Delve installed
- docker-compose.example.yml — example compose stack (app + postgres + redis)
- docker-compose.override.yml — optional override to run app via Delve

Run (normal):
```bash
docker compose -f docker-compose.example.yml up -d --build
```

HTTP service:
- http://localhost:6060

Run (debug):
Make sure docker-compose.override.yml exists in the same directory, then run:
```bash
docker compose -f docker-compose.example.yml up -d --build
```

Alternatively, specify both files explicitly:
```bash
docker compose -f docker-compose.example.yml -f docker-compose.override.yml up -d --build
```

Debug ports:
- HTTP: http://localhost:6060
- Delve: localhost:2345

Stop:
```bash
docker compose -f docker-compose.example.yml down
```

Rebuild:
```bash
docker compose -f docker-compose.example.yml up -d --build
```

Notes:
- In debug mode, the app is started via:
  dlv exec /app/delivery --headless --listen=:2345 --api-version=2 --accept-multiclient --log
- Postgres is exposed on localhost:54333 (container port 5432).
- Redis is exposed on the default port 6379 if you add a port mapping (optional).

==================================================

## CONVENTIONS

- Explicit boundaries between engine and application layers
- Reusable, dependency-light engine modules
- Explicit configuration, routing, and middleware
- Production-first design (graceful shutdown, context propagation)

==================================================

## LICENSE

Licensed under the Apache License 2.0.
See the LICENSE file for details.

==================================================

## GIT HOSTING MODEL

- Primary repository: GitLab
- Mirror / template: GitHub
