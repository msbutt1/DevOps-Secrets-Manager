package http_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestPasswordReset(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("Forgetful Fran")
	verificationToken := api.Email.WaitForToken(t, user.Email)
	var session tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": apitest.TestPassword}).Decode(t, &session)

	// Same answer for known and unknown addresses; no email for unknown ones.
	known := api.MustDo(http.StatusAccepted, "POST", "/auth/forgot-password", "", map[string]string{"email": user.Email})
	unknown := api.MustDo(http.StatusAccepted, "POST", "/auth/forgot-password", "", map[string]string{"email": "ghost@example.test"})
	if string(known.Body) != string(unknown.Body) {
		t.Fatalf("responses differ: %s vs %s", known.Body, unknown.Body)
	}
	api.MustDo(http.StatusBadRequest, "POST", "/auth/forgot-password", "", map[string]string{"email": "nope"})
	first := waitForNewToken(t, api, user.Email, verificationToken)

	// A second request replaces the first link.
	api.MustDo(http.StatusAccepted, "POST", "/auth/forgot-password", "", map[string]string{"email": user.Email})
	second := waitForNewToken(t, api, user.Email, first)
	api.MustDo(http.StatusBadRequest, "POST", "/auth/reset-password", "", map[string]string{"token": first, "new_password": "Replacement-Copper-Kettle-5"})

	// A weak password is rejected without using up the token.
	api.MustDo(http.StatusBadRequest, "POST", "/auth/reset-password", "", map[string]string{"token": second, "new_password": "password1234"})

	// Lock the account, then reset: the lock is cleared, sessions end, the old password stops working.
	for i := 0; i < 5; i++ {
		api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": "Wrong-Guess-Number-1"})
	}
	api.MustDo(http.StatusTooManyRequests, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": apitest.TestPassword})

	newPassword := "Replacement-Copper-Kettle-5"
	api.MustDo(http.StatusOK, "POST", "/auth/reset-password", "", map[string]string{"token": second, "new_password": newPassword})
	api.MustDo(http.StatusBadRequest, "POST", "/auth/reset-password", "", map[string]string{"token": second, "new_password": "Another-Copper-Kettle-6"})

	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": session.RefreshToken})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": apitest.TestPassword})
	token := api.Login(user.Email, newPassword)

	var page struct {
		Data []struct {
			TargetName string `json:"target_name"`
		} `json:"data"`
	}
	api.MustDo(http.StatusOK, "GET", "/audit?action=user.password_reset", token, nil).Decode(t, &page)
	if len(page.Data) != 1 || page.Data[0].TargetName != user.Email {
		t.Fatalf("password reset not audited: %+v", page.Data)
	}

	// Expired links do not work.
	api.MustDo(http.StatusAccepted, "POST", "/auth/forgot-password", "", map[string]string{"email": user.Email})
	expired := waitForNewToken(t, api, user.Email, second)
	if _, err := api.Pool.Exec(context.Background(), `UPDATE password_reset_tokens SET expires_at = $1 WHERE used_at IS NULL`, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	api.MustDo(http.StatusBadRequest, "POST", "/auth/reset-password", "", map[string]string{"token": expired, "new_password": "Expired-Copper-Kettle-7"})
	if got := api.Email.Count("ghost@example.test"); got != 0 {
		t.Fatalf("%d emails sent to an unknown address", got)
	}
}

func TestPasswordResetVerifiesEmail(t *testing.T) {
	api := apitest.New(t)
	address := "never.verified@example.test"
	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{"email": address, "password": apitest.TestPassword, "name": "Never Verified"})
	verification := api.Email.WaitForToken(t, address)
	api.MustDo(http.StatusAccepted, "POST", "/auth/forgot-password", "", map[string]string{"email": address})
	reset := waitForNewToken(t, api, address, verification)
	api.MustDo(http.StatusOK, "POST", "/auth/reset-password", "", map[string]string{"token": reset, "new_password": "Proven-Mailbox-Owner-8"})
	api.Login(address, "Proven-Mailbox-Owner-8")
}
