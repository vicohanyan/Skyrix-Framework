package service

import (
	"strings"

	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
)

// Config configures JWT signing and verification. The yaml and env tags let
// an application embed it in its configuration (type JWT = service.Config).
type Config struct {
	PublicKeyPath  string `yaml:"JWT_PUBLIC_KEY_PATH" env:"JWT_PUBLIC_KEY_PATH"`
	PrivateKeyPath string `yaml:"JWT_PRIVATE_KEY_PATH" env:"JWT_PRIVATE_KEY_PATH"`
	// Expiration is the token lifetime in hours for issuers (security.NewClaims).
	Expiration int    `yaml:"JWT_EXPIRE" env:"JWT_EXPIRE" env-default:"24"`
	Algorithm  string `yaml:"JWT_ALGORITHM" env:"JWT_ALGORITHM"`
	Issuer     string `yaml:"JWT_ISSUER" env:"JWT_ISSUER"`
	Audience   string `yaml:"JWT_AUDIENCE" env:"JWT_AUDIENCE"`
	// AllowedRoles are the token roles the auth middleware lets through, for
	// example [customer] or JWT_ALLOWED_ROLES=customer,staff. Empty allows
	// every role.
	AllowedRoles []string `yaml:"JWT_ALLOWED_ROLES" env:"JWT_ALLOWED_ROLES" env-separator:","`
}

// Roles returns AllowedRoles as roles, trimmed, without empty entries.
func (c Config) Roles() []security.Role {
	roles := make([]security.Role, 0, len(c.AllowedRoles))
	for _, role := range c.AllowedRoles {
		if role = strings.TrimSpace(role); role != "" {
			roles = append(roles, security.Role(role))
		}
	}
	return roles
}
