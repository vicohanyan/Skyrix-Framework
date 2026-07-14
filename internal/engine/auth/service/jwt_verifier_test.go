package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skyrix/internal/config"
	"skyrix/internal/utils/security"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTVerifierValidToken(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	tokenString := signTestClaims(t, privateKey, security.CustomClaims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "skyrix-auth",
			Subject:   "user:42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, jwt.SigningMethodRS256)

	claims, err := verifier.ParseToken(tokenString)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
}

func TestJWTVerifierRejectsWrongKey(t *testing.T) {
	_, verifier := newTestJWTVerifier(t, "skyrix-auth")
	wrongPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	tokenString := signTestClaims(t, wrongPrivateKey, validTestClaims(), jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierRejectsWrongAlg(t *testing.T) {
	_, verifier := newTestJWTVerifier(t, "skyrix-auth")
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, validTestClaims()).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierRejectsExpiredToken(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	claims := validTestClaims()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierRejectsMissingExp(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	claims := validTestClaims()
	claims.ExpiresAt = nil
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierRejectsWrongIssuer(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	claims := validTestClaims()
	claims.Issuer = "other-issuer"
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierAcceptsTokenWithoutAudienceWhenAudienceNotConfigured(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	claims := validTestClaims()
	claims.Audience = nil
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err != nil {
		t.Fatalf("ParseToken() error = %v, want nil", err)
	}
}

func TestJWTVerifierRejectsWrongAudienceWhenAudienceConfigured(t *testing.T) {
	privateKey, verifier := newTestJWTVerifierWithAudience(t, "skyrix-auth", "skyrix-platform")
	claims := validTestClaims()
	claims.Audience = []string{"other-platform"}
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func TestJWTVerifierRejectsMissingUserID(t *testing.T) {
	privateKey, verifier := newTestJWTVerifier(t, "skyrix-auth")
	claims := validTestClaims()
	claims.UserID = 0
	tokenString := signTestClaims(t, privateKey, claims, jwt.SigningMethodRS256)

	if _, err := verifier.ParseToken(tokenString); err == nil {
		t.Fatalf("ParseToken() error = nil, want error")
	}
}

func validTestClaims() security.CustomClaims {
	return security.CustomClaims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "skyrix-auth",
			Subject:   "user:42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func newTestJWTVerifier(t *testing.T, issuer string) (*rsa.PrivateKey, *JWTService) {
	t.Helper()
	return newTestJWTVerifierWithAudience(t, issuer, "")
}

func newTestJWTVerifierWithAudience(t *testing.T, issuer string, audience string) (*rsa.PrivateKey, *JWTService) {
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

	verifier := NewJWTVerifier(noopJWTLogger{}, &config.JWT{
		PublicKeyPath: publicKeyPath,
		Algorithm:     "RS256",
		Issuer:        issuer,
		Audience:      audience,
	})
	return privateKey, verifier
}

func signTestClaims(t *testing.T, privateKey *rsa.PrivateKey, claims security.CustomClaims, method jwt.SigningMethod) string {
	t.Helper()

	tokenString, err := jwt.NewWithClaims(method, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return tokenString
}

type noopJWTLogger struct{}

func (noopJWTLogger) Error(msg string, keysAndValues ...interface{}) {}
func (noopJWTLogger) Info(msg string, keysAndValues ...interface{})  {}
func (noopJWTLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (noopJWTLogger) Warn(msg string, keysAndValues ...interface{})  {}
