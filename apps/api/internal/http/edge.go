package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// EdgeTokenHeader carries the shared secret that proves a request came through the edge.
const EdgeTokenHeader = "X-Edge-Token"

// edgeAuth refuses requests that did not arrive through the edge in front of the API.
//
// A platform origin such as https://name.onrender.com or https://name.fly.dev stays publicly
// reachable whatever the DNS for the public hostname says, so without this anyone can address
// the API directly and skip Cloudflare entirely: its rate limiting, its WAF rules, and the
// CF-Connecting-IP header that the audit log and the per-IP limits believe. The proxy in
// apps/web/functions/api adds the header from a secret only it and the API hold.
//
// The answer is 404 rather than 401, for the same reason the rest of the API answers 404 for
// resources the caller cannot see: an unauthorized caller learns nothing about what is here.
// /health is exempt, because the platform's own health check cannot send the header.
func edgeAuth(token string) func(http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			// Hashing first keeps the comparison constant time over a token of any length.
			got := sha256.Sum256([]byte(r.Header.Get(EdgeTokenHeader)))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				writeError(w, http.StatusNotFound, "not_found", "Not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
