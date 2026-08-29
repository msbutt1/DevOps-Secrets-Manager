package http_test

import (
	"net/http"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	f := newFixture(t)
	for _, resp := range []struct {
		name string
		r    interface{ Get(string) string }
	}{
		{"reveal", f.api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil).Header},
		{"list", f.api.MustDo(http.StatusOK, "GET", "/envs/"+f.envID+"/secrets", f.owner.Token, nil).Header},
		{"error", f.api.MustDo(http.StatusUnauthorized, "GET", "/vaults", "", nil).Header},
	} {
		want := map[string]string{
			"Cache-Control":           "no-store",
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "no-referrer",
			"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		}
		for header, value := range want {
			if got := resp.r.Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", resp.name, header, got, value)
			}
		}
	}
}
