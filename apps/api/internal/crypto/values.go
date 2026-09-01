package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptValue encrypts a secret value with a vault data key using AES-256-GCM and a random
// 12-byte nonce, which is returned separately.
func EncryptValue(plaintext, dek []byte) (ciphertext, nonce []byte, err error) {
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// DecryptValue reverses EncryptValue. It fails if the ciphertext, nonce or key is wrong,
// because GCM authenticates the data.
func DecryptValue(ciphertext, nonce, dek []byte) ([]byte, error) {
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("failed to decrypt: nonce must be %d bytes", gcm.NonceSize())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}
