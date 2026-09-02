package http_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestResendVerification(t *testing.T) {
	api := apitest.New(t)
	address := "late.verifier@example.test"
	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{"email": address, "password": apitest.TestPassword, "name": "Late Verifier"})
	first := api.Email.WaitForToken(t, address)

	// Unknown, differently capitalised and verified addresses all get the same answer.
	unknown := api.MustDo(http.StatusAccepted, "POST", "/auth/resend-verification", "", map[string]string{"email": "nobody@example.test"})
	resent := api.MustDo(http.StatusAccepted, "POST", "/auth/resend-verification", "", map[string]string{"email": "Late.Verifier@Example.test"})
	if string(unknown.Body) != string(resent.Body) {
		t.Fatalf("responses differ: %s vs %s", unknown.Body, resent.Body)
	}
	api.MustDo(http.StatusBadRequest, "POST", "/auth/resend-verification", "", map[string]string{"email": "not-an-email"})

	second := waitForNewToken(t, api, address, first)

	// Only the newest link works.
	api.MustDo(http.StatusBadRequest, "POST", "/auth/verify-email", "", map[string]string{"token": first})
	api.MustDo(http.StatusOK, "POST", "/auth/verify-email", "", map[string]string{"token": second})
	api.Login(address, apitest.TestPassword)

	// Once verified, nothing more is sent.
	api.MustDo(http.StatusAccepted, "POST", "/auth/resend-verification", "", map[string]string{"email": address})
	time.Sleep(100 * time.Millisecond)
	if got := api.Email.WaitForToken(t, address); got != second {
		t.Fatal("a verification email was sent to a verified account")
	}
	if got := api.Email.Count("nobody@example.test"); got != 0 {
		t.Fatalf("%d emails sent to an unknown address", got)
	}
}

func waitForNewToken(t *testing.T, api *apitest.Server, address, previous string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if token := api.Email.WaitForToken(t, address); token != previous {
			return token
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no new email sent to %s", address)
	return ""
}
