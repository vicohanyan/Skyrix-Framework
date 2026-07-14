package service

import (
	"errors"

	"skyrix/internal/config"
	"skyrix/internal/logger"
	"skyrix/internal/utils/security"

	"github.com/golang-jwt/jwt/v5"
)

var ErrMissingUserIDClaim = errors.New("missing user_id claim")

// NewJWTVerifier loads only the public RSA key and returns a verifier-only JWT service.
func NewJWTVerifier(logger logger.Interface, cfg *config.JWT) *JWTService {
	publicKey, err := loadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		logger.Error("Failed to load JWT public key", err)
		panic("JWT verifier critical error: " + err.Error())
	}
	return &JWTService{
		PublicKey: publicKey,
		JWTConfig: cfg,
		Logger:    logger,
	}
}

// ParseToken validates the signature and decodes CustomClaims from the token string.
func (j *JWTService) ParseToken(tokenString string) (*security.CustomClaims, error) {
	parserOptions := []jwt.ParserOption{
		jwt.WithValidMethods([]string{j.JWTConfig.Algorithm}),
		jwt.WithExpirationRequired(),
	}
	if j.JWTConfig.Issuer != "" {
		parserOptions = append(parserOptions, jwt.WithIssuer(j.JWTConfig.Issuer))
	}
	if j.JWTConfig.Audience != "" {
		parserOptions = append(parserOptions, jwt.WithAudience(j.JWTConfig.Audience))
	}

	token, err := jwt.ParseWithClaims(tokenString, &security.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil {
			return nil, errors.New("missing signing method")
		}
		if token.Method.Alg() != j.JWTConfig.Algorithm {
			return nil, errors.New("unexpected signing method")
		}
		return j.PublicKey, nil
	}, parserOptions...)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*security.CustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.UserID <= 0 {
		return nil, ErrMissingUserIDClaim
	}

	return claims, nil
}
