package middleware

import (
	"context"
	"net/http"
	"skyrix/internal/engine/auth/service" // Changed import and aliased
	"skyrix/internal/kernel/contextkeys"
	"skyrix/internal/logger"
	"skyrix/internal/utils/security"
	"strings"
)

type AuthMiddleware struct {
	jwtService *service.JWTService // Changed type
	logger     logger.Interface
}

func NewAuthMiddleware(jwtService *service.JWTService, logger logger.Interface) *AuthMiddleware { // Changed type
	return &AuthMiddleware{
		jwtService: jwtService,
		logger:     logger,
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

		if claims.Role != security.RoleCustomer || claims.UserID <= 0 {
			http.Error(w, "customer token required", http.StatusForbidden)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextkeys.UserClaimsContextKey, claims)
		ctx = context.WithValue(ctx, contextkeys.IDContextKey, claims.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
