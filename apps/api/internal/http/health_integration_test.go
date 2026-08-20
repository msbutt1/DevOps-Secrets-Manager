package http_test

import (
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestRegisterVerifyLoginAndMe(t *testing.T) {
	api := apitest.New(t)

	api.MustDo(http.StatusOK, http.MethodGet, "/health", "", nil)

	user := api.CreateUser("Ada Lovelace")

	var me struct {
		Email         string `json:"email"`
		CreatedAt     string `json:"created_at"`
		Organizations []struct {
			Role string `json:"role"`
		} `json:"organizations"`
	}
	api.MustDo(http.StatusOK, http.MethodGet, "/auth/me", user.Token, nil).Decode(t, &me)
	if me.Email != user.Email || me.CreatedAt == "" {
		t.Fatalf("expected %s, got %s", user.Email, me.Email)
	}
	if len(me.Organizations) != 1 || me.Organizations[0].Role != "owner" {
		t.Fatalf("expected one owned organization, got %+v", me.Organizations)
	}

	api.MustDo(http.StatusUnauthorized, http.MethodGet, "/auth/me", "", nil)
}
