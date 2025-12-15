package schemaResolver

import "errors"

var (
	ErrTenantHeaderMissing = errors.New("tenant header missing")
	ErrTenantInvalid       = errors.New("invalid tenant")
	ErrTenantNotFound      = errors.New("tenant not found")

	ErrHostEmpty          = errors.New("empty host")
	ErrTenantNotFoundHost = errors.New("tenant not found by domain")

	ErrSchemaInvalid = errors.New("invalid schema")
)
