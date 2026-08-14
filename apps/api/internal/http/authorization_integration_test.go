package http_test

import (
	"context"
	"net/http"
	"testing"
)

func TestVaultViewerCannotRevealEvenWithHigherOrgRole(t *testing.T) {
	f := newFixture(t)
	// An organization developer made a viewer on this vault must be limited to viewer rights.
	viewer := f.member(t, "Vic Viewer", "developer", f.vaultID, "viewer")

	f.api.MustDo(http.StatusOK, http.MethodGet, "/envs/"+f.envID+"/secrets", viewer.Token, nil)
	f.api.MustDo(http.StatusForbidden, http.MethodPost, "/secrets/"+f.secretID+"/reveal", viewer.Token, nil)
	f.api.MustDo(http.StatusForbidden, http.MethodPost, "/envs/"+f.envID+"/secrets", viewer.Token,
		map[string]any{"key_name": "NEW_KEY", "value": "x"})
}

func TestOrgOwnersAndAdminsInheritVaultAccess(t *testing.T) {
	f := newFixture(t)
	admin := f.member(t, "Ada Admin", "admin", "", "")

	var vault struct {
		UserRole string `json:"user_role"`
	}
	f.api.MustDo(http.StatusOK, http.MethodGet, "/vaults/"+f.vaultID, admin.Token, nil).Decode(t, &vault)
	if vault.UserRole != "admin" {
		t.Fatalf("expected inherited admin role, got %q", vault.UserRole)
	}
	f.api.MustDo(http.StatusOK, http.MethodPost, "/secrets/"+f.secretID+"/reveal", admin.Token, nil)
	f.api.MustDo(http.StatusForbidden, http.MethodDelete, "/vaults/"+f.vaultID, admin.Token, nil)

	var list []struct {
		ID string `json:"id"`
	}
	f.api.MustDo(http.StatusOK, http.MethodGet, "/vaults", admin.Token, nil).Decode(t, &list)
	if len(list) != 1 || list[0].ID != f.vaultID {
		t.Fatalf("expected the organization's vault in the admin's list, got %+v", list)
	}
}

func TestOrgMemberWithoutVaultMembershipCannotSeeVault(t *testing.T) {
	f := newFixture(t)
	dev := f.member(t, "Dev Outsider", "developer", "", "")

	f.api.MustDo(http.StatusNotFound, http.MethodGet, "/vaults/"+f.vaultID, dev.Token, nil)
	f.api.MustDo(http.StatusNotFound, http.MethodPost, "/secrets/"+f.secretID+"/reveal", dev.Token, nil)

	var list []any
	f.api.MustDo(http.StatusOK, http.MethodGet, "/vaults", dev.Token, nil).Decode(t, &list)
	if len(list) != 0 {
		t.Fatalf("expected no vaults, got %d", len(list))
	}
}

func TestVaultMembershipIsVoidAfterLeavingOrganization(t *testing.T) {
	f := newFixture(t)
	oncall := f.member(t, "Ollie Oncall", "developer", f.vaultID, "oncall")
	f.api.MustDo(http.StatusOK, http.MethodPost, "/secrets/"+f.secretID+"/reveal", oncall.Token, nil)

	if _, err := f.api.Pool.Exec(context.Background(),
		`DELETE FROM user_organizations WHERE user_id = $1 AND organization_id = $2`, oncall.ID, f.owner.OrgID); err != nil {
		t.Fatal(err)
	}
	f.api.MustDo(http.StatusNotFound, http.MethodPost, "/secrets/"+f.secretID+"/reveal", oncall.Token, nil)
}

func TestAdminsCannotGrantOrRemoveOwners(t *testing.T) {
	f := newFixture(t)
	admin := f.member(t, "Ada Admin", "developer", f.vaultID, "admin")
	dev := f.member(t, "Dee Developer", "developer", f.vaultID, "developer")

	f.api.MustDo(http.StatusForbidden, http.MethodPut, "/vaults/"+f.vaultID+"/members/"+dev.ID.String(), admin.Token,
		map[string]any{"role": "owner"})
	f.api.MustDo(http.StatusForbidden, http.MethodPut, "/vaults/"+f.vaultID+"/members/"+admin.ID.String(), admin.Token,
		map[string]any{"role": "owner"})
	f.api.MustDo(http.StatusForbidden, http.MethodDelete, "/vaults/"+f.vaultID+"/members/"+f.owner.ID.String(), admin.Token, nil)
	f.api.MustDo(http.StatusOK, http.MethodPut, "/vaults/"+f.vaultID+"/members/"+dev.ID.String(), admin.Token,
		map[string]any{"role": "oncall"})

	// The last owner cannot be demoted, and unknown members are 404.
	other := f.member(t, "Second Owner", "developer", "", "")
	f.api.MustDo(http.StatusNotFound, http.MethodPut, "/vaults/"+f.vaultID+"/members/"+other.ID.String(), f.owner.Token,
		map[string]any{"role": "viewer"})
}

func TestAddingUnknownEmailReturns404(t *testing.T) {
	f := newFixture(t)
	f.api.MustDo(http.StatusNotFound, http.MethodPost, "/vaults/"+f.vaultID+"/members", f.owner.Token,
		map[string]any{"email": "nobody@example.test", "role": "viewer"})
}
