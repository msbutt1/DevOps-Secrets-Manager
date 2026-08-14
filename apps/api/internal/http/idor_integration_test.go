package http_test

import (
	"net/http"
	"testing"
)

// TestCrossTenantIDsAreNotFound has user A use every ID belonging to user B's vault. Each
// request must answer 404 (not 403, which would confirm the ID exists) and change nothing.
func TestCrossTenantIDsAreNotFound(t *testing.T) {
	b := newFixture(t) // user B owns the vault, environment and secret
	a := b.api.CreateUser("Mallory Other")

	requests := []struct {
		method, path string
		body         any
	}{
		{"GET", "/vaults/" + b.vaultID, nil},
		{"PUT", "/vaults/" + b.vaultID, map[string]any{"name": "stolen"}},
		{"DELETE", "/vaults/" + b.vaultID, nil},
		{"GET", "/vaults/" + b.vaultID + "/envs", nil},
		{"POST", "/vaults/" + b.vaultID + "/envs", map[string]any{"name": "backdoor"}},
		{"GET", "/vaults/" + b.vaultID + "/members", nil},
		{"POST", "/vaults/" + b.vaultID + "/members", map[string]any{"email": a.Email, "role": "owner"}},
		{"PUT", "/vaults/" + b.vaultID + "/members/" + a.ID.String(), map[string]any{"role": "owner"}},
		{"DELETE", "/vaults/" + b.vaultID + "/members/" + b.owner.ID.String(), nil},
		{"GET", "/envs/" + b.envID, nil},
		{"PUT", "/envs/" + b.envID, map[string]any{"name": "renamed"}},
		{"DELETE", "/envs/" + b.envID, nil},
		{"GET", "/envs/" + b.envID + "/secrets", nil},
		{"POST", "/envs/" + b.envID + "/secrets", map[string]any{"key_name": "INJECTED", "value": "x"}},
		{"PUT", "/secrets/" + b.secretID, map[string]any{"value": "overwritten"}},
		{"POST", "/secrets/" + b.secretID + "/reveal", nil},
		{"DELETE", "/secrets/" + b.secretID, nil},
	}

	for _, req := range requests {
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			resp := b.api.Do(req.method, req.path, a.Token, req.body)
			if resp.Status != http.StatusNotFound {
				t.Fatalf("want 404, got %d: %s", resp.Status, resp.Body)
			}
		})
	}

	// Nothing B owns was changed.
	var reveal struct {
		Value string `json:"value"`
	}
	b.api.MustDo(http.StatusOK, "POST", "/secrets/"+b.secretID+"/reveal", b.owner.Token, nil).Decode(t, &reveal)
	if reveal.Value != "postgres://fixture" {
		t.Fatalf("secret value changed to %q", reveal.Value)
	}
	var env struct {
		Name string `json:"name"`
	}
	b.api.MustDo(http.StatusOK, "GET", "/envs/"+b.envID, b.owner.Token, nil).Decode(t, &env)
	if env.Name != "production" {
		t.Fatalf("environment renamed to %q", env.Name)
	}
	var members []struct {
		Email string `json:"email"`
	}
	b.api.MustDo(http.StatusOK, "GET", "/vaults/"+b.vaultID+"/members", b.owner.Token, nil).Decode(t, &members)
	if len(members) != 1 {
		t.Fatalf("expected only the owner as member, got %+v", members)
	}
}

// TestResourcesOfDeletedVaultAreNotFound checks that deleting a vault closes access to its
// environments and secrets by ID, even for its former owner.
func TestResourcesOfDeletedVaultAreNotFound(t *testing.T) {
	f := newFixture(t)
	f.api.MustDo(http.StatusNoContent, "DELETE", "/vaults/"+f.vaultID, f.owner.Token, nil)

	f.api.MustDo(http.StatusNotFound, "GET", "/envs/"+f.envID, f.owner.Token, nil)
	f.api.MustDo(http.StatusNotFound, "GET", "/envs/"+f.envID+"/secrets", f.owner.Token, nil)
	f.api.MustDo(http.StatusNotFound, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)
}
