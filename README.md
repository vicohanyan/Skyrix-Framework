# Framework

Reusable application engine, kernel, Cobra infrastructure, logging, database
integration, and tenant extension for Go services.

The runnable service structure is kept in [`template/`](template/) until it is
moved to its own repository.

## Compatibility

| Framework | Go | Wire | EventBus |
|---|---|---|---|
| v0.3.x | 1.27+ | `github.com/goforj/wire` | v1.1.0+ |
| v0.2.x | 1.26+ | `github.com/google/wire` | v1.0.x |

v0.3.0 is not backward compatible with v0.2.x, and there is no compatibility
layer. Services and modules must be migrated to v0.3.0 explicitly; modules
released for v0.3.0 do not support v0.2.x. See [CHANGELOG.md](CHANGELOG.md)
for the list of breaking changes.
