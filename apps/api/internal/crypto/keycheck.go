package crypto

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// MinJWTSecretLength is the shortest HS256 signing secret the server accepts.
const MinJWTSecretLength = 32

// minDistinctSymbols rejects keys such as "0000...0000" or "abababab...".
const minDistinctSymbols = 8

// knownWeakKeys holds placeholders and values that have been published in this
// repository's examples. A server configured with any of them is not secure.
var knownWeakKeys = map[string]struct{}{
	"change-me": {},
	"changeme":  {},
	"secret":    {},
	"your-secret-key-change-this-in-production":                        {},
	"your-jwt-secret-min-32-chars":                                     {},
	"32-byte-hex-encoded-key":                                          {},
	"f9564fdfaedd5b5b16b069ecdbfa8c7ae30280e4b6872663d1b9926e1a2bdbd8": {},
	"3670641f8e1fe98863fd675f5388d4481c695a313ca7549cb12468aebdef63e0": {},
}

func isKnownWeakKey(value string) bool {
	_, ok := knownWeakKeys[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func distinctSymbols[T comparable](values []T) int {
	seen := make(map[T]struct{}, len(values))
	for _, v := range values {
		seen[v] = struct{}{}
	}
	return len(seen)
}

// ParseKEK decodes a hex-encoded 32-byte master key and rejects placeholder,
// published or obviously low-entropy values.
func ParseKEK(hexKEK string) ([]byte, error) {
	if strings.TrimSpace(hexKEK) == "" {
		return nil, fmt.Errorf("MASTER_KEK is not set; generate one with: openssl rand -hex 32")
	}
	if isKnownWeakKey(hexKEK) {
		return nil, fmt.Errorf("MASTER_KEK is a placeholder or published example value; generate a new one with: openssl rand -hex 32")
	}

	kek, err := hex.DecodeString(hexKEK)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MASTER_KEK: %w", err)
	}

	if len(kek) != 32 {
		return nil, fmt.Errorf("MASTER_KEK must be 32 bytes, got %d bytes", len(kek))
	}

	if distinctSymbols(kek) < minDistinctSymbols {
		return nil, fmt.Errorf("MASTER_KEK has too little variation to be a random key; generate one with: openssl rand -hex 32")
	}

	return kek, nil
}

// ValidateJWTSecret rejects missing, short, placeholder or low-entropy signing secrets.
func ValidateJWTSecret(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("APP_JWT_SECRET is not set; generate one with: openssl rand -hex 32")
	}
	if isKnownWeakKey(secret) {
		return fmt.Errorf("APP_JWT_SECRET is a placeholder or published example value; generate a new one with: openssl rand -hex 32")
	}
	if len(secret) < MinJWTSecretLength {
		return fmt.Errorf("APP_JWT_SECRET must be at least %d characters, got %d", MinJWTSecretLength, len(secret))
	}
	if distinctSymbols([]rune(secret)) < minDistinctSymbols {
		return fmt.Errorf("APP_JWT_SECRET has too little variation to be a random secret")
	}
	return nil
}
