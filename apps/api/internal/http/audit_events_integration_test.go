package http_test

import (
	"net/http"
	"sort"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
)

// TestEveryAuditActionIsRecorded performs each auditable operation once and checks that the
// audit log then contains every action the API and web app list.
func TestEveryAuditActionIsRecorded(t *testing.T) {
	f := newFixture(t) // vault.created, env.created, secret.created, login.success
	api := f.api

	api.MustDo(http.StatusOK, "PUT", "/vaults/"+f.vaultID, f.owner.Token, map[string]any{"name": "payments-core"})
	api.MustDo(http.StatusOK, "PUT", "/envs/"+f.envID, f.owner.Token, map[string]any{"name": "prod"})
	api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"value": "new"})
	api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)
	api.MustDo(http.StatusNoContent, "DELETE", "/secrets/"+f.createSecret(t, f.envID, "TEMP", "x"), f.owner.Token, nil)
	api.MustDo(http.StatusNoContent, "DELETE", "/envs/"+f.createEnv(t, f.vaultID, "scratch"), f.owner.Token, nil)

	teammate := f.member(t, "Tess Teammate", "developer", f.vaultID, "viewer") // member.added
	api.MustDo(http.StatusOK, "PUT", "/vaults/"+f.vaultID+"/members/"+teammate.ID.String(), f.owner.Token, map[string]any{"role": "oncall"})
	api.MustDo(http.StatusNoContent, "DELETE", "/vaults/"+f.vaultID+"/members/"+teammate.ID.String(), f.owner.Token, nil)

	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": f.owner.Email, "password": "wrong-password"})
	api.Login(f.owner.Email, apitest.TestPassword)

	api.MustDo(http.StatusNoContent, "DELETE", "/vaults/"+f.createVault(t, "short-lived"), f.owner.Token, nil)

	org := f.owner.OrgID.String()
	api.MustDo(http.StatusOK, "PATCH", "/orgs/"+org, f.owner.Token, map[string]any{"name": "Renamed Org"})
	colleague := f.member(t, "Cole Colleague", "developer", "", "")
	api.MustDo(http.StatusOK, "PUT", "/orgs/"+org+"/members/"+colleague.ID.String(), f.owner.Token, map[string]any{"role": "viewer"})
	api.MustDo(http.StatusNoContent, "DELETE", "/orgs/"+org+"/members/"+colleague.ID.String(), f.owner.Token, nil)
	var inv struct {
		ID string `json:"id"`
	}
	api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", f.owner.Token, map[string]any{"email": "revoked@example.test", "role": "viewer"}).Decode(t, &inv)
	api.MustDo(http.StatusNoContent, "DELETE", "/orgs/"+org+"/invites/"+inv.ID, f.owner.Token, nil)
	api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", f.owner.Token, map[string]any{"email": "joiner@example.test", "role": "viewer"})
	api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{
		"email": "joiner@example.test", "password": apitest.TestPassword, "name": "Joiner",
		"invite_token": api.Email.WaitForToken(t, "joiner@example.test"),
	})

	var page auditPage
	api.MustDo(http.StatusOK, "GET", "/audit?limit=200", f.owner.Token, nil).Decode(t, &page)

	seen := map[string]auditEvent{}
	for _, e := range page.Data {
		if _, ok := seen[e.Action]; !ok {
			seen[e.Action] = e
		}
	}
	var missing []string
	for _, action := range audit.Actions {
		if _, ok := seen[action]; !ok {
			missing = append(missing, action)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("actions never recorded: %v", missing)
	}

	checks := map[string]string{
		audit.ActionMemberRoleChanged: teammate.Email,
		audit.ActionVaultUpdated:      "payments-core",
		audit.ActionEnvDeleted:        "scratch",
		audit.ActionLoginFailure:      f.owner.Email,
	}
	for action, target := range checks {
		if got := seen[action].TargetName; got == nil || *got != target {
			t.Errorf("%s target name: want %q, got %v", action, target, got)
		}
	}
	if v := seen[audit.ActionEnvDeleted].VaultName; v == nil || *v != "payments-core" {
		t.Errorf("env.deleted vault name: got %v", v)
	}
}
