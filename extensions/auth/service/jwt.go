package service

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"crypto/rsa"
	"errors"
	"os"

	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"
	"gitlab.com/skyrix-lib/framework/extensions/auth/security"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	JWTConfig  *Config
	Logger     logger.Interface
	// Blacklist and Sessions are checked by ValidateToken; when both are nil
	// ValidateToken only parses the token.
	Blacklist contracts.TokenBlacklist
	Sessions  contracts.SessionStore
}

// NewJWTService loads RSA keys and returns a JWT implementation backed by the
// token blacklist and session store.
func NewJWTService(logger logger.Interface, cfg *Config, blacklist contracts.TokenBlacklist, sessions contracts.SessionStore) *JWTService {
	privateKey, publicKey, err := loadKeys(cfg.PrivateKeyPath, cfg.PublicKeyPath)
	if err != nil {
		logger.Error("Failed to load JWT keys", "error", err)
		panic("JWTService critical error: " + err.Error())
	}
	return &JWTService{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		JWTConfig:  cfg,
		Logger:     logger,
		Blacklist:  blacklist,
		Sessions:   sessions,
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
	if j.Blacklist == nil && j.Sessions == nil {
		return j.ParseToken(tokenString)
	}
	if j.Blacklist != nil {
		if black, err := j.Blacklist.IsTokenBlacklisted(ctx, tokenString); err != nil || black {
			return nil, errors.New("invalid or expired token")
		}
	}
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.ID != "" && j.Sessions != nil {
		ok, err := j.Sessions.SessionExists(ctx, claims.ID)
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
