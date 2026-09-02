package http_test

import (
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestRateLimits(t *testing.T) {
	api := apitest.NewWithOptions(t, apitest.Options{RateLimits: true})
	other := api.CreateUser("Other Account")

	// Login attempts are limited per email (10 per 5 minutes). An unknown address is used so the
	// account lockout, which starts after five failures, does not answer first.
	target := "Probed.Address@example.test"
	wrong := map[string]string{"email": target, "password": "not-the-password"}
	for i := 0; i < 10; i++ {
		api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", wrong)
	}
	limited := api.MustDo(http.StatusTooManyRequests, "POST", "/auth/login", "", wrong)
	if limited.Header.Get("Retry-After") == "" {
		t.Error("429 responses must include Retry-After")
	}
	var body struct {
		Error string `json:"error"`
	}
	limited.Decode(t, &body)
	if body.Error != "rate_limited" {
		t.Errorf("unexpected error code %q", body.Error)
	}
	// ...even with a different capitalisation of the address
	api.MustDo(http.StatusTooManyRequests, "POST", "/auth/login", "", map[string]string{"email": "  probed.address@EXAMPLE.test", "password": "x"})

	// Other accounts are unaffected
	api.Login(other.Email, apitest.TestPassword)

	// Verification emails can be requested three times an hour per address.
	resend := map[string]string{"email": "flooded@example.test"}
	for i := 0; i < 3; i++ {
		api.MustDo(http.StatusAccepted, "POST", "/auth/resend-verification", "", resend)
	}
	api.MustDo(http.StatusTooManyRequests, "POST", "/auth/resend-verification", "", resend)
	api.MustDo(http.StatusAccepted, "POST", "/auth/resend-verification", "", map[string]string{"email": "someone.else@example.test"})

}
