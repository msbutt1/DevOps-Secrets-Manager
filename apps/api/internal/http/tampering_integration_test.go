package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestTamperedCiphertextIsRejected changes stored ciphertext directly in the database, as an
// attacker with write access to the database but not the master key could, and checks that the
// API refuses to return a value instead of returning corrupted or foreign data.
func TestTamperedCiphertextIsRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	flipped := f.createSecret(t, f.envID, "FLIPPED", "original-value")
	if _, err := f.api.Pool.Exec(ctx, `UPDATE secrets SET encrypted_value = set_byte(encrypted_value, 0, get_byte(encrypted_value, 0) # 1) WHERE id = $1`, flipped); err != nil {
		t.Fatal(err)
	}
	resp := f.api.MustDo(http.StatusInternalServerError, "POST", "/secrets/"+flipped+"/reveal", f.owner.Token, nil)
	if strings.Contains(string(resp.Body), "original-value") {
		t.Fatal("tampered secret leaked its value")
	}

	// Copying ciphertext from a secret in another vault (another data key) does not decrypt.
	otherVault := f.createVault(t, "other-vault")
	otherEnv := f.createEnv(t, otherVault, "production")
	foreign := f.createSecret(t, otherEnv, "FOREIGN", "foreign-value")
	target := f.createSecret(t, f.envID, "TARGET", "target-value")
	if _, err := f.api.Pool.Exec(ctx, `
		UPDATE secrets t SET encrypted_value = s.encrypted_value, nonce = s.nonce
		FROM secrets s WHERE t.id = $1 AND s.id = $2`, target, foreign); err != nil {
		t.Fatal(err)
	}
	resp = f.api.MustDo(http.StatusInternalServerError, "POST", "/secrets/"+target+"/reveal", f.owner.Token, nil)
	if strings.Contains(string(resp.Body), "foreign-value") {
		t.Fatal("ciphertext moved from another vault decrypted")
	}

	// The untouched secret in the same vault is still readable.
	if got := revealValue(t, f, f.secretID); got != "postgres://fixture" {
		t.Fatalf("untouched secret: %q", got)
	}

	// A tampered wrapped data key makes the whole vault unreadable rather than wrong.
	if _, err := f.api.Pool.Exec(ctx, `UPDATE vaults SET encrypted_dek = set_byte(encrypted_dek, 20, get_byte(encrypted_dek, 20) # 1) WHERE id = $1`, f.vaultID); err != nil {
		t.Fatal(err)
	}
	f.api.MustDo(http.StatusInternalServerError, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)
}
