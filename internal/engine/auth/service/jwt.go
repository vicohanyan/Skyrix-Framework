package service

import (
	"context"
	"crypto/rsa"
	"errors"
	"os"
	"skyrix/internal/config"
	"skyrix/internal/engine/auth/contracts"
	"skyrix/internal/logger"
	"skyrix/internal/utils/security"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	JWTConfig  *config.JWT
	Logger     logger.Interface
	Store      contracts.Store
}

// NewJWTService loads RSA keys and returns a JWT implementation backed by Redis store.
func NewJWTService(logger logger.Interface, cfg *config.JWT, store contracts.Store) *JWTService {
	privateKey, publicKey, err := loadKeys(cfg.PrivateKeyPath, cfg.PublicKeyPath)
	if err != nil {
		logger.Error("Failed to load JWT keys", err)
		panic("JWTService critical error: " + err.Error())
	}
	return &JWTService{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		JWTConfig:  cfg,
		Logger:     logger,
		Store:      store,
	}
}

// GenerateToken creates a signed JWT using configured RSA algorithm and custom claims.
func (j *JWTService) GenerateToken(claims security.CustomClaims) (string, error) {
	method, err := j.getSigningMethod()
	if err != nil {
		return "", err
	}
	tok := jwt.NewWithClaims(method, claims)
	return tok.SignedString(j.PrivateKey)
}

// ParseToken validates the signature and decodes CustomClaims from the token string.
func (j *JWTService) ParseToken(tokenString string) (*security.CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &security.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.PublicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*security.CustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ValidateToken checks blacklist status, verifies signature, and confirms session existence.
func (j *JWTService) ValidateToken(ctx context.Context, tokenString string) (*security.CustomClaims, error) {
	if black, err := j.Store.IsTokenBlacklisted(ctx, tokenString); err != nil || black {
		return nil, errors.New("invalid or expired token")
	}
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.ID != "" {
		ok, err := j.Store.SessionExists(ctx, claims.ID)
		if err != nil || !ok {
			return nil, errors.New("session not found")
		}
	}

	return claims, nil
}

// getSigningMethod maps config string to a concrete RSA signing method.
func (j *JWTService) getSigningMethod() (jwt.SigningMethod, error) {
	switch j.JWTConfig.Algorithm {
	case "RS256":
		return jwt.SigningMethodRS256, nil
	case "RS384":
		return jwt.SigningMethodRS384, nil
	case "RS512":
		return jwt.SigningMethodRS512, nil
	default:
		return nil, errors.New("unsupported signing algorithm")
	}
}

// loadKeys reads RSA private/public keys from disk for JWT signing/verification.
func loadKeys(privateKeyPath, publicKeyPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	// Load private key
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, nil, err
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
	if err != nil {
		return nil, nil, err
	}

	// Load public key
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}
