package crypto

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestGenerateDEK(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK failed: %v", err)
	}

	if len(dek) != 32 {
		t.Errorf("Expected DEK length 32, got %d", len(dek))
	}

	// Generate another DEK and ensure they're different
	dek2, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK failed on second call: %v", err)
	}

	if bytes.Equal(dek, dek2) {
		t.Error("Two generated DEKs should not be identical")
	}
}

func TestEncryptDecryptDEK(t *testing.T) {
	// Generate KEK and DEK
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Encrypt DEK
	encryptedDEK, err := EncryptDEK(dek, kek)
	if err != nil {
		t.Fatalf("Failed to encrypt DEK: %v", err)
	}

	// Verify encrypted DEK is longer than original (includes nonce and auth tag)
	if len(encryptedDEK) <= len(dek) {
		t.Error("Encrypted DEK should be longer than original DEK")
	}

	// Decrypt DEK
	decryptedDEK, err := DecryptDEK(encryptedDEK, kek)
	if err != nil {
		t.Fatalf("Failed to decrypt DEK: %v", err)
	}

	// Verify decrypted DEK matches original
	if !bytes.Equal(dek, decryptedDEK) {
		t.Error("Decrypted DEK does not match original DEK")
	}
}

func TestDecryptDEKWithWrongKEK(t *testing.T) {
	// Generate two different KEKs and a DEK
	kek1 := make([]byte, 32)
	kek2 := make([]byte, 32)
	for i := range kek1 {
		kek1[i] = byte(i)
		kek2[i] = byte(i + 1)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Encrypt with kek1
	encryptedDEK, err := EncryptDEK(dek, kek1)
	if err != nil {
		t.Fatalf("Failed to encrypt DEK: %v", err)
	}

	// Attempt to decrypt with kek2 (wrong KEK)
	_, err = DecryptDEK(encryptedDEK, kek2)
	if err == nil {
		t.Error("Expected decryption to fail with wrong KEK, but it succeeded")
	}
}

func TestLoadKEKFromEnv(t *testing.T) {
	// Test with valid 32-byte hex-encoded KEK
	t.Run("ValidKEK", func(t *testing.T) {
		validKEK := make([]byte, 32)
		for i := range validKEK {
			validKEK[i] = byte(i)
		}
		hexKEK := hex.EncodeToString(validKEK)

		os.Setenv("MASTER_KEK", hexKEK)
		defer os.Unsetenv("MASTER_KEK")

		kek, err := LoadKEKFromEnv()
		if err != nil {
			t.Fatalf("LoadKEKFromEnv failed: %v", err)
		}

		if len(kek) != 32 {
			t.Errorf("Expected KEK length 32, got %d", len(kek))
		}

		if !bytes.Equal(kek, validKEK) {
			t.Error("Loaded KEK does not match expected value")
		}
	})

	// Test with missing environment variable
	t.Run("MissingEnvVar", func(t *testing.T) {
		os.Unsetenv("MASTER_KEK")

		_, err := LoadKEKFromEnv()
		if err == nil {
			t.Error("Expected error when MASTER_KEK is not set, but got none")
		}
	})

	// Test with invalid hex encoding
	t.Run("InvalidHex", func(t *testing.T) {
		os.Setenv("MASTER_KEK", "not-valid-hex")
		defer os.Unsetenv("MASTER_KEK")

		_, err := LoadKEKFromEnv()
		if err == nil {
			t.Error("Expected error for invalid hex encoding, but got none")
		}
	})

	// Test with wrong length (16 bytes instead of 32)
	t.Run("WrongLength", func(t *testing.T) {
		shortKEK := make([]byte, 16)
		hexKEK := hex.EncodeToString(shortKEK)

		os.Setenv("MASTER_KEK", hexKEK)
		defer os.Unsetenv("MASTER_KEK")

		_, err := LoadKEKFromEnv()
		if err == nil {
			t.Error("Expected error for wrong KEK length, but got none")
		}
	})
}

func TestDecryptDEKWithTooShortInput(t *testing.T) {
	kek := make([]byte, 32)
	shortInput := []byte{1, 2, 3}

	_, err := DecryptDEK(shortInput, kek)
	if err == nil {
		t.Error("Expected error when encrypted DEK is too short, but got none")
	}
}
