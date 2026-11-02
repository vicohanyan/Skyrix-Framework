// Package context carries the authenticated token claims in a request
// context.
package context

import (
	"context"

	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
)

type ctxKey string

const ctxClaimsKey ctxKey = "auth_claims"

// WithClaims returns ctx carrying the token claims.
func WithClaims(ctx context.Context, claims *security.CustomClaims) context.Context {
	return context.WithValue(ctx, ctxClaimsKey, claims)
}

// ClaimsFrom returns the claims stored by WithClaims.
func ClaimsFrom(ctx context.Context) (*security.CustomClaims, bool) {
	claims, ok := ctx.Value(ctxClaimsKey).(*security.CustomClaims)
	return claims, ok && claims != nil
}

// UserIDFrom returns the user ID of the claims; ok is false when there are no
// claims or the ID is not positive.
func UserIDFrom(ctx context.Context) (int64, bool) {
	claims, ok := ClaimsFrom(ctx)
	if !ok || claims.UserID <= 0 {
		return 0, false
	}
	return claims.UserID, true
}
