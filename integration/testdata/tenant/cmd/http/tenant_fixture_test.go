package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/auth/security"

	"github.com/golang-jwt/jwt/v5"
)

// TestTenantMiddlewareInGraph serves requests through the router built by
// the generated Wire graph. PostgreSQL and Redis point at a closed port, so
// any tenant lookup fails: the answer shows which middleware ran.
func TestTenantMiddlewareInGraph(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(publicKey, pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)}), 0o600); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"APP_ENV": "tenant-fixture", "JWT_PUBLIC_KEY_PATH": publicKey, "JWT_ALGORITHM": "RS256",
		"DB_HOST": "127.0.0.1", "DB_PORT": "1", "DB_NAME": "app", "DB_USER": "app",
		"REDIS_HOST": "127.0.0.1", "REDIS_PORT": "1", "REDIS_KEY_PREFIX": "fixture",
		"TENANT_CACHE_TTL": "1m", "LOG_OUTPUT": "stdout", "LOG_LEVEL": "error",
	} {
		t.Setenv(k, v)
	}
	handler, cleanup, err := buildHTTPHandler()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, security.CustomClaims{
		UserID:           42,
		Role:             security.Role("customer"),
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	serve := func(path, token, tenant string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://unknown.test"+path, http.NoBody)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if tenant != "" {
			req.Header.Set("X-Tenant", tenant)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	tenantVary := func(rec *httptest.ResponseRecorder) bool {
		return strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "X-Tenant")
	}

	// System routes are outside the API chain: no tenant resolution.
	if rec := serve("/metrics", "", "acme"); rec.Code != http.StatusOK || tenantVary(rec) {
		t.Fatalf("/metrics: %d, Vary %v", rec.Code, rec.Header().Values("Vary"))
	}
	if rec := serve("/health", "", "acme"); tenantVary(rec) || rec.Header().Get("X-Tenant-Resolved-By") != "" {
		t.Fatalf("/health went through tenant resolution: %d", rec.Code)
	}
	// Auth runs first: without a token there is no tenant lookup.
	if rec := serve("/api/v1/examples/tasks/", "", "acme"); rec.Code != http.StatusUnauthorized || tenantVary(rec) {
		t.Fatalf("without token: %d, Vary %v", rec.Code, rec.Header().Values("Vary"))
	}
	// Then tenant: the lookup fails (database unavailable) with 503, never
	// the main schema, with or without a tenant header.
	for _, tenant := range []string{"acme", ""} {
		rec := serve("/api/v1/examples/tasks/", signed, tenant)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "TENANT_LOOKUP_FAILED") || !tenantVary(rec) {
			t.Fatalf("tenant %q: %d %s, Vary %v", tenant, rec.Code, rec.Body.String(), rec.Header().Values("Vary"))
		}
	}
	// An invalid identifier is rejected before any lookup.
	if rec := serve("/api/v1/examples/tasks/", signed, "not-valid"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid tenant: %d", rec.Code)
	}
}
