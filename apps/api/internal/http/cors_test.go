package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowlist(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := corsMiddleware([]string{"https://app.example.com/", " https://admin.example.com"})(ok)

	do := func(method, origin string, preflight bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/vaults", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if preflight {
			req.Header.Set("Access-Control-Request-Method", "POST")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	res := do("GET", "https://app.example.com", false)
	if res.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" || res.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed origin missing CORS headers: %v", res.Header())
	}

	res = do("OPTIONS", "https://admin.example.com", true)
	if res.Code != http.StatusNoContent || res.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("preflight from allowed origin: %d %v", res.Code, res.Header())
	}

	res = do("GET", "https://evil.example", false)
	if res.Header().Get("Access-Control-Allow-Origin") != "" || res.Code != http.StatusOK {
		t.Fatalf("disallowed origin got CORS headers: %v", res.Header())
	}
	if res = do("OPTIONS", "https://evil.example", true); res.Code != http.StatusForbidden {
		t.Fatalf("disallowed preflight should be 403, got %d", res.Code)
	}
	if res = do("GET", "", false); res.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("same-origin request should not get CORS headers")
	}

	none := corsMiddleware(nil)(ok)
	req := httptest.NewRequest("OPTIONS", "/vaults", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	none.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("no CORS headers expected without configured origins")
	}
}
