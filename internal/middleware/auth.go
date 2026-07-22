package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"skyrix/internal/config"
	authservice "skyrix/internal/engine/auth/service"
	"skyrix/internal/kernel/contextkeys"
	"skyrix/internal/logger"
	"skyrix/internal/utils/security"
)

type AuthIdentity struct {
	UserID  uint64
	Subject string
	Role    security.Role
}

type authIdentityContextKey struct{}

func WithAuthIdentity(ctx context.Context, identity AuthIdentity) context.Context {
	return context.WithValue(ctx, authIdentityContextKey{}, identity)
}

func AuthIdentityFromContext(ctx context.Context) (AuthIdentity, bool) {
	identity, ok := ctx.Value(authIdentityContextKey{}).(AuthIdentity)
	return identity, ok
}

func StoreIDFromIdentity(identity AuthIdentity) (string, error) {
	if identity.UserID == 0 {
		return "", errors.New("auth identity is required")
	}

	if storeID, ok := strings.CutPrefix(strings.TrimSpace(identity.Subject), "store:"); ok && strings.TrimSpace(storeID) != "" {
		return strings.TrimSpace(storeID), nil
	}

	return "", errors.New("store identity is required")
}

type AuthMiddleware struct {
	jwtVerifier *authservice.JWTService
	logger      logger.Interface
}

func NewAuthMiddleware(logger logger.Interface, cfg *config.JWT) *AuthMiddleware {
	return &AuthMiddleware{
		jwtVerifier: authservice.NewJWTVerifier(logger, cfg),
		logger:      logger,
	}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtVerifier.ValidateToken(r.Context(), tokenString)
		if err != nil {
			if m.logger != nil {
				m.logger.Warn("jwt validation failed", "error", err)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if claims.UserID <= 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		identity := AuthIdentity{
			UserID:  uint64(claims.UserID),
			Subject: claims.Subject,
			Role:    claims.Role,
		}
		ctx := WithAuthIdentity(r.Context(), identity)
		ctx = context.WithValue(ctx, contextkeys.UserClaimsContextKey, claims)
		ctx = context.WithValue(ctx, contextkeys.IDContextKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalized := normalizeRole(role)
		if normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := AuthIdentityFromContext(r.Context())
			if !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if _, ok := allowed[normalizeRole(string(identity.Role))]; !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func normalizeRole(role string) string {
	return strings.ToUpper(strings.TrimSpace(role))
}

func bearerToken(authHeader string) (string, error) {
	if strings.TrimSpace(authHeader) == "" {
		return "", errors.New("missing authorization header")
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}
