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
