package http_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type statsResponse struct {
	Vaults                 int `json:"vaults"`
	Environments           int `json:"environments"`
	Secrets                int `json:"secrets"`
	SecretsExpired         int `json:"secrets_expired"`
	SecretsExpiringSoon    int `json:"secrets_expiring_soon"`
	SecretsRotationOverdue int `json:"secrets_rotation_overdue"`
	Users                  int `json:"users"`
	ActiveUsers            int `json:"active_users"`
}

func TestDashboardStatsCountOnlyAccessibleData(t *testing.T) {
	f := newFixture(t) // 1 vault, 1 environment, 1 secret
	staging := f.createEnv(t, f.vaultID, "staging")

	soon := time.Now().Add(5 * 24 * time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+staging+"/secrets", f.owner.Token, map[string]any{"key_name": "SOON", "value": "x", "expires_at": soon})
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+staging+"/secrets", f.owner.Token, map[string]any{"key_name": "EXPIRED", "value": "x", "expires_at": past})
	overdue := f.createSecret(t, staging, "OVERDUE", "x")
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+overdue, f.owner.Token, map[string]any{"rotation_interval_days": 30})
	if _, err := f.api.Pool.Exec(context.Background(),
		`UPDATE secrets SET created_at = now() - interval '45 days' WHERE id = $1`, overdue); err != nil {
		t.Fatal(err)
	}
	// Not yet due: rotated today with a 30 day interval.
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+staging+"/secrets", f.owner.Token, map[string]any{"key_name": "FRESH", "value": "x", "rotation_interval_days": 30})

	viewer := f.member(t, "Val Viewer", "developer", f.vaultID, "viewer")
	// A teammate with access who has never logged in (inserted directly).
	idle := f.member(t, "Ian Idle", "developer", f.vaultID, "viewer")
	if _, err := f.api.Pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE user_id = $1 AND action = 'login.success'`, idle.ID); err != nil {
		t.Fatal(err)
	}

	// Someone else's vault must not be counted.
	other := f.api.CreateUser("Other Team")
	var otherVault idResponse
	f.api.MustDo(http.StatusCreated, "POST", "/vaults", other.Token, map[string]any{"name": "theirs"}).Decode(t, &otherVault)

	var stats statsResponse
	f.api.MustDo(http.StatusOK, "GET", "/stats", f.owner.Token, nil).Decode(t, &stats)
	want := statsResponse{
		Vaults: 1, Environments: 2, Secrets: 5,
		SecretsExpired: 1, SecretsExpiringSoon: 1, SecretsRotationOverdue: 1,
		Users: 3, ActiveUsers: 2,
	}
	if stats != want {
		t.Fatalf("stats: want %+v, got %+v", want, stats)
	}

	var viewerStats statsResponse
	f.api.MustDo(http.StatusOK, "GET", "/stats", viewer.Token, nil).Decode(t, &viewerStats)
	if viewerStats.Vaults != 1 || viewerStats.Secrets != 5 {
		t.Errorf("viewer should see the shared vault's counts, got %+v", viewerStats)
	}

	f.api.MustDo(http.StatusUnauthorized, "GET", "/stats", "", nil)
}

func TestHealthReportsDatabaseAndSchema(t *testing.T) {
	f := newFixture(t)
	var health struct {
		Status           string `json:"status"`
		Database         string `json:"database"`
		MigrationVersion *int   `json:"migration_version"`
		MigrationDirty   bool   `json:"migration_dirty"`
		StartedAt        string `json:"started_at"`
		Version          string `json:"version"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/health", "", nil).Decode(t, &health)
	if health.Status != "ok" || health.Database != "ok" || health.MigrationVersion == nil || *health.MigrationVersion < 13 || health.MigrationDirty || health.StartedAt == "" || health.Version != "dev" {
		t.Fatalf("unexpected health: %+v", health)
	}

	if _, err := f.api.Pool.Exec(context.Background(), `UPDATE schema_migrations SET dirty = true`); err != nil {
		t.Fatal(err)
	}
	f.api.MustDo(http.StatusServiceUnavailable, "GET", "/health", "", nil)
}
