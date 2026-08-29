package http

import (
	"net/http"
	"time"
)

// RefreshCookie describes the HttpOnly cookie that holds the web app's refresh token, so
// the token is never readable by page scripts.
type RefreshCookie struct {
	// Secure marks the cookie Secure and uses the __Host- prefix; off only in development over plain HTTP.
	Secure bool
	// TTL is the cookie lifetime and should match the refresh token lifetime.
	TTL time.Duration
}

// name returns the cookie name. The __Host- prefix makes browsers require Secure,
// Path=/ and no Domain, so a sibling subdomain cannot set or overwrite it.
func (c RefreshCookie) name() string {
	if c.Secure {
		return "__Host-dsm_refresh"
	}
	return "dsm_refresh"
}

func (c RefreshCookie) set(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(c.TTL.Seconds()),
		HttpOnly: true,
		Secure:   c.Secure,
		// Strict keeps the browser from sending it on cross-site requests (CSRF)
		SameSite: http.SameSiteStrictMode,
	})
}

func (c RefreshCookie) clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// read returns the refresh token from the cookie, if present.
func (c RefreshCookie) read(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(c.name())
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}
