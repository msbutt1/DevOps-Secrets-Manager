package http_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAuditLogFilters(t *testing.T) {
	f := newFixture(t)
	otherVault := f.createVault(t, "other-vault")
	otherEnv := f.createEnv(t, otherVault, "staging")
	f.createSecret(t, otherEnv, "OTHER_KEY", "x")
	teammate := f.member(t, "Theo Teammate", "developer", f.vaultID, "oncall")
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", teammate.Token, nil)
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+f.secretID+"/reveal", f.owner.Token, nil)

	// Move one old event far into the past to exercise the date filters.
	if _, err := f.api.Pool.Exec(context.Background(),
		`UPDATE audit_logs SET timestamp = '2020-01-15T12:00:00Z' WHERE action = 'vault.created' AND target_name = 'other-vault'`); err != nil {
		t.Fatal(err)
	}

	all := func(t *testing.T, query string, check func(e auditEvent) bool) int {
		t.Helper()
		page := getAudit(t, f, f.owner.Token, "?limit=200&"+query)
		for _, e := range page.Data {
			if !check(e) {
				t.Errorf("%s returned unexpected event %+v", query, e)
			}
		}
		if page.Total != len(page.Data) {
			t.Errorf("%s: total %d, got %d rows", query, page.Total, len(page.Data))
		}
		return page.Total
	}

	t.Run("vaultId", func(t *testing.T) {
		if n := all(t, "vaultId="+otherVault, func(e auditEvent) bool { return e.VaultName != nil && *e.VaultName == "other-vault" }); n != 3 {
			t.Errorf("want 3 events (vault, env, secret created), got %d", n)
		}
	})
	t.Run("environmentId", func(t *testing.T) {
		if n := all(t, "environmentId="+otherEnv, func(e auditEvent) bool { return e.EnvironmentName != nil && *e.EnvironmentName == "staging" }); n != 2 {
			t.Errorf("want 2 events, got %d", n)
		}
	})
	t.Run("userId", func(t *testing.T) {
		if n := all(t, "userId="+teammate.ID.String(), func(e auditEvent) bool { return e.UserEmail == teammate.Email }); n != 1 {
			t.Errorf("want the teammate's reveal only (their login is not in a vault the owner administers), got %d", n)
		}
	})
	t.Run("userEmail substring", func(t *testing.T) {
		part := strings.ToUpper(strings.Split(teammate.Email, "+")[0])
		if n := all(t, "userEmail="+url.QueryEscape(part), func(e auditEvent) bool { return e.UserEmail == teammate.Email }); n != 1 {
			t.Errorf("want 1 event, got %d", n)
		}
	})
	t.Run("action", func(t *testing.T) {
		if n := all(t, "action=secret.revealed", func(e auditEvent) bool { return e.Action == "secret.revealed" }); n != 2 {
			t.Errorf("want 2 reveals, got %d", n)
		}
	})
	t.Run("excludeAction", func(t *testing.T) {
		n := all(t, "excludeAction=login.success&excludeAction=secret.revealed", func(e auditEvent) bool {
			return e.Action != "login.success" && e.Action != "secret.revealed"
		})
		if n == 0 {
			t.Error("expected remaining events")
		}
	})
	t.Run("date range", func(t *testing.T) {
		if n := all(t, "startDate=2020-01-01&endDate=2020-01-15", func(e auditEvent) bool { return e.Action == "vault.created" }); n != 1 {
			t.Errorf("endDate as a day must include that whole day; got %d", n)
		}
		if n := all(t, "endDate=2020-01-14", func(e auditEvent) bool { return false }); n != 0 {
			t.Errorf("want nothing before the old event, got %d", n)
		}
		today := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
		if n := all(t, "startDate="+url.QueryEscape(today)+"&vaultId="+otherVault, func(e auditEvent) bool { return true }); n != 2 {
			t.Errorf("want 2 recent events in other-vault, got %d", n)
		}
	})
	t.Run("invalid values", func(t *testing.T) {
		for _, q := range []string{"startDate=yesterday", "environmentId=abc", "userId=1", "limit=-1"} {
			f.api.MustDo(http.StatusBadRequest, "GET", "/audit?"+q, f.owner.Token, nil)
		}
	})
}
