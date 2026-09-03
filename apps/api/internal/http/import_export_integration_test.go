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
