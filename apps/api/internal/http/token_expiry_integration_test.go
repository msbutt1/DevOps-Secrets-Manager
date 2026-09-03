package http_test

import (
	"net/http"
	"testing"
	"time"
)

func TestTokenSecretsIncludeExpiry(t *testing.T) {
	f := newFixture(t) // DATABASE_URL without expiry
	expired := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/secrets", f.owner.Token, map[string]any{
		"key_name": "OLD_API_KEY", "value": "still-readable", "expires_at": expired.Format(time.RFC3339),
	})

	var tok struct {
		Token string `json:"token"`
	}
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token, map[string]any{"name": "ci"}).Decode(t, &tok)
	var body struct {
		Secrets []struct {
			Key       string     `json:"key"`
			Value     string     `json:"value"`
			ExpiresAt *time.Time `json:"expires_at"`
		} `json:"secrets"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/token/secrets", tok.Token, nil).Decode(t, &body)

	found := map[string]*time.Time{}
	for _, s := range body.Secrets {
		found[s.Key] = s.ExpiresAt
		if s.Key == "OLD_API_KEY" && s.Value != "still-readable" {
			t.Errorf("expired value not returned: %q", s.Value)
		}
	}
	if got, ok := found["OLD_API_KEY"]; !ok || got == nil || !got.Equal(expired) {
		t.Errorf("OLD_API_KEY expires_at: %v", got)
	}
	if got, ok := found["DATABASE_URL"]; !ok || got != nil {
		t.Errorf("DATABASE_URL expires_at should be null, got %v", got)
	}
}
