package security

import (
	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 12

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	return string(b), err
}

func CheckPasswordHash(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
