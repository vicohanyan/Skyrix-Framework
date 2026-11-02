// Package engine is the root of the framework's core infrastructure modules.
// The root package holds no code; the modules live in subpackages:
//
//   - engine/postgres: PostgreSQL configuration, database/sql pool and the
//     lifecycle component (no GORM);
//   - engine/postgres/gormdb: GORM on top of engine/postgres — the
//     schema-aware Database, the schema-routing plugin and transactions;
//   - engine/redis: Redis connection, key namespaces and the lifecycle
//     component;
//   - engine/scope: the request data scope (schema) carried in context;
//   - engine/jobs: the background job registry and runner.
//
// engine/... never imports extensions/... (see internal/depcheck).
package engine
