package middleware

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"net/http"
	"slices"
	"strings"

	authcontext "gitlab.com/skyrix-lib/framework/extensions/auth/context"
	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
	"gitlab.com/skyrix-lib/framework/extensions/auth/service"
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

		tokenString, ok := BearerToken(authHeader)
		if !ok {
			http.Error(w, "invalid Authorization header format", http.StatusUnauthorized)
			return
		}

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

// BearerToken returns the token of an Authorization header value of the form
// "<scheme> <token>" whose scheme is Bearer. The scheme is compared
// case-insensitively (RFC 7235): "Bearer", "bearer" and "BEARER" are
// accepted. Scheme and token are separated by the first space; everything
// after it is the token, unchanged (extra whitespace is not trimmed and makes
// the token invalid). It reports false for another scheme, a missing
// separator or an empty token.
func BearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || token == "" || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}
