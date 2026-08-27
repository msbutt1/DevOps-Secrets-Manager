package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type createdToken struct {
	ID     string `json:"id"`
	Token  string `json:"token"`
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
}

func TestServiceTokenLifecycle(t *testing.T) {
	f := newFixture(t) // payments-api/production with DATABASE_URL
	f.createSecret(t, f.envID, "API_KEY", "sk_live_123")
	staging := f.createEnv(t, f.vaultID, "staging")
	f.createSecret(t, staging, "STAGING_ONLY", "not for production tokens")
	dev := f.member(t, "Dev Developer", "developer", f.vaultID, "developer")

	f.api.MustDo(http.StatusForbidden, "POST", "/envs/"+f.envID+"/tokens", dev.Token, map[string]any{"name": "ci"})
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token, map[string]any{"name": ""})
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token, map[string]any{"name": "x", "expires_in_days": 999})

	var created createdToken
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token,
		map[string]any{"name": "github-actions", "expires_in_days": 30}).Decode(t, &created)
	if !strings.HasPrefix(created.Token, "dsm_st_") || !strings.HasPrefix(created.Token, created.Prefix) {
		t.Fatalf("unexpected token: %+v", created)
	}

	// Listing never shows the value
	list := f.api.MustDo(http.StatusOK, "GET", "/envs/"+f.envID+"/tokens", f.owner.Token, nil)
	if strings.Contains(string(list.Body), created.Token) {
		t.Fatal("token value leaked in list response")
	}
	var stored int
	if err := f.api.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM service_tokens WHERE token_hash = $1 OR token_prefix = $1`, created.Token).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("plaintext token stored in the database")
	}

	// The token reads its own environment and nothing else
	resp := f.api.MustDo(http.StatusOK, "GET", "/token/secrets", created.Token, nil)
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("want Cache-Control no-store, got %q", resp.Header.Get("Cache-Control"))
	}
	var body struct {
		VaultName       string `json:"vault_name"`
		EnvironmentName string `json:"environment_name"`
		Secrets         []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"secrets"`
	}
	resp.Decode(t, &body)
	got := map[string]string{}
	for _, s := range body.Secrets {
		got[s.Key] = s.Value
	}
	if body.VaultName != "payments-api" || body.EnvironmentName != "production" || len(got) != 2 || got["API_KEY"] != "sk_live_123" {
		t.Fatalf("unexpected secrets: %+v", body)
	}

	// A service token is not a user session
	f.api.MustDo(http.StatusUnauthorized, "GET", "/vaults", created.Token, nil)
	f.api.MustDo(http.StatusUnauthorized, "GET", "/token/secrets", f.owner.Token, nil)
	f.api.MustDo(http.StatusUnauthorized, "GET", "/token/secrets", "dsm_st_notarealtoken", nil)

	// The read is audited as the token
	var page auditPage
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=env.exported", f.owner.Token, nil).Decode(t, &page)
	if page.Total != 1 || page.Data[0].UserEmail != "token:github-actions" || page.Data[0].EnvironmentName == nil || *page.Data[0].EnvironmentName != "production" {
		t.Fatalf("export not audited as the token: %+v", page)
	}

	// Revocation: developers cannot, strangers get 404, owners can
	stranger := f.api.CreateUser("Stranger")
	f.api.MustDo(http.StatusForbidden, "DELETE", "/tokens/"+created.ID, dev.Token, nil)
	f.api.MustDo(http.StatusNotFound, "DELETE", "/tokens/"+created.ID, stranger.Token, nil)
	f.api.MustDo(http.StatusNoContent, "DELETE", "/tokens/"+created.ID, f.owner.Token, nil)
	f.api.MustDo(http.StatusUnauthorized, "GET", "/token/secrets", created.Token, nil)
	f.api.MustDo(http.StatusNotFound, "DELETE", "/tokens/"+created.ID, f.owner.Token, nil)

	// Expired tokens stop working
	var expiring createdToken
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token, map[string]any{"name": "short"}).Decode(t, &expiring)
	if _, err := f.api.Pool.Exec(context.Background(), `UPDATE service_tokens SET expires_at = now() - interval '1 second' WHERE id = $1`, expiring.ID); err != nil {
		t.Fatal(err)
	}
	f.api.MustDo(http.StatusUnauthorized, "GET", "/token/secrets", expiring.Token, nil)
}
