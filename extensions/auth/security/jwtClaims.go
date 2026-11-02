package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

// Role is the role claim of a token. The framework defines no roles: they
// come from the token issuer and are configured by the application, for
// example service.Config.AllowedRoles (JWT_ALLOWED_ROLES).
type Role string

type CustomClaims struct {
	UserID int64  `json:"user_id"`
	Tenant string `json:"tenant,omitempty"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

func NewClaims(uid int64, role Role, tenant string, iss string, aud []string, expHours int) CustomClaims {
	return CustomClaims{
		UserID: uid,
		Tenant: tenant,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Audience:  aud,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expHours) * time.Hour)),
			Subject:   string(role),
			ID:        ulid.Make().String(),
		},
	}
}
