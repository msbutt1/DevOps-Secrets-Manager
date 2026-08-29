package http_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRequestBodiesAreValidated(t *testing.T) {
	f := newFixture(t)
	secrets := "/envs/" + f.envID + "/secrets"

	cases := []struct {
		name   string
		body   any
		status int
		code   string
	}{
		{"unknown field", map[string]any{"key_name": "A", "value": "x", "is_admin": true}, 400, "invalid_request"},
		{"wrong type", map[string]any{"key_name": 42, "value": "x"}, 400, "invalid_request"},
		{"bad key name", map[string]any{"key_name": "has-dash", "value": "x"}, 400, "validation_failed"},
		{"value too large", map[string]any{"key_name": "BIG", "value": strings.Repeat("v", 64*1024+1)}, 400, "validation_failed"},
		{"metadata must be strings", map[string]any{"key_name": "META", "value": "x", "metadata": map[string]any{"n": 1}}, 400, "validation_failed"},
		{"rotation out of range", map[string]any{"key_name": "ROT", "value": "x", "rotation_interval_days": 0}, 400, "validation_failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := f.api.Do("POST", secrets, f.owner.Token, c.body)
			if resp.Status != c.status || !strings.Contains(string(resp.Body), `"error":"`+c.code+`"`) {
				t.Fatalf("want %d %s, got %d %s", c.status, c.code, resp.Status, resp.Body)
			}
		})
	}

	raw := func(body string) (int, string) {
		req, _ := http.NewRequest("POST", f.api.URL+secrets, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+f.owner.Token)
		res, err := f.api.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if status, body := raw(`{"key_name":"A","value":"x"}{"key_name":"B","value":"y"}`); status != 400 {
		t.Errorf("trailing JSON: want 400, got %d %s", status, body)
	}
	if status, body := raw(`not json`); status != 400 {
		t.Errorf("malformed JSON: want 400, got %d %s", status, body)
	}
	if status, body := raw(`{"key_name":"HUGE","value":"` + strings.Repeat("x", 2<<20) + `"}`); status != 413 {
		t.Errorf("oversized body: want 413, got %d %.80s", status, body)
	}

	// Registration and vault names are bounded too
	f.api.MustDo(400, "POST", "/auth/register", "", map[string]string{"email": "long@example.test", "password": strings.Repeat("p", 73), "name": "Long"})
	f.api.MustDo(400, "POST", "/auth/register", "", map[string]string{"email": "not-an-email", "password": "Some-Passw0rd!", "name": "X"})
	f.api.MustDo(400, "POST", "/vaults", f.owner.Token, map[string]any{"name": strings.Repeat("v", 101)})

	// Nothing invalid was stored
	var list []struct {
		KeyName string `json:"key_name"`
	}
	f.api.MustDo(200, "GET", secrets, f.owner.Token, nil).Decode(t, &list)
	if len(list) != 1 {
		t.Fatalf("expected only the fixture secret, got %+v", list)
	}
}
