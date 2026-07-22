package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skyrix/internal/config"
	"skyrix/internal/utils/security"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddlewareMissingAuthorization(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler was called")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareMalformedAuthorization(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Basic abc")

	mw.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler was called")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareInvalidBearerToken(t *testing.T) {
	mw, _ := newTestAuthMiddleware(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")

	mw.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler was called")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareValidTokenAddsIdentity(t *testing.T) {
	mw, privateKey := newTestAuthMiddleware(t)
	tokenString := signMiddlewareToken(t, privateKey, 42)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer "+tokenString)

	mw.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := AuthIdentityFromContext(r.Context())
		if !ok {
			t.Fatalf("AuthIdentityFromContext() ok = false, want true")
		}
		if identity.UserID != 42 || identity.Subject != "user:42" {
			t.Fatalf("identity = %+v, want user_id 42 subject user:42", identity)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestStoreIDFromIdentityRejectsMissingStoreSubject(t *testing.T) {
	_, err := StoreIDFromIdentity(AuthIdentity{UserID: 42, Subject: "user:42", Role: security.RoleStoreOperator})
	if err == nil {
		t.Fatal("StoreIDFromIdentity() error = nil, want missing store identity error")
	}
}

func TestStoreIDFromIdentityUsesCatalogStoreSubject(t *testing.T) {
	storeID, err := StoreIDFromIdentity(AuthIdentity{UserID: 42, Subject: "store:store-one", Role: security.RoleStoreManager})
	if err != nil {
		t.Fatalf("StoreIDFromIdentity() error = %v", err)
	}
	if storeID != "store-one" {
		t.Fatalf("StoreIDFromIdentity() = %q, want store-one", storeID)
	}
}

func newTestAuthMiddleware(t *testing.T) (*AuthMiddleware, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})
	publicKeyPath := filepath.Join(t.TempDir(), "jwt_public.pem")
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return NewAuthMiddleware(noopAuthLogger{}, &config.JWT{
		PublicKeyPath: publicKeyPath,
		Algorithm:     "RS256",
		Issuer:        "skyrix-auth",
	}), privateKey
}

func signMiddlewareToken(t *testing.T, privateKey *rsa.PrivateKey, userID int64) string {
	t.Helper()

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, security.CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "skyrix-auth",
			Subject:   "user:42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return tokenString
}

type noopAuthLogger struct{}

func (noopAuthLogger) Error(msg string, keysAndValues ...interface{}) {}
func (noopAuthLogger) Info(msg string, keysAndValues ...interface{})  {}
func (noopAuthLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (noopAuthLogger) Warn(msg string, keysAndValues ...interface{})  {}
