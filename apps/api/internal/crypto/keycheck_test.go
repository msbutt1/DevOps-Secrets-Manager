package crypto

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestParseKEK(t *testing.T) {
	random := make([]byte, 32)
	for i := range random {
		random[i] = byte(i*37 + 11)
	}

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "valid random key", value: hex.EncodeToString(random)},
		{name: "missing", value: "", wantErr: "not set"},
		{name: "whitespace", value: "   ", wantErr: "not set"},
		{name: "placeholder", value: "change-me", wantErr: "placeholder"},
		{name: "old example value", value: "f9564fdfaedd5b5b16b069ecdbfa8c7ae30280e4b6872663d1b9926e1a2bdbd8", wantErr: "placeholder"},
		{name: "old example value uppercase", value: "F9564FDFAEDD5B5B16B069ECDBFA8C7AE30280E4B6872663D1B9926E1A2BDBD8", wantErr: "placeholder"},
		{name: "not hex", value: "zz", wantErr: "decode"},
		{name: "too short", value: hex.EncodeToString(random[:16]), wantErr: "32 bytes"},
		{name: "all zeros", value: strings.Repeat("00", 32), wantErr: "variation"},
		{name: "repeating pattern", value: strings.Repeat("0102", 16), wantErr: "variation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kek, err := ParseKEK(tt.value)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected key to be accepted, got %v", err)
				}
				if len(kek) != 32 {
					t.Fatalf("expected 32-byte key, got %d", len(kek))
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "random hex secret", value: "5b1f0e3c9a7d42b8e6f01c2d3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b"},
		{name: "long passphrase", value: "correct horse battery staple and then some"},
		{name: "missing", value: "", wantErr: "not set"},
		{name: "placeholder", value: "change-me", wantErr: "placeholder"},
		{name: "config example placeholder", value: "your-secret-key-change-this-in-production", wantErr: "placeholder"},
		{name: "old example value", value: "3670641f8e1fe98863fd675f5388d4481c695a313ca7549cb12468aebdef63e0", wantErr: "placeholder"},
		{name: "too short", value: "a1b2c3d4e5f6a7b8", wantErr: "at least 32"},
		{name: "low variation", value: strings.Repeat("ab", 20), wantErr: "variation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJWTSecret(tt.value)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected secret to be accepted, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
