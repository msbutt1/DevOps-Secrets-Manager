package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

const refreshCookieName = "__Host-dsm_refresh"

// refreshCookie returns the refresh cookie set by the response, failing if there is none.
func refreshCookie(t *testing.T, resp apitest.Response) *http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: resp.Header}).Cookies() {
		if c.Name == refreshCookieName {
			return c
		}
	}
	t.Fatalf("no %s cookie in response: %v", refreshCookieName, resp.Header.Values("Set-Cookie"))
	return nil
}

func TestRefreshTokenCookie(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("Cookie User")

	login := api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]any{
		"email": user.Email, "password": apitest.TestPassword, "use_cookie": true,
	})
	var tokens tokenPair
	login.Decode(t, &tokens)
	if tokens.AccessToken == "" {
		t.Fatal("login did not return an access token")
	}
	if tokens.RefreshToken != "" || strings.Contains(string(login.Body), "refresh_token") {
		t.Fatalf("refresh token must not be in the body in cookie mode: %s", login.Body)
	}

	first := refreshCookie(t, login)
	if !first.HttpOnly || !first.Secure || first.SameSite != http.SameSiteStrictMode || first.Path != "/" || first.MaxAge <= 0 {
		t.Fatalf("cookie flags: %+v", first)
	}

	// Refreshing with only the cookie rotates it and keeps the token out of the body.
	refreshed := api.DoWithCookies("POST", "/auth/refresh", "", map[string]any{}, first)
	if refreshed.Status != http.StatusOK {
		t.Fatalf("cookie refresh: %d %s", refreshed.Status, refreshed.Body)
	}
	if strings.Contains(string(refreshed.Body), "refresh_token") {
		t.Fatalf("refresh token leaked into the body: %s", refreshed.Body)
	}
	var again tokenPair
	refreshed.Decode(t, &again)
	api.MustDo(http.StatusOK, "GET", "/auth/me", again.AccessToken, nil)
	second := refreshCookie(t, refreshed)
	if second.Value == first.Value {
		t.Fatal("refresh cookie was not rotated")
	}

	// The old cookie is revoked. The failure does not touch cookies, so it cannot clobber
	// a newer cookie that another tab received.
	stale := api.DoWithCookies("POST", "/auth/refresh", "", map[string]any{}, first)
	if stale.Status != http.StatusUnauthorized {
		t.Fatalf("stale cookie refresh: want 401, got %d", stale.Status)
	}
	if cookies := stale.Header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("failed refresh changed cookies: %v", cookies)
	}

	// No cookie and no body token is rejected.
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]any{})

	// A cookie with another name (e.g. set by a sibling site without the __Host- prefix) is ignored.
	if resp := api.DoWithCookies("POST", "/auth/refresh", "", map[string]any{}, &http.Cookie{Name: "dsm_refresh", Value: second.Value}); resp.Status != http.StatusUnauthorized {
		t.Fatalf("unprefixed cookie accepted: %d", resp.Status)
	}

	// Logout with the cookie revokes the token and clears the cookie.
	out := api.DoWithCookies("POST", "/auth/logout", "", map[string]any{}, second)
	if out.Status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", out.Status, out.Body)
	}
	if c := refreshCookie(t, out); c.MaxAge >= 0 {
		t.Fatalf("logout did not clear the cookie: %+v", c)
	}
	if resp := api.DoWithCookies("POST", "/auth/refresh", "", map[string]any{}, second); resp.Status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: want 401, got %d", resp.Status)
	}
}

func TestBodyRefreshTokensStillWork(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("CLI User")

	// Clients that do not ask for a cookie (the CLI) get the refresh token in the body and no cookie.
	login := api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]any{"email": user.Email, "password": apitest.TestPassword})
	if cookies := login.Header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("unexpected cookies for body login: %v", cookies)
	}
	var tokens tokenPair
	login.Decode(t, &tokens)
	if tokens.RefreshToken == "" {
		t.Fatal("body login did not return a refresh token")
	}
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]any{"refresh_token": tokens.RefreshToken})
}
