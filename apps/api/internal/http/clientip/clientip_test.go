package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	r, err := NewResolver([]string{"10.0.0.0/8", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, remote string
		xff          []string
		want         string
	}{
		{"direct client ignores headers", "203.0.113.9:5000", []string{"1.2.3.4"}, "203.0.113.9"},
		{"trusted proxy forwards client", "10.0.0.5:80", []string{"198.51.100.7"}, "198.51.100.7"},
		{"spoofed left-most entry is ignored", "10.0.0.5:80", []string{"6.6.6.6, 198.51.100.7"}, "198.51.100.7"},
		{"chain of trusted proxies", "127.0.0.1:80", []string{"198.51.100.7, 10.1.2.3"}, "198.51.100.7"},
		{"multiple headers", "10.0.0.5:80", []string{"6.6.6.6", "198.51.100.7"}, "198.51.100.7"},
		{"garbage stops the walk", "10.0.0.5:80", []string{"not-an-ip"}, "10.0.0.5"},
		{"no header from trusted proxy", "127.0.0.1:1234", nil, "127.0.0.1"},
		{"ipv6 client", "[2001:db8::1]:443", nil, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			for _, h := range tt.xff {
				req.Header.Add("X-Forwarded-For", h)
			}
			if got := r.ClientIP(req); got != tt.want {
				t.Fatalf("want %s, got %s", tt.want, got)
			}
		})
	}

	if _, err := NewResolver([]string{"nonsense"}); err == nil {
		t.Fatal("invalid proxy range should be rejected")
	}
}

func TestClientIPFromEdgeHeader(t *testing.T) {
	resolver, err := NewResolverWithHeader([]string{"10.0.0.0/8"}, "CF-Connecting-IP")
	if err != nil {
		t.Fatal(err)
	}
	request := func(remote string, headers map[string]string) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remote
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req
	}

	// From a trusted proxy the edge header wins, even when the client prepends a fake hop
	got := resolver.ClientIP(request("10.1.2.3:4567", map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
		"X-Forwarded-For":  "1.2.3.4, 203.0.113.9",
	}))
	if got != "203.0.113.9" {
		t.Errorf("edge header ignored: got %s", got)
	}

	// A spoofed edge header from an untrusted peer is ignored
	got = resolver.ClientIP(request("198.51.100.7:1234", map[string]string{"CF-Connecting-IP": "1.2.3.4"}))
	if got != "198.51.100.7" {
		t.Errorf("untrusted peer chose its own address: got %s", got)
	}

	// Without the header, X-Forwarded-For is still used
	got = resolver.ClientIP(request("10.1.2.3:4567", map[string]string{"X-Forwarded-For": "203.0.113.10"}))
	if got != "203.0.113.10" {
		t.Errorf("fallback to X-Forwarded-For failed: got %s", got)
	}

	// Garbage in the header falls back rather than returning nonsense
	got = resolver.ClientIP(request("10.1.2.3:4567", map[string]string{
		"CF-Connecting-IP": "not-an-ip",
		"X-Forwarded-For":  "203.0.113.11",
	}))
	if got != "203.0.113.11" {
		t.Errorf("invalid edge header not ignored: got %s", got)
	}
}
