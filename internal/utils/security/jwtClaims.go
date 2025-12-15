package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleStaff      Role = "staff"
	RoleCustomer   Role = "customer"
)

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
