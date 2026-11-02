# extensions/tenant

Schema-per-tenant resolution for HTTP requests. The extension finds the tenant
of a request (by header or domain), and stores its PostgreSQL schema in the
request context with `engine/scope`; `engine/postgres/gormdb` then runs the
request's queries in that schema. The engine and the kernel do not know this
extension (see [ARCHITECTURE.md](../../ARCHITECTURE.md)).

## Packages

| Package          | Contents                                                                                                   |
|------------------|------------------------------------------------------------------------------------------------------------|
| `entity`         | `Tenant`, the GORM model of the `tenants` table (a `scope.Main` model: it lives in the main schema).        |
| `repository`     | Read access: `GetByNamespace`, `GetByDomain`, `ListDomains` (active tenants with a domain, for CORS).       |
| `service`        | `TenantService`: in-memory cache, optional shared cache (`service.Cache`), then the repository; only active tenants with a schema are returned. |
| `cache`          | `RedisCache`, the shared cache on `engine/redis` under `<REDIS_KEY_PREFIX>:tenant:namespace:<ns>` and `...:tenant:domain:<domain>` (TTL 3 minutes by default). |
| `schemaResolver` | Resolvers by header (`X-Tenant` by default) and by domain (`X-Forwarded-Host`, else `Host`), tried in order (`header`, `domain` by default). |
| `middleware`     | `TenantMiddleware` (resolves the schema, stores it with `scope.WithSchema`), `CorsMiddleware` (allows the origins of the tenant domains). |
| `context`        | How the tenant was resolved (`header`, `domain` or `default`).                                             |

`TenantMiddleware` sets `X-Tenant-Resolved-By`. When no resolver succeeds and
the last error is a missing header, an empty host or an unknown domain, the
request runs in the main schema (`default`); other errors are answered with a
JSON error (400 or 404). The template wires the extension in
`providers.TenantProviderSet` (it needs `RedisSet`); no template binary
includes it by default.

## The `tenants` table belongs to the application

The extension provides the model, not the table: it ships no migrations and
never creates, changes or deletes tenants. The application that uses the
extension owns the `tenants` table in its main schema:

1. add the model to the application's Atlas schema loader,
   `tools/schemaloader/main.go`:

   ```go
   tenantEntity "gitlab.com/skyrix-lib/framework/extensions/tenant/entity"
   // ...
   statements, err := gormschema.New("postgres").Load(
   	&exampleEntity.Task{},
   	&tenantEntity.Tenant{},
   )
   ```

2. generate the migration into the application's `migrations/` directory with
   `cobra db migrate:diff <name>` (Atlas) and apply it with `cobra db migrate`.

For development, `cobra db automigrate` creates the table from the model when
`entity.Tenant` is listed in `internal/database.Models` (the template lists
it). Creating the per-tenant schemas and their tables is also up to the
application.

Availability is expressed by `is_active` and `active_to`; there is no soft
delete and no `deleted_at` column.

### Index names

The unique indexes keep the names `ux_tenant_alive`,
`uniq_subscriber_schema_nz` and `uniq_subscriber_domain_nz` on purpose: they
come from the services that created `tenants` before the extension, and
AutoMigrate finds indexes by name, so existing databases keep their indexes
and get no duplicates. PostgreSQL unique indexes ignore NULL, so `schema` and
`domain` are unique only when set.

## Known limitation: Raw, Row and Scan

`gormdb.Plugin` qualifies table names with the request schema in the GORM
query, create, update and delete callbacks. Statements that bypass them —
`Raw`, `Row`, `Rows` and `Scan` (including `Model(...).Select(...).Scan(...)`)
— are not qualified and rely only on the `search_path` that
`gormdb.Database.WithContext` sets (`<schema>, <main>, public`). Write such
statements with this in mind: an unqualified table resolves to the request
schema first, then to the main schema. `repository.ListDomains` uses `Scan`:
it reads the main schema when the context carries no tenant schema.
