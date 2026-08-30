package http_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestPasswordPolicy(t *testing.T) {
	api := apitest.New(t)

	weak := map[string]string{
		"short":             "at least 12 characters",
		"Password2024!":     "too common",
		"Ada-Lovelace-99":   "name or email",
		"countess-of-code":  "name or email", // the email's local part
		"aaaaaaaaaaaaaaaaa": "5 different characters",
	}
	for password, want := range weak {
		resp := api.Do("POST", "/auth/register", "", map[string]string{
			"email": "countess-of-code@example.test", "password": password, "name": "Ada Lovelace",
		})
		if resp.Status != http.StatusBadRequest || !strings.Contains(string(resp.Body), "validation_failed") || !strings.Contains(string(resp.Body), want) {
			t.Errorf("register with %q: got %d %s, want 400 containing %q", password, resp.Status, resp.Body, want)
		}
	}
	// None of the rejected attempts created the account.
	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{
		"email": "countess-of-code@example.test", "password": "analytical-engine-notes", "name": "Ada Lovelace",
	})

	user := api.CreateUser("Charles Babbage")
	for _, password := range []string{"too-short", "qwerty123456", "Charles-Babbage-1"} {
		api.MustDo(http.StatusBadRequest, "POST", "/auth/change-password", user.Token, map[string]string{
			"current_password": apitest.TestPassword, "new_password": password,
		})
	}
	// The old password still works after the rejected changes.
	api.Login(user.Email, apitest.TestPassword)

	api.MustDo(http.StatusOK, "POST", "/auth/change-password", user.Token, map[string]string{
		"current_password": apitest.TestPassword, "new_password": "difference-engine-no-2",
	})
	api.Login(user.Email, "difference-engine-no-2")
}
