package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/clientip"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
)

// rateLimiter builds per-route limits. When disabled (integration tests) it returns no-op middleware.
type rateLimiter struct {
	disabled bool
}

type keyFunc = httprate.KeyFunc

// limit allows n requests per window for each key, answering 429 with Retry-After beyond that.
func (rl rateLimiter) limit(name string, n int, window time.Duration, key keyFunc) func(http.Handler) http.Handler {
	if rl.disabled {
		return func(next http.Handler) http.Handler { return next }
	}
	message := "Too many requests. Try again in " + window.String() + "."
	return httprate.LimitBy(n, window,
		func(r *http.Request) (string, error) {
			k, err := key(r)
			return name + ":" + k, err
		},
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			// httprate has already set Retry-After
			writeError(w, http.StatusTooManyRequests, "rate_limited", message)
		}),
	)
}

// byIP keys on the client address resolved behind trusted proxies.
func byIP(r *http.Request) (string, error) {
	if ip := clientip.FromContext(r.Context()); ip != "" {
		return httprate.CanonicalizeIP(ip), nil
	}
	return httprate.KeyByIP(r)
}

// byLoginEmail keys on the email in a JSON body, so one account cannot be attacked from many
// addresses. The body is restored for the handler.
func byLoginEmail(r *http.Request) (string, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var payload struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(body, &payload)
	return strings.ToLower(strings.TrimSpace(payload.Email)), nil
}

// byUser keys on the authenticated user, falling back to the client address.
func byUser(r *http.Request) (string, error) {
	if claims, err := middleware.GetUserClaims(r.Context()); err == nil {
		return claims.UserID.String(), nil
	}
	return byIP(r)
}

// byBearer keys on a hash of the bearer credential (e.g. a service token) without keeping it.
func byBearer(r *http.Request) (string, error) {
	sum := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	return hex.EncodeToString(sum[:8]), nil
}
