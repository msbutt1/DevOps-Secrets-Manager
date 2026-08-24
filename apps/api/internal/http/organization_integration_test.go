package http_test

import (
	"net/http"
	"testing"
)

type orgResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	MemberCount int    `json:"member_count"`
	VaultCount  int    `json:"vault_count"`
}

type orgMember struct {
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	LastLoginAt *string `json:"last_login_at"`
}

func TestOrganizationEndpoints(t *testing.T) {
	f := newFixture(t) // owner's organization has one vault
	admin := f.member(t, "Ada Admin", "admin", "", "")
	dev := f.member(t, "Dev Developer", "developer", f.vaultID, "developer")
	stranger := f.api.CreateUser("Stranger")
	org := f.owner.OrgID.String()

	var orgs []orgResponse
	f.api.MustDo(http.StatusOK, "GET", "/orgs", dev.Token, nil).Decode(t, &orgs)
	if len(orgs) != 2 {
		t.Fatalf("developer should see their own and the owner's organization, got %+v", orgs)
	}
	var one orgResponse
	f.api.MustDo(http.StatusOK, "GET", "/orgs/"+org, dev.Token, nil).Decode(t, &one)
	if one.Role != "developer" || one.MemberCount != 3 || one.VaultCount != 1 {
		t.Fatalf("unexpected organization: %+v", one)
	}
	f.api.MustDo(http.StatusNotFound, "GET", "/orgs/"+org, stranger.Token, nil)
	f.api.MustDo(http.StatusNotFound, "GET", "/orgs/"+org+"/members", stranger.Token, nil)

	// Renaming needs owner or admin.
	f.api.MustDo(http.StatusForbidden, "PATCH", "/orgs/"+org, dev.Token, map[string]any{"name": "Nope"})
	f.api.MustDo(http.StatusBadRequest, "PATCH", "/orgs/"+org, admin.Token, map[string]any{"name": "  "})
	f.api.MustDo(http.StatusOK, "PATCH", "/orgs/"+org, admin.Token, map[string]any{"name": "Acme Payments"}).Decode(t, &one)
	if one.Name != "Acme Payments" {
		t.Fatalf("rename failed: %+v", one)
	}

	var members []orgMember
	f.api.MustDo(http.StatusOK, "GET", "/orgs/"+org+"/members", dev.Token, nil).Decode(t, &members)
	if len(members) != 3 || members[0].Email != f.owner.Email || members[0].LastLoginAt == nil {
		t.Fatalf("unexpected members: %+v", members)
	}

	// Role changes: admins manage non-owners; only owners touch the owner role.
	f.api.MustDo(http.StatusForbidden, "PUT", "/orgs/"+org+"/members/"+admin.ID.String(), dev.Token, map[string]any{"role": "viewer"})
	f.api.MustDo(http.StatusForbidden, "PUT", "/orgs/"+org+"/members/"+dev.ID.String(), admin.Token, map[string]any{"role": "owner"})
	f.api.MustDo(http.StatusForbidden, "PUT", "/orgs/"+org+"/members/"+f.owner.ID.String(), admin.Token, map[string]any{"role": "viewer"})
	f.api.MustDo(http.StatusBadRequest, "PUT", "/orgs/"+org+"/members/"+f.owner.ID.String(), f.owner.Token, map[string]any{"role": "admin"})
	f.api.MustDo(http.StatusBadRequest, "PUT", "/orgs/"+org+"/members/"+dev.ID.String(), admin.Token, map[string]any{"role": "superuser"})
	f.api.MustDo(http.StatusNotFound, "PUT", "/orgs/"+org+"/members/"+stranger.ID.String(), admin.Token, map[string]any{"role": "viewer"})
	var changed orgMember
	f.api.MustDo(http.StatusOK, "PUT", "/orgs/"+org+"/members/"+dev.ID.String(), admin.Token, map[string]any{"role": "viewer"}).Decode(t, &changed)
	if changed.Role != "viewer" {
		t.Fatalf("role not changed: %+v", changed)
	}

	// Removal also removes vault access.
	f.api.MustDo(http.StatusOK, "GET", "/vaults/"+f.vaultID, dev.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "DELETE", "/orgs/"+org+"/members/"+admin.ID.String(), admin.Token, nil)
	f.api.MustDo(http.StatusForbidden, "DELETE", "/orgs/"+org+"/members/"+f.owner.ID.String(), admin.Token, nil)
	f.api.MustDo(http.StatusNoContent, "DELETE", "/orgs/"+org+"/members/"+dev.ID.String(), admin.Token, nil)
	f.api.MustDo(http.StatusNotFound, "GET", "/vaults/"+f.vaultID, dev.Token, nil)
	var vaultMembers []struct {
		Email string `json:"email"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/vaults/"+f.vaultID+"/members", f.owner.Token, nil).Decode(t, &vaultMembers)
	for _, m := range vaultMembers {
		if m.Email == dev.Email {
			t.Fatal("removed member still listed on the vault")
		}
	}

	var page auditPage
	f.api.MustDo(http.StatusOK, "GET", "/audit?organizationId="+org+"&limit=200", f.owner.Token, nil).Decode(t, &page)
	seen := map[string]bool{}
	for _, e := range page.Data {
		seen[e.Action] = true
	}
	for _, action := range []string{"org.updated", "org.member_role_changed", "org.member_removed"} {
		if !seen[action] {
			t.Errorf("%s not audited", action)
		}
	}
}

func TestOrganizationFilterScopesVaultsAndDashboard(t *testing.T) {
	f := newFixture(t) // a vault with one secret in the owner's organization
	dev := f.member(t, "Dual Member", "developer", f.vaultID, "developer")
	var own idResponse
	f.api.MustDo(http.StatusCreated, "POST", "/vaults", dev.Token, map[string]any{"name": "personal", "organization_id": dev.OrgID.String()}).Decode(t, &own)

	count := func(query string) (vaults int, secrets int) {
		var list []idResponse
		f.api.MustDo(http.StatusOK, "GET", "/vaults"+query, dev.Token, nil).Decode(t, &list)
		var stats statsResponse
		f.api.MustDo(http.StatusOK, "GET", "/stats"+query, dev.Token, nil).Decode(t, &stats)
		if stats.Vaults != len(list) {
			t.Fatalf("%s: stats count %d differs from list %d", query, stats.Vaults, len(list))
		}
		return len(list), stats.Secrets
	}

	if v, _ := count(""); v != 2 {
		t.Fatalf("without a filter expected both vaults, got %d", v)
	}
	if v, s := count("?organizationId=" + f.owner.OrgID.String()); v != 1 || s != 1 {
		t.Fatalf("team organization: want 1 vault and 1 secret, got %d and %d", v, s)
	}
	if v, s := count("?organizationId=" + dev.OrgID.String()); v != 1 || s != 0 {
		t.Fatalf("personal organization: want 1 vault and no secrets, got %d and %d", v, s)
	}
	f.api.MustDo(http.StatusOK, "GET", "/alerts?organizationId="+dev.OrgID.String(), dev.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "GET", "/vaults?organizationId=nope", dev.Token, nil)
}
