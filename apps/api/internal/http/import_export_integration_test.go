package http_test

import (
	"net/http"
	"sort"
	"testing"
)

type importResult struct {
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Unchanged []string `json:"unchanged"`
	Skipped   []string `json:"skipped"`
	DryRun    bool     `json:"dry_run"`
}

func TestImportAndExportEnvironment(t *testing.T) {
	f := newFixture(t) // DATABASE_URL = postgres://fixture
	f.createSecret(t, f.envID, "SAME", "unchanged-value")
	body := func(overwrite, dryRun bool) map[string]any {
		return map[string]any{
			"secrets": []map[string]string{
				{"key": "DATABASE_URL", "value": "postgres://imported"},
				{"key": "SAME", "value": "unchanged-value"},
				{"key": "NEW_KEY", "value": "brand new"},
			},
			"overwrite": overwrite,
			"dry_run":   dryRun,
		}
	}

	// A dry run without overwrite previews creates and skips and writes nothing.
	var preview importResult
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/import", f.owner.Token, body(false, true)).Decode(t, &preview)
	if !preview.DryRun || len(preview.Created) != 1 || len(preview.Skipped) != 2 {
		t.Fatalf("preview: %+v", preview)
	}
	var list []struct {
		KeyName string `json:"key_name"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/envs/"+f.envID+"/secrets", f.owner.Token, nil).Decode(t, &list)
	if len(list) != 2 {
		t.Fatalf("dry run wrote secrets: %+v", list)
	}

	// With overwrite: create, update, and leave identical values alone.
	var done importResult
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/import", f.owner.Token, body(true, false)).Decode(t, &done)
	if len(done.Created) != 1 || done.Created[0] != "NEW_KEY" || len(done.Updated) != 1 || done.Updated[0] != "DATABASE_URL" || len(done.Unchanged) != 1 || done.DryRun {
		t.Fatalf("import: %+v", done)
	}
	if got := revealValue(t, f, f.secretID); got != "postgres://imported" {
		t.Fatalf("updated value: %q", got)
	}
	if n := len(listVersions(t, f, f.owner.Token, f.secretID)); n != 2 {
		t.Fatalf("DATABASE_URL should have 2 versions, has %d", n)
	}

	// Export returns every value and is audited once.
	var exported struct {
		Secrets []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"secrets"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/export", f.owner.Token, nil).Decode(t, &exported)
	values := map[string]string{}
	for _, s := range exported.Secrets {
		values[s.Key] = s.Value
	}
	if len(values) != 3 || values["NEW_KEY"] != "brand new" || values["DATABASE_URL"] != "postgres://imported" {
		t.Fatalf("export: %+v", values)
	}
	var page struct {
		Data []struct {
			Action   string         `json:"action"`
			Metadata map[string]any `json:"metadata"`
		} `json:"data"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=env.exported", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 1 {
		t.Fatalf("export audit: %+v", page.Data)
	}
	keys := []string{}
	for _, k := range page.Data[0].Metadata["keys"].([]any) {
		keys = append(keys, k.(string))
	}
	sort.Strings(keys)
	if len(keys) != 3 {
		t.Fatalf("export audit keys: %v", keys)
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=env.imported", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 1 {
		t.Fatalf("import audit: %+v", page.Data)
	}

	// Validation and permissions.
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/import", f.owner.Token, map[string]any{"secrets": []map[string]string{}})
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/import", f.owner.Token, map[string]any{"secrets": []map[string]string{{"key": "1BAD", "value": "x"}}})
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/import", f.owner.Token, map[string]any{"secrets": []map[string]string{{"key": "A", "value": "1"}, {"key": "A", "value": "2"}}})
	dev := f.member(t, "Dev Eloper", "developer", f.vaultID, "developer")
	f.api.MustDo(http.StatusForbidden, "POST", "/envs/"+f.envID+"/export", dev.Token, nil)
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/import", dev.Token, map[string]any{"secrets": []map[string]string{{"key": "DEV_KEY", "value": "x"}}})
	oncall := f.member(t, "On Call", "developer", f.vaultID, "oncall")
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/export", oncall.Token, nil)
	f.api.MustDo(http.StatusForbidden, "POST", "/envs/"+f.envID+"/import", oncall.Token, map[string]any{"secrets": []map[string]string{{"key": "X", "value": "x"}}})
	stranger := f.api.CreateUser("Stan Stranger")
	f.api.MustDo(http.StatusNotFound, "POST", "/envs/"+f.envID+"/export", stranger.Token, nil)
}

func TestCopySecretsBetweenEnvironments(t *testing.T) {
	f := newFixture(t) // production holds DATABASE_URL = postgres://fixture
	staging := f.createEnv(t, f.vaultID, "staging")
	f.createSecret(t, staging, "DATABASE_URL", "postgres://staging")
	f.createSecret(t, staging, "STAGING_ONLY", "only-here")

	copyBody := func(keys []string, overwrite, dryRun bool) map[string]any {
		return map[string]any{
			"source_environment_id": staging, "keys": keys, "overwrite": overwrite, "dry_run": dryRun,
		}
	}

	// A dry run of the selected keys reports without writing.
	var preview importResult
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/copy-from", f.owner.Token, copyBody([]string{"STAGING_ONLY"}, false, true)).Decode(t, &preview)
	if len(preview.Created) != 1 || preview.Created[0] != "STAGING_ONLY" {
		t.Fatalf("preview: %+v", preview)
	}
	if got := revealValue(t, f, f.secretID); got != "postgres://fixture" {
		t.Fatalf("dry run changed a value: %q", got)
	}

	// Copying everything without overwrite creates the new key and skips the existing one.
	var done importResult
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/copy-from", f.owner.Token, copyBody(nil, false, false)).Decode(t, &done)
	if len(done.Created) != 1 || len(done.Skipped) != 1 || done.Skipped[0] != "DATABASE_URL" {
		t.Fatalf("copy without overwrite: %+v", done)
	}

	// With overwrite the existing key takes the source value, re-encrypted for the target.
	f.api.MustDo(http.StatusOK, "POST", "/envs/"+f.envID+"/copy-from", f.owner.Token, copyBody(nil, true, false)).Decode(t, &done)
	if len(done.Updated) != 1 || done.Updated[0] != "DATABASE_URL" || len(done.Unchanged) != 1 {
		t.Fatalf("copy with overwrite: %+v", done)
	}
	if got := revealValue(t, f, f.secretID); got != "postgres://staging" {
		t.Fatalf("value after copy: %q", got)
	}

	// Both sides are audited: a read on the source and a write on the target.
	var page struct {
		Data []struct {
			Metadata map[string]any `json:"metadata"`
		} `json:"data"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=env.imported", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 2 || page.Data[0].Metadata["copied_from"] != "staging" {
		t.Fatalf("copy not audited on the target: %+v", page.Data)
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=env.exported", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 2 || page.Data[0].Metadata["via"] != "copy" {
		t.Fatalf("copy not audited on the source: %+v", page.Data)
	}

	// Rejected: same environment, unknown keys, another vault, and roles without permission.
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+staging+"/copy-from", f.owner.Token, map[string]any{"source_environment_id": staging})
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/copy-from", f.owner.Token, copyBody([]string{"NOT_THERE"}, true, true))
	otherEnv := f.createEnv(t, f.createVault(t, "other-vault"), "production")
	f.api.MustDo(http.StatusBadRequest, "POST", "/envs/"+f.envID+"/copy-from", f.owner.Token, map[string]any{"source_environment_id": otherEnv, "overwrite": true})

	dev := f.member(t, "Dev Eloper", "developer", f.vaultID, "developer")
	f.api.MustDo(http.StatusForbidden, "POST", "/envs/"+f.envID+"/copy-from", dev.Token, copyBody(nil, true, true))
	oncall := f.member(t, "On Call", "developer", f.vaultID, "oncall")
	f.api.MustDo(http.StatusForbidden, "POST", "/envs/"+f.envID+"/copy-from", oncall.Token, copyBody(nil, true, true))
	stranger := f.api.CreateUser("Stan Stranger")
	f.api.MustDo(http.StatusNotFound, "POST", "/envs/"+f.envID+"/copy-from", stranger.Token, copyBody(nil, true, true))
}
