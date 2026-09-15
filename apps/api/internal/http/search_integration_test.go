package http_test

import (
	"net/http"
	"testing"
)

type searchResult struct {
	KeyName         string `json:"key_name"`
	VaultName       string `json:"vault_name"`
	EnvironmentName string `json:"environment_name"`
}

func TestSearchSecrets(t *testing.T) {
	f := newFixture(t) // payments-api / production / DATABASE_URL
	staging := f.createEnv(t, f.vaultID, "staging")
	f.createSecret(t, staging, "DATABASE_PASSWORD", "x")
	other := f.createVault(t, "web-frontend")
	otherEnv := f.createEnv(t, other, "production")
	f.createSecret(t, otherEnv, "ANALYTICS_KEY", "x")

	search := func(token, query string) []searchResult {
		t.Helper()
		var results []searchResult
		f.api.MustDo(http.StatusOK, "GET", "/search?q="+query, token, nil).Decode(t, &results)
		return results
	}

	// Matches across vaults and environments, case-insensitively.
	results := search(f.owner.Token, "database")
	if len(results) != 2 || results[0].KeyName != "DATABASE_PASSWORD" || results[1].KeyName != "DATABASE_URL" {
		t.Fatalf("search across vaults: %+v", results)
	}
	if results[0].VaultName != "payments-api" || results[0].EnvironmentName != "staging" {
		t.Fatalf("result location: %+v", results[0])
	}
	if got := search(f.owner.Token, "KEY"); len(got) != 1 || got[0].VaultName != "web-frontend" {
		t.Fatalf("search in another vault: %+v", got)
	}
	if got := search(f.owner.Token, "nothing-matches"); len(got) != 0 {
		t.Fatalf("expected no results, got %+v", got)
	}

	// Results are limited to vaults the caller can see.
	viewer := f.member(t, "Vera Viewer", "viewer", f.vaultID, "viewer")
	if got := search(viewer.Token, "e"); len(got) != 2 {
		t.Fatalf("viewer should only see the vault they are a member of: %+v", got)
	}
	stranger := f.api.CreateUser("Stan Stranger")
	if got := search(stranger.Token, "database"); len(got) != 0 {
		t.Fatalf("stranger sees secrets: %+v", got)
	}

	// Deleted secrets and environments drop out.
	doomed := f.createSecret(t, f.envID, "DATABASE_TEMP", "x")
	f.api.MustDo(http.StatusNoContent, "DELETE", "/secrets/"+doomed, f.owner.Token, nil)
	if got := search(f.owner.Token, "database"); len(got) != 2 {
		t.Fatalf("deleted secret still listed: %+v", got)
	}
	f.api.MustDo(http.StatusNoContent, "DELETE", "/envs/"+staging, f.owner.Token, nil)
	if got := search(f.owner.Token, "database"); len(got) != 1 || got[0].KeyName != "DATABASE_URL" {
		t.Fatalf("secrets of a deleted environment still listed: %+v", got)
	}

	f.api.MustDo(http.StatusBadRequest, "GET", "/search?q=", f.owner.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "GET", "/search", f.owner.Token, nil)
	f.api.MustDo(http.StatusUnauthorized, "GET", "/search?q=database", "", nil)
}
