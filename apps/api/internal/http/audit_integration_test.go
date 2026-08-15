package http_test

import (
	"fmt"
	"net/http"
	"testing"
)

type auditEvent struct {
	Action          string  `json:"action"`
	UserEmail       string  `json:"user_email"`
	VaultName       *string `json:"vault_name"`
	EnvironmentName *string `json:"environment_name"`
	TargetName      *string `json:"target_name"`
	IPAddress       *string `json:"ip_address"`
	UserAgent       *string `json:"user_agent"`
}

type auditPage struct {
	Data    []auditEvent `json:"data"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	Limit   int          `json:"limit"`
	HasMore bool         `json:"has_more"`
}

func getAudit(t *testing.T, f *fixture, token, query string) auditPage {
	t.Helper()
	var page auditPage
	f.api.MustDo(http.StatusOK, "GET", "/audit"+query, token, nil).Decode(t, &page)
	return page
}

func TestAuditLogIsPaginatedWithNames(t *testing.T) {
	f := newFixture(t)
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)

	page := getAudit(t, f, f.owner.Token, "")
	if page.Page != 1 || page.Limit != 50 || page.HasMore {
		t.Fatalf("unexpected pagination: %+v", page)
	}
	if page.Total != len(page.Data) || page.Total == 0 {
		t.Fatalf("total %d does not match %d events", page.Total, len(page.Data))
	}

	newest := page.Data[0]
	if newest.UserEmail != f.owner.Email {
		t.Errorf("user email: got %q", newest.UserEmail)
	}
	if newest.VaultName == nil || *newest.VaultName != "payments-api" {
		t.Errorf("vault name: got %v", newest.VaultName)
	}
	if newest.EnvironmentName == nil || *newest.EnvironmentName != "production" {
		t.Errorf("environment name: got %v", newest.EnvironmentName)
	}
	if newest.TargetName == nil || *newest.TargetName != "DATABASE_URL" {
		t.Errorf("target name: got %v", newest.TargetName)
	}
	if newest.IPAddress == nil || *newest.IPAddress != "127.0.0.1" {
		t.Errorf("ip address: got %v", newest.IPAddress)
	}
	if newest.UserAgent == nil || *newest.UserAgent == "" {
		t.Errorf("user agent missing")
	}

	for i := 0; i < 3; i++ {
		f.createSecret(t, f.envID, fmt.Sprintf("PAGED_%d", i), "v")
	}
	first := getAudit(t, f, f.owner.Token, "?page=1&limit=2")
	second := getAudit(t, f, f.owner.Token, "?page=2&limit=2")
	if len(first.Data) != 2 || !first.HasMore || first.Total != page.Total+3 {
		t.Fatalf("unexpected first page: %+v", first)
	}
	if len(second.Data) != 2 || second.Data[0] == first.Data[0] {
		t.Fatalf("unexpected second page: %+v", second)
	}

	f.api.MustDo(http.StatusBadRequest, "GET", "/audit?page=0", f.owner.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "GET", "/audit?vaultId=nope", f.owner.Token, nil)
}

func TestAuditLogVisibility(t *testing.T) {
	f := newFixture(t)
	viewer := f.member(t, "Vera Viewer", "developer", f.vaultID, "viewer")
	vaultAdmin := f.member(t, "Van Admin", "developer", f.vaultID, "admin")
	stranger := f.api.CreateUser("Stan Stranger")

	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)
	f.createSecret(t, f.envID, "VIEWER_CANNOT_SEE", "v")

	ownerTotal := getAudit(t, f, f.owner.Token, "").Total
	if got := getAudit(t, f, vaultAdmin.Token, "").Total; got != ownerTotal {
		t.Errorf("vault admin should see all %d vault events, saw %d", ownerTotal, got)
	}
	if got := getAudit(t, f, viewer.Token, "").Total; got != 0 {
		t.Errorf("vault viewer should see only their own events (none), saw %d", got)
	}
	if got := getAudit(t, f, stranger.Token, "?vaultId="+f.vaultID).Total; got != 0 {
		t.Errorf("stranger should see nothing, saw %d", got)
	}
}
