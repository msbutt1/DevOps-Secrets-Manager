package http_test

import (
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func TestAuthLifecycle(t *testing.T) {
	api := apitest.New(t)
	email := "grace@example.test"

	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{"email": email, "password": apitest.TestPassword, "name": "Grace Hopper"})
	api.MustDo(http.StatusConflict, "POST", "/auth/register", "", map[string]string{"email": email, "password": apitest.TestPassword, "name": "Grace Again"})
	api.MustDo(http.StatusBadRequest, "POST", "/auth/register", "", map[string]string{"email": "", "password": "x", "name": "x"})

	// Unverified accounts cannot log in.
	api.MustDo(http.StatusForbidden, "POST", "/auth/login", "", map[string]string{"email": email, "password": apitest.TestPassword})

	token := api.Email.WaitForToken(t, email)
	api.MustDo(http.StatusBadRequest, "POST", "/auth/verify-email", "", map[string]string{"token": "not-a-token"})
	api.MustDo(http.StatusOK, "POST", "/auth/verify-email", "", map[string]string{"token": token})
	api.MustDo(http.StatusBadRequest, "POST", "/auth/verify-email", "", map[string]string{"token": token}) // single use

	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": email, "password": "wrong-password"})

	var first tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": email, "password": apitest.TestPassword}).Decode(t, &first)
	if first.AccessToken == "" || first.RefreshToken == "" || first.ExpiresIn != 900 {
		t.Fatalf("unexpected tokens: %+v", first)
	}
	api.MustDo(http.StatusOK, "GET", "/auth/me", first.AccessToken, nil)
	api.MustDo(http.StatusUnauthorized, "GET", "/auth/me", "garbage", nil)

	// Refresh rotates the refresh token; the old one stops working.
	var second tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/refresh", "", map[string]string{"refresh_token": first.RefreshToken}).Decode(t, &second)
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": first.RefreshToken})
	api.MustDo(http.StatusOK, "GET", "/auth/me", second.AccessToken, nil)

	// Logout revokes the refresh token.
	api.MustDo(http.StatusNoContent, "POST", "/auth/logout", "", map[string]string{"refresh_token": second.RefreshToken})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/refresh", "", map[string]string{"refresh_token": second.RefreshToken})

	// Change password: the old password stops working.
	var third tokenPair
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": email, "password": apitest.TestPassword}).Decode(t, &third)
	api.MustDo(http.StatusBadRequest, "POST", "/auth/change-password", third.AccessToken, map[string]string{"current_password": "wrong", "new_password": "Another-Passw0rd!"})
	api.MustDo(http.StatusOK, "POST", "/auth/change-password", third.AccessToken, map[string]string{"current_password": apitest.TestPassword, "new_password": "Another-Passw0rd!"})
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": email, "password": apitest.TestPassword})
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", map[string]string{"email": email, "password": "Another-Passw0rd!"})
}

// TestSecretLifecycleIsAudited walks a team through the main flow and checks that the audit log
// tells the same story, in order.
func TestSecretLifecycleIsAudited(t *testing.T) {
	api := apitest.New(t)
	owner := api.CreateUser("Olga Owner")
	oncall := api.CreateUser("Omar Oncall")
	api.AddToOrg(oncall, owner.OrgID, "developer")

	var vault idResponse
	api.MustDo(http.StatusCreated, "POST", "/vaults", owner.Token, map[string]any{"name": "checkout"}).Decode(t, &vault)
	var env idResponse
	api.MustDo(http.StatusCreated, "POST", "/vaults/"+vault.ID+"/envs", owner.Token, map[string]any{"name": "production"}).Decode(t, &env)
	var secret idResponse
	api.MustDo(http.StatusCreated, "POST", "/envs/"+env.ID+"/secrets", owner.Token, map[string]any{"key_name": "PAYMENT_KEY", "value": "v1"}).Decode(t, &secret)

	api.MustDo(http.StatusCreated, "POST", "/vaults/"+vault.ID+"/members", owner.Token, map[string]any{"email": oncall.Email, "role": "viewer"})
	api.MustDo(http.StatusForbidden, "POST", "/secrets/"+secret.ID+"/reveal", oncall.Token, nil)
	api.MustDo(http.StatusOK, "PUT", "/vaults/"+vault.ID+"/members/"+oncall.ID.String(), owner.Token, map[string]any{"role": "oncall"})

	var reveal struct {
		Value string `json:"value"`
	}
	api.MustDo(http.StatusOK, "POST", "/secrets/"+secret.ID+"/reveal", oncall.Token, nil).Decode(t, &reveal)
	if reveal.Value != "v1" {
		t.Fatalf("revealed %q", reveal.Value)
	}
	api.MustDo(http.StatusForbidden, "PUT", "/secrets/"+secret.ID, oncall.Token, map[string]any{"value": "v2"})
	api.MustDo(http.StatusOK, "PUT", "/secrets/"+secret.ID, owner.Token, map[string]any{"value": "v2"})
	api.MustDo(http.StatusNoContent, "DELETE", "/secrets/"+secret.ID, owner.Token, nil)
	api.MustDo(http.StatusNotFound, "POST", "/secrets/"+secret.ID+"/reveal", owner.Token, nil)

	var page auditPage
	api.MustDo(http.StatusOK, "GET", "/audit?vaultId="+vault.ID, owner.Token, nil).Decode(t, &page)
	var got []string
	for i := len(page.Data) - 1; i >= 0; i-- { // oldest first
		e := page.Data[i]
		got = append(got, e.Action+" by "+e.UserEmail)
	}
	want := []string{
		"vault.created by " + owner.Email,
		"env.created by " + owner.Email,
		"secret.created by " + owner.Email,
		"member.added by " + owner.Email,
		"member.role_changed by " + owner.Email,
		"secret.revealed by " + oncall.Email,
		"secret.updated by " + owner.Email,
		"secret.deleted by " + owner.Email,
	}
	if len(got) != len(want) {
		t.Fatalf("audit trail:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("audit trail step %d: got %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}
