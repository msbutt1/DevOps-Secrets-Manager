package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// GenerateDEK generates a 32-byte AES-256 Data Encryption Key
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}
	return dek, nil
}

// EncryptDEK encrypts a Data Encryption Key with a Key Encryption Key using AES-256-GCM
// The nonce is prepended to the ciphertext
func EncryptDEK(dek, kek []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, dek, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// DecryptDEK decrypts an encrypted Data Encryption Key using a Key Encryption Key
// Expects the nonce to be prepended to the ciphertext
func DecryptDEK(encryptedDEK, kek []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedDEK) < nonceSize {
		return nil, fmt.Errorf("encrypted DEK is too short")
	}

	nonce := encryptedDEK[:nonceSize]
	ciphertext := encryptedDEK[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	return plaintext, nil
}

// LoadKEKFromEnv reads the Master Key Encryption Key from the MASTER_KEK environment variable
// Expects a hex-encoded 32-byte key
func LoadKEKFromEnv() ([]byte, error) {
	hexKEK := os.Getenv("MASTER_KEK")
	if hexKEK == "" {
		return nil, fmt.Errorf("MASTER_KEK environment variable is not set")
	}

	kek, err := hex.DecodeString(hexKEK)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MASTER_KEK: %w", err)
	}

	if len(kek) != 32 {
		return nil, fmt.Errorf("MASTER_KEK must be 32 bytes, got %d bytes", len(kek))
	}

	return kek, nil
}
