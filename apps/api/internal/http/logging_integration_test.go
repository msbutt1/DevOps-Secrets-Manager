package http_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

// syncBuffer collects log output written concurrently by request goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestLogsNeverContainCredentials drives every credential-bearing flow and then checks that no
// password, token or secret value reached the logs.
func TestLogsNeverContainCredentials(t *testing.T) {
	logs := &syncBuffer{}
	api := apitest.NewWithOptions(t, apitest.Options{LogOutput: logs})
	var sensitive []string
	remember := func(values ...string) {
		for _, v := range values {
			if v == "" {
				t.Fatal("expected a credential value to check for")
			}
			sensitive = append(sensitive, v)
		}
	}

	// Registration, verification and login
	password := "Logging-Test-Horse-Staple-7"
	email := "log.checker@example.test"
	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{"email": email, "password": password, "name": "Log Checker"})
	verification := api.Email.WaitForToken(t, email)
	api.MustDo(http.StatusOK, "POST", "/auth/verify-email", "", map[string]string{"token": verification})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": email, "password": "Wrong-Password-Attempt-1"})
	var tokens tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": email, "password": password}).Decode(t, &tokens)
	remember(password, "Wrong-Password-Attempt-1", verification, tokens.AccessToken, tokens.RefreshToken)

	// Cookie login and refresh
	cookieLogin := api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]any{"email": email, "password": password, "use_cookie": true})
	cookie := refreshCookie(t, cookieLogin)
	remember(cookie.Value)
	refreshed := api.DoWithCookies("POST", "/auth/refresh", "", map[string]any{}, cookie)
	remember(refreshCookie(t, refreshed).Value)

	// Secrets: create, update, reveal
	token := tokens.AccessToken
	var vault, env, secret idResponse
	api.MustDo(http.StatusCreated, "POST", "/vaults", token, map[string]any{"name": "logging-vault"}).Decode(t, &vault)
	api.MustDo(http.StatusCreated, "POST", "/vaults/"+vault.ID+"/envs", token, map[string]any{"name": "production"}).Decode(t, &env)
	firstValue, secondValue := "sk_live_first_value_9f8e7d6c", "sk_live_second_value_1a2b3c4d"
	api.MustDo(http.StatusCreated, "POST", "/envs/"+env.ID+"/secrets", token, map[string]any{"key_name": "STRIPE_KEY", "value": firstValue}).Decode(t, &secret)
	api.MustDo(http.StatusOK, "PUT", "/secrets/"+secret.ID, token, map[string]any{"value": secondValue})
	api.MustDo(http.StatusOK, "POST", "/secrets/"+secret.ID+"/reveal", token, nil)
	remember(firstValue, secondValue)

	// Service token creation and use
	var created struct {
		Token string `json:"token"`
	}
	api.MustDo(http.StatusCreated, "POST", "/envs/"+env.ID+"/tokens", token, map[string]any{"name": "ci"}).Decode(t, &created)
	api.MustDo(http.StatusOK, "GET", "/token/secrets", created.Token, nil)
	remember(created.Token)

	// Invitation
	var user struct {
		Organizations []struct {
			ID string `json:"id"`
		} `json:"organizations"`
	}
	api.MustDo(http.StatusOK, "GET", "/auth/me", token, nil).Decode(t, &user)
	api.MustDo(http.StatusCreated, "POST", "/orgs/"+user.Organizations[0].ID+"/invites", token, map[string]any{"email": "invitee@example.test", "role": "viewer"})
	invite := api.Email.WaitForToken(t, "invitee@example.test")
	api.MustDo(http.StatusOK, "POST", "/invites/lookup", "", map[string]string{"token": invite})
	remember(invite)

	// Change password, and a credential in a query string
	newPassword := "Logging-Test-Lantern-Moss-8"
	api.MustDo(http.StatusOK, "POST", "/auth/change-password", token, map[string]string{"current_password": password, "new_password": newPassword})
	remember(newPassword)
	api.Do("GET", "/vaults?api_key=query-string-credential-5e6f", token, nil)
	remember("query-string-credential-5e6f")

	// Logout
	api.MustDo(http.StatusNoContent, "POST", "/auth/logout", "", map[string]string{"refresh_token": tokens.RefreshToken})

	output := logs.String()
	for _, value := range sensitive {
		if strings.Contains(output, value) {
			t.Errorf("logs contain a credential or secret value %q", value)
		}
	}
	for _, header := range []string{"Authorization", "Bearer", "Cookie", "dsm_refresh"} {
		if strings.Contains(output, header) {
			t.Errorf("logs mention %q", header)
		}
	}

	// Every line is JSON, and every request was logged with a request ID.
	requests := 0
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("log line is not JSON: %q", scanner.Text())
		}
		if line["msg"] == "request" {
			requests++
			if id, _ := line["request_id"].(string); id == "" {
				t.Errorf("request log without request_id: %v", line)
			}
		}
	}
	if requests < 19 {
		t.Fatalf("expected an access log line per request, got %d:\n%s", requests, output)
	}
}

func TestRequestIDs(t *testing.T) {
	logs := &syncBuffer{}
	api := apitest.NewWithOptions(t, apitest.Options{LogOutput: logs})
	user := api.CreateUser("Request Tracer")

	send := func(requestID string) apitest.Response {
		req, err := http.NewRequest("GET", api.URL+"/vaults", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+user.Token)
		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
		return api.Send(req)
	}

	findRequest := func(id string) map[string]any {
		t.Helper()
		scanner := bufio.NewScanner(strings.NewReader(logs.String()))
		for scanner.Scan() {
			var line map[string]any
			if json.Unmarshal(scanner.Bytes(), &line) == nil && line["msg"] == "request" && line["request_id"] == id {
				return line
			}
		}
		t.Fatalf("no request log with request_id %q", id)
		return nil
	}

	// A generated ID is returned to the client and matches the log line.
	generated := send("").Header.Get("X-Request-ID")
	if len(generated) < 16 {
		t.Fatalf("missing generated request ID: %q", generated)
	}
	line := findRequest(generated)
	if line["route"] != "/vaults/" && line["route"] != "/vaults" {
		t.Errorf("unexpected route: %v", line["route"])
	}
	if line["status"] != float64(200) || line["method"] != "GET" || line["user_id"] != user.ID.String() {
		t.Errorf("unexpected request log: %v", line)
	}

	// A well-formed incoming ID (e.g. from a load balancer) is kept.
	if got := send("lb-7f3a9c1e-5b2d").Header.Get("X-Request-ID"); got != "lb-7f3a9c1e-5b2d" {
		t.Errorf("incoming request ID not kept: %q", got)
	}
	findRequest("lb-7f3a9c1e-5b2d")

	// A malformed one cannot inject content into logs and is replaced.
	if got := send("bad id {\"level\":\"ERROR\"}").Header.Get("X-Request-ID"); got == "" || strings.ContainsAny(got, " \n{") {
		t.Errorf("malformed request ID was not replaced: %q", got)
	}
}
