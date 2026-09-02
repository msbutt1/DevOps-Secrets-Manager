package http_test

import (
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestChangePasswordSignsOutOtherSessions(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("Pat Keeper") // one session from CreateUser

	login := func(password string) tokenPair {
		var tokens tokenPair
		api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": password}).Decode(t, &tokens)
		return tokens
	}
	laptop, phone, tablet := login(apitest.TestPassword), login(apitest.TestPassword), login(apitest.TestPassword)

	// The phone rotates its refresh token once, so the session is a family of tokens.
	var phoneRefreshed tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": phone.RefreshToken}).Decode(t, &phoneRefreshed)

	newPassword := "Harbor-New-Lantern-Garden-9"
	var resp struct {
		Message string `json:"message"`
	}
	api.MustDo(http.StatusOK, "POST", "/auth/change-password", laptop.AccessToken, map[string]string{
		"current_password": apitest.TestPassword, "new_password": newPassword,
	}).Decode(t, &resp)

	// The laptop keeps its session; every other session's refresh token stops working.
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": laptop.RefreshToken})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": phoneRefreshed.RefreshToken})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": tablet.RefreshToken})

	// The old password no longer works, the new one does.
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": user.Email, "password": apitest.TestPassword})
	login(newPassword)

	// Audited with the number of sessions ended: CreateUser's, the phone's and the tablet's.
	var page struct {
		Data []struct {
			Metadata map[string]any `json:"metadata"`
		} `json:"data"`
	}
	api.MustDo(http.StatusOK, "GET", "/audit?action=user.password_changed", laptop.AccessToken, nil).Decode(t, &page)
	if len(page.Data) != 1 || page.Data[0].Metadata["sessions_revoked"] != float64(3) {
		t.Fatalf("password change not audited as expected: %+v", page.Data)
	}

	// A wrong current password changes nothing and signs nobody out.
	fresh := login(newPassword)
	api.MustDo(http.StatusBadRequest, "POST", "/auth/change-password", laptop.AccessToken, map[string]string{
		"current_password": "Not-The-Password-1", "new_password": "Another-Strong-Phrase-4",
	})
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": fresh.RefreshToken})
}
