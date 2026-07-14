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
	if j.PrivateKey == nil {
		return "", errors.New("jwt private key is not configured")
	}
	method, err := j.getSigningMethod()
	if err != nil {
		return "", err
	}
	tok := jwt.NewWithClaims(method, claims)
	return tok.SignedString(j.PrivateKey)
}

// ValidateToken checks blacklist status, verifies signature, and confirms session existence.
func (j *JWTService) ValidateToken(ctx context.Context, tokenString string) (*security.CustomClaims, error) {
	if j.Store == nil {
		return j.ParseToken(tokenString)
	}
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
	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, publicKey, nil
}

func loadPublicKey(publicKeyPath string) (*rsa.PublicKey, error) {
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
}
