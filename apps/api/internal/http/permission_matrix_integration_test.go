package http_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
)

// TestRolePermissionMatrix exercises every vault-scoped endpoint as each of the five vault roles
// and checks the result against the permission matrix shared with the web app.
func TestRolePermissionMatrix(t *testing.T) {
	f := newFixture(t)

	type call struct {
		name   string
		needs  func(policy.Permissions) bool
		status int // success status
		run    func(t *testing.T, token string, vaultID string) int
	}
	read := func(p policy.Permissions) bool { return p.CanRead }
	write := func(p policy.Permissions) bool { return p.CanWrite }
	reveal := func(p policy.Permissions) bool { return p.CanReveal }
	manage := func(p policy.Permissions) bool { return p.CanManageMembers }
	del := func(p policy.Permissions) bool { return p.CanDelete }

	calls := []call{
		{"get vault", read, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("GET", "/vaults/"+v, tok, nil).Status
		}},
		{"list environments", read, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("GET", "/vaults/"+v+"/envs", tok, nil).Status
		}},
		{"get environment", read, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("GET", "/envs/"+f.envID, tok, nil).Status
		}},
		{"list secrets", read, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("GET", "/envs/"+f.envID+"/secrets", tok, nil).Status
		}},
		{"list members", read, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("GET", "/vaults/"+v+"/members", tok, nil).Status
		}},
		{"update vault", write, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("PUT", "/vaults/"+v, tok, map[string]any{"name": "payments-api"}).Status
		}},
		{"create environment", write, 201, func(t *testing.T, tok, v string) int {
			return f.api.Do("POST", "/vaults/"+v+"/envs", tok, map[string]any{"name": "env-" + suffix()}).Status
		}},
		{"update environment", write, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("PUT", "/envs/"+f.envID, tok, map[string]any{"name": "production"}).Status
		}},
		{"delete environment", write, 204, func(t *testing.T, tok, v string) int {
			return f.api.Do("DELETE", "/envs/"+f.createEnv(t, v, "doomed-"+suffix()), tok, nil).Status
		}},
		{"create secret", write, 201, func(t *testing.T, tok, v string) int {
			return f.api.Do("POST", "/envs/"+f.envID+"/secrets", tok, map[string]any{"key_name": "K_" + suffix(), "value": "v"}).Status
		}},
		{"update secret", write, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("PUT", "/secrets/"+f.secretID, tok, map[string]any{"value": "postgres://updated"}).Status
		}},
		{"delete secret", write, 204, func(t *testing.T, tok, v string) int {
			return f.api.Do("DELETE", "/secrets/"+f.createSecret(t, f.envID, "DOOMED_"+suffix(), "x"), tok, nil).Status
		}},
		{"reveal secret", reveal, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("POST", "/secrets/"+f.secretID+"/reveal", tok, nil).Status
		}},
		{"add, change and remove a member", manage, 204, func(t *testing.T, tok, v string) int {
			guest := f.member(t, "Guest "+suffix(), "viewer", "", "")
			if s := f.api.Do("POST", "/vaults/"+v+"/members", tok, map[string]any{"email": guest.Email, "role": "viewer"}).Status; s != 201 {
				return s
			}
			if s := f.api.Do("PUT", "/vaults/"+v+"/members/"+guest.ID.String(), tok, map[string]any{"role": "oncall"}).Status; s != 200 {
				return s
			}
			return f.api.Do("DELETE", "/vaults/"+v+"/members/"+guest.ID.String(), tok, nil).Status
		}},
		{"rotate vault key", manage, 200, func(t *testing.T, tok, v string) int {
			return f.api.Do("POST", "/vaults/"+v+"/rotate-key", tok, nil).Status
		}},
		{"delete vault", del, 204, func(t *testing.T, tok, v string) int {
			return f.api.Do("DELETE", "/vaults/"+v, tok, nil).Status
		}},
	}

	for _, role := range []string{"owner", "admin", "developer", "oncall", "viewer"} {
		perms := policy.PermissionsFor(role)
		// Members hold the lowest organization role so only the vault role grants anything.
		user := f.member(t, "Matrix "+role, "viewer", f.vaultID, role)

		for _, c := range calls {
			t.Run(fmt.Sprintf("%s/%s", role, c.name), func(t *testing.T) {
				vaultID := f.vaultID
				if c.name == "delete vault" {
					// Use a separate vault so the shared one survives.
					vaultID = f.createVault(t, "disposable-"+role)
					f.api.MustDo(http.StatusCreated, "POST", "/vaults/"+vaultID+"/members", f.owner.Token,
						map[string]any{"email": user.Email, "role": role})
				}

				want := http.StatusForbidden
				if c.needs(perms) {
					want = c.status
				}
				if got := c.run(t, user.Token, vaultID); got != want {
					t.Fatalf("%s as %s: want %d, got %d", c.name, role, want, got)
				}
			})
		}
	}
}

// suffix returns a short unique lowercase string for resource names.
func suffix() string {
	return strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
}
