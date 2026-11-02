package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/extensions/auth/middleware"
	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
	"gitlab.com/skyrix-lib/framework/extensions/auth/service"

	"github.com/golang-jwt/jwt/v5"
)

// TestAuthMiddlewareRoles documents the role rules: an empty JWT_ALLOWED_ROLES
// lets every role through but still requires a valid token with a user ID;
// tokens without a user ID are rejected by the verifier (401); a non-empty
// list rejects other roles with 403.
func TestAuthMiddlewareRoles(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwtService := &service.JWTService{PublicKey: &key.PublicKey, JWTConfig: &service.Config{Algorithm: "RS256"}}
	token := func(userID int64, role string) string {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, security.CustomClaims{
			UserID: userID,
			Role:   security.Role(role),
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + signed
	}

	tests := []struct {
		name          string
		allowedRoles  []string
		authorization string
		want          int
	}{
		{name: "empty list, no token", want: http.StatusUnauthorized},
		{name: "empty list, invalid token", authorization: "Bearer invalid", want: http.StatusUnauthorized},
		{name: "empty list, any role", authorization: token(42, "anything"), want: http.StatusNoContent},
		{name: "empty list, no user id", authorization: token(0, "customer"), want: http.StatusUnauthorized},
		{name: "list, allowed role", allowedRoles: []string{"customer"}, authorization: token(42, "customer"), want: http.StatusNoContent},
		{name: "list, other role", allowedRoles: []string{"customer"}, authorization: token(42, "admin"), want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := middleware.OptionsFromConfig(&service.Config{AllowedRoles: test.allowedRoles})
			handler := middleware.NewAuthMiddleware(jwtService, nil, options).Handle(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

// TestAuthMiddlewareScheme checks that the Bearer scheme is matched
// case-insensitively (RFC 7235) and that other schemes, a missing token and
// whitespace around the token are rejected with 401.
func TestAuthMiddlewareScheme(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwtService := &service.JWTService{PublicKey: &key.PublicKey, JWTConfig: &service.Config{Algorithm: "RS256"}}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, security.CustomClaims{
		UserID: 42,
		Role:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		authorization string
		want          int
	}{
		{name: "Bearer", authorization: "Bearer " + signed, want: http.StatusNoContent},
		{name: "bearer", authorization: "bearer " + signed, want: http.StatusNoContent},
		{name: "BEARER", authorization: "BEARER " + signed, want: http.StatusNoContent},
		{name: "mixed case", authorization: "bEaReR " + signed, want: http.StatusNoContent},
		{name: "basic scheme", authorization: "Basic " + signed, want: http.StatusUnauthorized},
		{name: "scheme prefix only", authorization: "Bearertoken " + signed, want: http.StatusUnauthorized},
		{name: "scheme without token", authorization: "Bearer", want: http.StatusUnauthorized},
		{name: "scheme with empty token", authorization: "Bearer ", want: http.StatusUnauthorized},
		{name: "lowercase scheme with empty token", authorization: "bearer ", want: http.StatusUnauthorized},
		{name: "spaces only after scheme", authorization: "Bearer   ", want: http.StatusUnauthorized},
		{name: "extra space before token", authorization: "Bearer  " + signed, want: http.StatusUnauthorized},
		{name: "token without scheme", authorization: signed, want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := middleware.NewAuthMiddleware(jwtService, nil, middleware.Options{}).Handle(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			request.Header.Set("Authorization", test.authorization)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	for _, test := range []struct {
		header string
		token  string
		ok     bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"BEARER abc", "abc", true},
		{"Bearer a b", "a b", true},
		{"Bearer  abc", " abc", true},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"", "", false},
		{strings.Repeat("x", 8), "", false},
	} {
		token, ok := middleware.BearerToken(test.header)
		if token != test.token || ok != test.ok {
			t.Errorf("BearerToken(%q) = %q, %v; want %q, %v", test.header, token, ok, test.token, test.ok)
		}
	}
}
