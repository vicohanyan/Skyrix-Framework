package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/skyrix-lib/framework/extensions/auth/security"
)

func TestConstructorsReturnErrors(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.pem")
	if err := os.WriteFile(invalid, []byte("invalid RSA PEM"), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(dir, "private.pem")
	if err := os.WriteFile(private, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(dir, "public.pem")
	if err := os.WriteFile(public, pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		cfg     *Config
		missing bool
	}{
		{"nil", nil, false},
		{"missing private", &Config{PrivateKeyPath: filepath.Join(dir, "missing"), PublicKeyPath: public}, true},
		{"invalid private", &Config{PrivateKeyPath: invalid, PublicKeyPath: public}, false},
		{"missing public", &Config{PrivateKeyPath: private, PublicKeyPath: filepath.Join(dir, "missing")}, true},
		{"invalid public", &Config{PrivateKeyPath: private, PublicKeyPath: invalid}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := NewJWTService(noopJWTLogger{}, test.cfg, nil, nil)
			if value != nil || err == nil {
				t.Fatalf("service=%v error=%v", value, err)
			}
			if test.missing && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lost cause: %v", err)
			}
		})
	}
	for _, path := range []string{invalid, filepath.Join(dir, "missing")} {
		value, err := NewJWTVerifier(noopJWTLogger{}, &Config{PublicKeyPath: path})
		if value != nil || err == nil {
			t.Fatalf("verifier=%v error=%v", value, err)
		}
	}
	if value, err := NewJWTVerifier(noopJWTLogger{}, nil); value != nil || err == nil {
		t.Fatal("nil config accepted")
	}
	for _, algorithm := range []string{"RS256", "RS384", "RS512"} {
		cfg := &Config{PrivateKeyPath: private, PublicKeyPath: public, Algorithm: algorithm}
		signer, err := NewJWTService(noopJWTLogger{}, cfg, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		verifier, err := NewJWTVerifier(noopJWTLogger{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		claims := validTestClaims()
		token, err := signer.GenerateToken(claims)
		if err != nil {
			t.Fatal(err)
		}
		got, err := verifier.ParseToken(token)
		if err != nil || got.UserID != claims.UserID {
			t.Fatalf("round trip %s: %v %v", algorithm, got, err)
		}
		if _, err := verifier.GenerateToken(security.CustomClaims{}); err == nil {
			t.Fatal("verifier signed token")
		}
	}
}
