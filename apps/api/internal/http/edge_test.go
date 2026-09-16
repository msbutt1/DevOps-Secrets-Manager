package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEdgeAuth(t *testing.T) {
	const token = "s3cret-edge-token"

	reached := false
	handler := edgeAuth(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		path   string
		header string
		send   bool
		status int
		reach  bool
	}{
		{"the edge is let through", "/vaults", token, true, http.StatusOK, true},
		{"a direct caller is not", "/vaults", "", false, http.StatusNotFound, false},
		{"an empty token is not", "/vaults", "", true, http.StatusNotFound, false},
		{"a wrong token is not", "/vaults", "s3cret-edge-tokeN", true, http.StatusNotFound, false},
		{"a prefix of the token is not", "/vaults", "s3cret", true, http.StatusNotFound, false},
		{"health is exempt for the platform check", "/health", "", false, http.StatusOK, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reached = false
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.send {
				req.Header.Set(EdgeTokenHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if reached != tc.reach {
				t.Errorf("handler reached = %v, want %v", reached, tc.reach)
			}
		})
	}
}
