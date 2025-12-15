package contextkeys

import (
	"context"
	"errors"
)

// ContextKey is a type-safe key for context values
// Exported so it can be used across packages
// Example: context.WithValue(ctx, contextkeys.TenantContextKey, value)
type ContextKey string

const (
	DBContextKey         ContextKey = "db"
	TenantContextKey     ContextKey = "tenant"
	IDContextKey         ContextKey = "id"
	UserClaimsContextKey ContextKey = "user_claims"
	UserTypeContextKey   ContextKey = "user_type"
)

func GetCustomerIDFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(IDContextKey)
	if val == nil {
		return 0, errors.New("user id not found in context")
	}

	id, ok := val.(int64)
	if !ok || id <= 0 {
		return 0, errors.New("invalid user id in context")
	}

	return id, nil
}
