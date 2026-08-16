package http_test

import (
	"net/http"
	"testing"
)

func revealValue(t *testing.T, f *fixture, secretID string) string {
	t.Helper()
	var out struct {
		Value string `json:"value"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secretID+"/reveal", f.owner.Token, nil).Decode(t, &out)
	return out.Value
}

func TestUpdatingMetadataKeepsSecretValue(t *testing.T) {
	f := newFixture(t)

	// The web form omits the value when the user only edits the description.
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"description": "Primary database"})
	if got := revealValue(t, f, f.secretID); got != "postgres://fixture" {
		t.Fatalf("value changed to %q by a metadata-only update", got)
	}

	// An explicit new value, including an empty one, is still applied.
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"value": "postgres://rotated"})
	if got := revealValue(t, f, f.secretID); got != "postgres://rotated" {
		t.Fatalf("value not updated, got %q", got)
	}
}
