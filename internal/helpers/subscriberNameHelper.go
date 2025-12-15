package helpers

import (
	"context"
	"skyrix/internal/kernel/contextkeys"
)

// GetSubscriberNameFromContext retrieves the subscriber name from the context.
func GetSubscriberNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(contextkeys.TenantContextKey).(string)
	return name, ok
}
