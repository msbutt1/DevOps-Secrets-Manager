package http_test

import (
	"net/http"
	"testing"
)

type secretVersion struct {
	Version      int    `json:"version"`
	CreatedBy    string `json:"created_by"`
	RestoredFrom *int   `json:"restored_from"`
	Current      bool   `json:"current"`
}

func listVersions(t *testing.T, f *fixture, token, secretID string) []secretVersion {
	t.Helper()
	var versions []secretVersion
	f.api.MustDo(http.StatusOK, "GET", "/secrets/"+secretID+"/versions", token, nil).Decode(t, &versions)
	return versions
}

func TestSecretVersionHistoryAndRollback(t *testing.T) {
	f := newFixture(t) // DATABASE_URL = postgres://fixture, version 1
	secret := f.secretID

	// Value changes add versions; metadata-only edits do not.
	var updated struct {
		Version int `json:"version"`
	}
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+secret, f.owner.Token, map[string]any{"value": "postgres://v2"}).Decode(t, &updated)
	if updated.Version != 2 {
		t.Fatalf("version after value change: %d", updated.Version)
	}
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+secret, f.owner.Token, map[string]any{"description": "metadata only"}).Decode(t, &updated)
	if updated.Version != 2 {
		t.Fatalf("metadata edit changed the version to %d", updated.Version)
	}
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+secret, f.owner.Token, map[string]any{"value": "postgres://broken"})

	versions := listVersions(t, f, f.owner.Token, secret)
	if len(versions) != 3 || versions[0].Version != 3 || !versions[0].Current || versions[2].Version != 1 || versions[2].CreatedBy != "Olive Owner" {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	// Earlier values can be revealed (audited with the version) and restored.
	var revealed struct {
		Value string `json:"value"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secret+"/versions/2/reveal", f.owner.Token, nil).Decode(t, &revealed)
	if revealed.Value != "postgres://v2" {
		t.Fatalf("version 2 value: %q", revealed.Value)
	}
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secret+"/versions/2/restore", f.owner.Token, nil).Decode(t, &updated)
	if updated.Version != 4 {
		t.Fatalf("restore should add version 4, got %d", updated.Version)
	}
	if got := revealValue(t, f, secret); got != "postgres://v2" {
		t.Fatalf("value after restore: %q", got)
	}
	versions = listVersions(t, f, f.owner.Token, secret)
	if versions[0].RestoredFrom == nil || *versions[0].RestoredFrom != 2 {
		t.Fatalf("restored version not marked: %+v", versions[0])
	}
	f.api.MustDo(http.StatusConflict, "POST", "/secrets/"+secret+"/versions/4/restore", f.owner.Token, nil)
	f.api.MustDo(http.StatusNotFound, "POST", "/secrets/"+secret+"/versions/9/restore", f.owner.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "POST", "/secrets/"+secret+"/versions/zero/restore", f.owner.Token, nil)

	var page struct {
		Data []struct {
			Action   string         `json:"action"`
			Metadata map[string]any `json:"metadata"`
		} `json:"data"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=secret.restored", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 1 || page.Data[0].Metadata["restored_version"] != float64(2) || page.Data[0].Metadata["version"] != float64(4) {
		t.Fatalf("restore not audited: %+v", page.Data)
	}

	// History survives a data key rotation.
	f.api.MustDo(http.StatusOK, "POST", "/vaults/"+f.vaultID+"/rotate-key", f.owner.Token, nil)
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secret+"/versions/1/reveal", f.owner.Token, nil).Decode(t, &revealed)
	if revealed.Value != "postgres://fixture" {
		t.Fatalf("version 1 after key rotation: %q", revealed.Value)
	}
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secret+"/versions/3/restore", f.owner.Token, nil)
	if got := revealValue(t, f, secret); got != "postgres://broken" {
		t.Fatalf("restore after key rotation: %q", got)
	}

	// Permissions: viewers see history but cannot reveal or restore; developers restore but not reveal; outsiders get 404.
	viewer := f.member(t, "Vera Viewer", "viewer", f.vaultID, "viewer")
	if len(listVersions(t, f, viewer.Token, secret)) != 5 {
		t.Fatal("viewer cannot list versions")
	}
	f.api.MustDo(http.StatusForbidden, "POST", "/secrets/"+secret+"/versions/1/reveal", viewer.Token, nil)
	f.api.MustDo(http.StatusForbidden, "POST", "/secrets/"+secret+"/versions/1/restore", viewer.Token, nil)
	dev := f.member(t, "Dev Eloper", "developer", f.vaultID, "developer")
	f.api.MustDo(http.StatusForbidden, "POST", "/secrets/"+secret+"/versions/1/reveal", dev.Token, nil)
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secret+"/versions/1/restore", dev.Token, nil)
	stranger := f.api.CreateUser("Stan Stranger")
	f.api.MustDo(http.StatusNotFound, "GET", "/secrets/"+secret+"/versions", stranger.Token, nil)
	f.api.MustDo(http.StatusNotFound, "POST", "/secrets/"+secret+"/versions/1/reveal", stranger.Token, nil)
}
