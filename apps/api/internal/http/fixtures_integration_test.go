package http_test

import (
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

// fixture is an owner's vault with one environment and one secret.
type fixture struct {
	api      *apitest.Server
	owner    apitest.User
	vaultID  string
	envID    string
	secretID string
}

type idResponse struct {
	ID string `json:"id"`
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	api := apitest.New(t)
	f := &fixture{api: api, owner: api.CreateUser("Olive Owner")}
	f.vaultID = f.createVault(t, "payments-api")
	f.envID = f.createEnv(t, f.vaultID, "production")
	f.secretID = f.createSecret(t, f.envID, "DATABASE_URL", "postgres://fixture")
	return f
}

func (f *fixture) createVault(t *testing.T, name string) string {
	t.Helper()
	var v idResponse
	f.api.MustDo(http.StatusCreated, http.MethodPost, "/vaults", f.owner.Token, map[string]any{"name": name}).Decode(t, &v)
	return v.ID
}

func (f *fixture) createEnv(t *testing.T, vaultID, name string) string {
	t.Helper()
	var e idResponse
	f.api.MustDo(http.StatusCreated, http.MethodPost, "/vaults/"+vaultID+"/envs", f.owner.Token, map[string]any{"name": name}).Decode(t, &e)
	return e.ID
}

func (f *fixture) createSecret(t *testing.T, envID, key, value string) string {
	t.Helper()
	var s idResponse
	f.api.MustDo(http.StatusCreated, http.MethodPost, "/envs/"+envID+"/secrets", f.owner.Token, map[string]any{"key_name": key, "value": value}).Decode(t, &s)
	return s.ID
}

// member creates a user in the owner's organization with the given org role and, when
// vaultRole is not empty, adds them to the vault through the API.
func (f *fixture) member(t *testing.T, name, orgRole, vaultID, vaultRole string) apitest.User {
	t.Helper()
	u := f.api.CreateUser(name)
	f.api.AddToOrg(u, f.owner.OrgID, orgRole)
	if vaultRole != "" {
		f.api.MustDo(http.StatusCreated, http.MethodPost, "/vaults/"+vaultID+"/members", f.owner.Token,
			map[string]any{"email": u.Email, "role": vaultRole})
	}
	return u
}
