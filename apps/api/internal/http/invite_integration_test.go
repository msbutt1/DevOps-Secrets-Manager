package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

type inviteResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	InvitedBy string `json:"invited_by"`
	EmailSent bool   `json:"email_sent"`
}

func TestInviteNewUserRegistersIntoOrganization(t *testing.T) {
	f := newFixture(t)
	org := f.owner.OrgID.String()

	var inv inviteResponse
	f.api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", f.owner.Token,
		map[string]any{"email": "  New.Hire@Example.test ", "role": "developer"}).Decode(t, &inv)
	if inv.Email != "new.hire@example.test" || inv.Role != "developer" || inv.InvitedBy != "Olive Owner" || !inv.EmailSent {
		t.Fatalf("unexpected invite: %+v", inv)
	}
	token := f.api.Email.WaitForToken(t, "new.hire@example.test")

	var lookup struct {
		OrganizationName string `json:"organization_name"`
		Email            string `json:"email"`
		AccountExists    bool   `json:"account_exists"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/invites/lookup", "", map[string]string{"token": token}).Decode(t, &lookup)
	if lookup.Email != "new.hire@example.test" || lookup.AccountExists || lookup.OrganizationName == "" {
		t.Fatalf("unexpected lookup: %+v", lookup)
	}
	f.api.MustDo(http.StatusNotFound, "POST", "/invites/lookup", "", map[string]string{"token": "wrong"})

	// The address must match the invitation.
	f.api.MustDo(http.StatusBadRequest, "POST", "/auth/register", "", map[string]string{
		"email": "someone.else@example.test", "password": apitest.TestPassword, "name": "Else", "invite_token": token,
	})

	var reg struct {
		VerificationRequired bool `json:"verification_required"`
	}
	f.api.MustDo(http.StatusCreated, "POST", "/auth/register", "", map[string]string{
		"email": "new.hire@example.test", "password": apitest.TestPassword, "name": "New Hire", "invite_token": token,
	}).Decode(t, &reg)
	if reg.VerificationRequired {
		t.Fatal("invited accounts should not need email verification")
	}

	// Logs in straight away, belongs only to the inviting organization, with the invited role.
	newToken := f.api.Login("new.hire@example.test", apitest.TestPassword)
	var orgs []orgResponse
	f.api.MustDo(http.StatusOK, "GET", "/orgs", newToken, nil).Decode(t, &orgs)
	if len(orgs) != 1 || orgs[0].ID != org || orgs[0].Role != "developer" {
		t.Fatalf("unexpected organizations: %+v", orgs)
	}

	// The invitation is single use and no longer listed.
	f.api.MustDo(http.StatusNotFound, "POST", "/invites/lookup", "", map[string]string{"token": token})
	var open []inviteResponse
	f.api.MustDo(http.StatusOK, "GET", "/orgs/"+org+"/invites", f.owner.Token, nil).Decode(t, &open)
	if len(open) != 0 {
		t.Fatalf("accepted invitation still open: %+v", open)
	}

	// Now the owner can put the new member on a vault.
	f.api.MustDo(http.StatusCreated, "POST", "/vaults/"+f.vaultID+"/members", f.owner.Token, map[string]any{"email": "new.hire@example.test", "role": "viewer"})

	var page auditPage
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=invite.accepted", f.owner.Token, nil).Decode(t, &page)
	if page.Total != 1 || page.Data[0].UserEmail != "new.hire@example.test" {
		t.Fatalf("invite.accepted not audited: %+v", page)
	}
}

func TestExistingUserAcceptsInvite(t *testing.T) {
	f := newFixture(t)
	org := f.owner.OrgID.String()
	existing := f.api.CreateUser("Existing Person")
	other := f.api.CreateUser("Other Person")

	f.api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", f.owner.Token, map[string]any{"email": existing.Email, "role": "oncall"})
	token := f.api.Email.WaitForToken(t, existing.Email)

	var lookup struct {
		AccountExists bool `json:"account_exists"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/invites/lookup", "", map[string]string{"token": token}).Decode(t, &lookup)
	if !lookup.AccountExists {
		t.Fatal("lookup should say the account exists")
	}

	f.api.MustDo(http.StatusUnauthorized, "POST", "/invites/accept", "", map[string]string{"token": token})
	f.api.MustDo(http.StatusForbidden, "POST", "/invites/accept", other.Token, map[string]string{"token": token})
	var joined orgResponse
	f.api.MustDo(http.StatusOK, "POST", "/invites/accept", existing.Token, map[string]string{"token": token}).Decode(t, &joined)
	if joined.ID != org || joined.Role != "oncall" {
		t.Fatalf("unexpected organization: %+v", joined)
	}
	f.api.MustDo(http.StatusNotFound, "POST", "/invites/accept", existing.Token, map[string]string{"token": token})

	// Inviting a member again is a conflict.
	f.api.MustDo(http.StatusConflict, "POST", "/orgs/"+org+"/invites", f.owner.Token, map[string]any{"email": existing.Email, "role": "viewer"})
}

func TestInvitePermissionsRevocationAndExpiry(t *testing.T) {
	f := newFixture(t)
	org := f.owner.OrgID.String()
	admin := f.member(t, "Ada Admin", "admin", "", "")
	dev := f.member(t, "Dev Developer", "developer", "", "")

	f.api.MustDo(http.StatusForbidden, "POST", "/orgs/"+org+"/invites", dev.Token, map[string]any{"email": "x@example.test", "role": "viewer"})
	f.api.MustDo(http.StatusForbidden, "GET", "/orgs/"+org+"/invites", dev.Token, nil)
	f.api.MustDo(http.StatusForbidden, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "boss@example.test", "role": "owner"})
	f.api.MustDo(http.StatusBadRequest, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "not-an-email", "role": "viewer"})
	f.api.MustDo(http.StatusBadRequest, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "a@example.test", "role": "root"})

	// Re-inviting replaces the old link.
	var first inviteResponse
	f.api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "twice@example.test", "role": "viewer"}).Decode(t, &first)
	oldToken := f.api.Email.WaitForToken(t, "twice@example.test")
	f.api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "twice@example.test", "role": "developer"})
	newToken := f.api.Email.WaitForToken(t, "twice@example.test")
	if newToken == oldToken {
		t.Fatal("expected a new token")
	}
	f.api.MustDo(http.StatusNotFound, "POST", "/invites/lookup", "", map[string]string{"token": oldToken})

	var open []inviteResponse
	f.api.MustDo(http.StatusOK, "GET", "/orgs/"+org+"/invites", admin.Token, nil).Decode(t, &open)
	if len(open) != 1 || open[0].Role != "developer" {
		t.Fatalf("expected one open invite with the new role, got %+v", open)
	}

	// Revoking stops the link.
	f.api.MustDo(http.StatusNoContent, "DELETE", "/orgs/"+org+"/invites/"+open[0].ID, admin.Token, nil)
	f.api.MustDo(http.StatusNotFound, "DELETE", "/orgs/"+org+"/invites/"+open[0].ID, admin.Token, nil)
	f.api.MustDo(http.StatusNotFound, "POST", "/invites/lookup", "", map[string]string{"token": newToken})

	// Expired links do not work.
	f.api.MustDo(http.StatusCreated, "POST", "/orgs/"+org+"/invites", admin.Token, map[string]any{"email": "late@example.test", "role": "viewer"})
	lateToken := f.api.Email.WaitForToken(t, "late@example.test")
	if _, err := f.api.Pool.Exec(context.Background(), `UPDATE organization_invites SET expires_at = now() - interval '1 minute' WHERE email = 'late@example.test'`); err != nil {
		t.Fatal(err)
	}
	f.api.MustDo(http.StatusBadRequest, "POST", "/auth/register", "", map[string]string{
		"email": "late@example.test", "password": apitest.TestPassword, "name": "Late", "invite_token": lateToken,
	})

	// Tokens are only stored hashed.
	var stored int
	if err := f.api.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM organization_invites WHERE token_hash IN ($1, $2)`, oldToken, newToken).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatal("plaintext invite tokens found in the database")
	}
}
