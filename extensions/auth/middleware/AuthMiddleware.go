package middleware

import (
	"net/http"
	"slices"
	"strings"

	authcontext "gitlab.com/skyrix-lib/framework/extensions/auth/context"
	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
	"gitlab.com/skyrix-lib/framework/extensions/auth/service"
	"gitlab.com/skyrix-lib/framework/logger"
)

// Options configures AuthMiddleware.
type Options struct {
	// Roles are the token roles allowed through; empty allows every role.
	Roles []security.Role
}

// AuthMiddleware validates the bearer token (signature, blacklist, session)
// and stores the claims in the request context (auth/context).
type AuthMiddleware struct {
	jwtService *service.JWTService
	logger     logger.Interface
	roles      []security.Role
}

// OptionsFromConfig allows the roles of cfg.AllowedRoles.
func OptionsFromConfig(cfg *service.Config) Options {
	if cfg == nil {
		return Options{}
	}
	return Options{Roles: cfg.Roles()}
}

func NewAuthMiddleware(jwtService *service.JWTService, logger logger.Interface, opts Options) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		logger:     logger,
		roles:      slices.Clone(opts.Roles),
	}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := m.jwtService.ValidateToken(r.Context(), tokenString)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		if claims.UserID <= 0 || (len(m.roles) > 0 && !slices.Contains(m.roles, claims.Role)) {
			http.Error(w, "token role is not allowed", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r.WithContext(authcontext.WithClaims(r.Context(), claims)))
	})
}
