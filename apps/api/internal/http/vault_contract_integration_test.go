package http_test

import (
	"net/http"
	"testing"
)

func TestVaultResponseIncludesCreator(t *testing.T) {
	f := newFixture(t)
	viewer := f.member(t, "Vince Viewer", "developer", f.vaultID, "viewer")

	var vault struct {
		CreatedBy   string `json:"created_by"`
		CreatedByID string `json:"created_by_id"`
		CreatedAt   string `json:"created_at"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/vaults/"+f.vaultID, viewer.Token, nil).Decode(t, &vault)
	if vault.CreatedBy != "Olive Owner" || vault.CreatedByID != f.owner.ID.String() || vault.CreatedAt == "" {
		t.Fatalf("unexpected creator fields: %+v", vault)
	}
}

func TestMemberResponseIncludesWhoAddedThem(t *testing.T) {
	f := newFixture(t)
	admin := f.member(t, "Ada Admin", "developer", f.vaultID, "admin")
	guest := f.member(t, "Gus Guest", "developer", "", "")

	type member struct {
		Email     string  `json:"email"`
		AddedBy   string  `json:"added_by"`
		AddedByID *string `json:"added_by_id"`
	}
	var added member
	f.api.MustDo(http.StatusCreated, "POST", "/vaults/"+f.vaultID+"/members", admin.Token,
		map[string]any{"email": guest.Email, "role": "viewer"}).Decode(t, &added)
	if added.AddedBy != "Ada Admin" || added.AddedByID == nil || *added.AddedByID != admin.ID.String() {
		t.Fatalf("unexpected added_by on create: %+v", added)
	}

	var members []member
	f.api.MustDo(http.StatusOK, "GET", "/vaults/"+f.vaultID+"/members", f.owner.Token, nil).Decode(t, &members)
	want := map[string]string{f.owner.Email: "Olive Owner", admin.Email: "Olive Owner", guest.Email: "Ada Admin"}
	for _, m := range members {
		if m.AddedBy != want[m.Email] {
			t.Errorf("%s added_by: want %q, got %q", m.Email, want[m.Email], m.AddedBy)
		}
	}

	var errBody struct {
		Message string `json:"message"`
	}
	f.api.MustDo(http.StatusNotFound, "POST", "/vaults/"+f.vaultID+"/members", f.owner.Token,
		map[string]any{"email": "ghost@example.test", "role": "viewer"}).Decode(t, &errBody)
	if errBody.Message != "User not found with that email" {
		t.Errorf("unexpected message for unknown email: %q", errBody.Message)
	}
}
