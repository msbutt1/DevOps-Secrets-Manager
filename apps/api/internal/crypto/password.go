// Package crypto provides cryptographic utilities for the application.
package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plaintext password using bcrypt with the default cost.
// Returns the bcrypt hash as a string, or an error if hashing fails.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword compares a plaintext password with a bcrypt hash.
// Returns nil if the password matches the hash, or an error otherwise.
func VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
