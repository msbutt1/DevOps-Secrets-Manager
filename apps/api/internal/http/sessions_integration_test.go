package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

type sessionDTO struct {
	ID        string  `json:"id"`
	IPAddress *string `json:"ip_address"`
	UserAgent *string `json:"user_agent"`
	Current   bool    `json:"current"`
}

func TestSessions(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("Sam Sessions")

	loginAs := func(userAgent string) tokenPair {
		req := map[string]string{"email": user.Email, "password": apitest.TestPassword}
		httpReq := jsonRequest(t, api, "POST", "/auth/login", req)
		httpReq.Header.Set("User-Agent", userAgent)
		resp := api.Send(httpReq)
		if resp.Status != http.StatusOK {
			t.Fatalf("login: %d %s", resp.Status, resp.Body)
		}
		var tokens tokenPair
		resp.Decode(t, &tokens)
		return tokens
	}
	listSessions := func(token string) []sessionDTO {
		var sessions []sessionDTO
		api.MustDo(http.StatusOK, "GET", "/auth/sessions", token, nil).Decode(t, &sessions)
		return sessions
	}

	laptop := loginAs("Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0")
	phone := loginAs("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Safari/604.1")

	sessions := listSessions(laptop.AccessToken)
	if len(sessions) != 3 { // CreateUser's login, laptop and phone
		t.Fatalf("want 3 sessions, got %+v", sessions)
	}
	var current, phoneSession *sessionDTO
	for i := range sessions {
		s := &sessions[i]
		if s.Current {
			current = s
		}
		if s.UserAgent != nil && *s.UserAgent == "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Safari/604.1" {
			phoneSession = s
		}
	}
	if current == nil || current.UserAgent == nil || *current.UserAgent != "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0" || current.IPAddress == nil {
		t.Fatalf("current session not identified: %+v", sessions)
	}
	if phoneSession == nil || phoneSession.Current {
		t.Fatalf("phone session missing: %+v", sessions)
	}

	// Refreshing keeps the session (same ID) rather than creating a new one.
	var phoneRefreshed tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": phone.RefreshToken}).Decode(t, &phoneRefreshed)
	if got := listSessions(phoneRefreshed.AccessToken); len(got) != 3 {
		t.Fatalf("refresh changed the session count: %+v", got)
	}

	// Another user cannot see or revoke these sessions.
	other := api.CreateUser("Other Person")
	if got := listSessions(other.Token); len(got) != 1 {
		t.Fatalf("other user sees %d sessions", len(got))
	}
	api.MustDo(http.StatusNotFound, "DELETE", "/auth/sessions/"+phoneSession.ID, other.Token, nil)
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": phoneRefreshed.RefreshToken}).Decode(t, &phoneRefreshed)

	// Sign out the phone from the laptop.
	api.MustDo(http.StatusNoContent, "DELETE", "/auth/sessions/"+phoneSession.ID, laptop.AccessToken, nil)
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": phoneRefreshed.RefreshToken})
	api.MustDo(http.StatusNotFound, "DELETE", "/auth/sessions/"+phoneSession.ID, laptop.AccessToken, nil)
	api.MustDo(http.StatusBadRequest, "DELETE", "/auth/sessions/not-a-uuid", laptop.AccessToken, nil)

	// Sign out everywhere else: only the laptop remains.
	loginAs("curl/8.5.0")
	var revoked struct {
		SessionsRevoked int `json:"sessions_revoked"`
	}
	api.MustDo(http.StatusOK, "POST", "/auth/sessions/revoke-others", laptop.AccessToken, nil).Decode(t, &revoked)
	if revoked.SessionsRevoked != 2 { // CreateUser's and curl's
		t.Fatalf("want 2 sessions revoked, got %d", revoked.SessionsRevoked)
	}
	if got := listSessions(laptop.AccessToken); len(got) != 1 || !got[0].Current {
		t.Fatalf("after signing out others: %+v", got)
	}
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": laptop.RefreshToken})

	var page struct {
		Data []struct {
			Metadata map[string]any `json:"metadata"`
		} `json:"data"`
	}
	api.MustDo(http.StatusOK, "GET", "/audit?action=user.sessions_revoked", laptop.AccessToken, nil).Decode(t, &page)
	if len(page.Data) != 2 {
		t.Fatalf("want 2 sessions_revoked events, got %+v", page.Data)
	}
}

// jsonRequest builds a request with a JSON body for tests that need to set extra headers.
func jsonRequest(t *testing.T, api *apitest.Server, method, path string, body any) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, api.URL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}
