package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"go.uber.org/zap"
)

// accessibleVaultsCTE selects the IDs of live vaults user $1 can access: vaults they are a
// member of while still in the vault's organization, and every vault in organizations they own
// or administer. It matches policy.EffectiveVaultRole.
const accessibleVaultsCTE = `
	accessible AS (
		SELECT v.id, v.organization_id FROM vaults v
		JOIN user_organizations uo ON uo.organization_id = v.organization_id AND uo.user_id = $1
		WHERE v.deleted_at IS NULL
		  AND (uo.role IN ('owner', 'admin')
		       OR EXISTS (SELECT 1 FROM vault_members vm WHERE vm.vault_id = v.id AND vm.user_id = $1))
	)`

const (
	// expiringSoonWindow is how far ahead a secret's expiry counts as "expiring soon".
	expiringSoonWindow = 30 * 24 * time.Hour
	// activeUserWindow is how recently someone must have logged in to count as active.
	activeUserWindow = 30 * 24 * time.Hour
)

// StatsResponse holds dashboard counts across the vaults the caller can access
type StatsResponse struct {
	Vaults                 int `json:"vaults"`
	Environments           int `json:"environments"`
	Secrets                int `json:"secrets"`
	SecretsExpired         int `json:"secrets_expired"`
	SecretsExpiringSoon    int `json:"secrets_expiring_soon"`
	SecretsRotationOverdue int `json:"secrets_rotation_overdue"`
	// Users counts people with access to those vaults; ActiveUsers those who logged in recently.
	Users            int `json:"users"`
	ActiveUsers      int `json:"active_users"`
	ExpiringSoonDays int `json:"expiring_soon_days"`
	ActiveWindowDays int `json:"active_window_days"`
}

// StatsHandlers serves dashboard statistics and the health check
type StatsHandlers struct {
	db        *pgxpool.Pool
	logger    *zap.Logger
	startedAt time.Time
	version   string
}

// NewStatsHandlers creates a new instance of StatsHandlers
func NewStatsHandlers(db *pgxpool.Pool, logger *zap.Logger, startedAt time.Time, version string) *StatsHandlers {
	return &StatsHandlers{db: db, logger: logger, startedAt: startedAt, version: version}
}

// HandleStats handles GET /stats
func (h *StatsHandlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	stats := StatsResponse{
		ExpiringSoonDays: int(expiringSoonWindow.Hours() / 24),
		ActiveWindowDays: int(activeUserWindow.Hours() / 24),
	}

	secretsQuery := `WITH` + accessibleVaultsCTE + `
		SELECT
			(SELECT COUNT(*) FROM accessible),
			COUNT(DISTINCT e.id),
			COUNT(s.id),
			COUNT(s.id) FILTER (WHERE s.expires_at <= now()),
			COUNT(s.id) FILTER (WHERE s.expires_at > now() AND s.expires_at <= now() + $2::interval),
			COUNT(s.id) FILTER (WHERE s.rotation_interval_days > 0
				AND COALESCE(s.last_rotated_at, s.created_at) + make_interval(days => s.rotation_interval_days) <= now())
		FROM accessible a
		LEFT JOIN environments e ON e.vault_id = a.id AND e.deleted_at IS NULL
		LEFT JOIN secrets s ON s.environment_id = e.id AND s.deleted_at IS NULL
	`
	if err := h.db.QueryRow(r.Context(), secretsQuery, claims.UserID, expiringSoonWindow).Scan(
		&stats.Vaults, &stats.Environments, &stats.Secrets,
		&stats.SecretsExpired, &stats.SecretsExpiringSoon, &stats.SecretsRotationOverdue,
	); err != nil {
		h.logger.Error("Failed to compute secret statistics", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load statistics")
		return
	}

	usersQuery := `WITH` + accessibleVaultsCTE + `,
		people AS (
			SELECT vm.user_id FROM vault_members vm
			JOIN accessible a ON a.id = vm.vault_id
			JOIN user_organizations uo ON uo.organization_id = a.organization_id AND uo.user_id = vm.user_id
			UNION
			SELECT uo.user_id FROM user_organizations uo
			JOIN accessible a ON a.organization_id = uo.organization_id
			WHERE uo.role IN ('owner', 'admin')
		)
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE EXISTS (
				SELECT 1 FROM audit_logs al
				WHERE al.user_id = p.user_id AND al.action = 'login.success' AND al.timestamp > now() - $2::interval))
		FROM people p
	`
	if err := h.db.QueryRow(r.Context(), usersQuery, claims.UserID, activeUserWindow).Scan(&stats.Users, &stats.ActiveUsers); err != nil {
		h.logger.Error("Failed to compute user statistics", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load statistics")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// HealthResponse reports whether the API can serve requests
type HealthResponse struct {
	Status           string    `json:"status"` // "ok" or "unavailable"
	Database         string    `json:"database"`
	MigrationVersion *uint     `json:"migration_version"`
	MigrationDirty   bool      `json:"migration_dirty"`
	StartedAt        time.Time `json:"started_at"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	Version          string    `json:"version"`
	Timestamp        time.Time `json:"timestamp"`
}

// HandleHealth handles GET /health. It pings the database and reads the schema version, and
// answers 503 when the database is unreachable or a migration failed part-way.
func (h *StatsHandlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	resp := HealthResponse{
		Status:        "ok",
		Database:      "ok",
		StartedAt:     h.startedAt,
		UptimeSeconds: int64(now.Sub(h.startedAt).Seconds()),
		Version:       h.version,
		Timestamp:     now,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		h.logger.Warn("Health check: database ping failed", zap.Error(err))
		resp.Status, resp.Database = "unavailable", "unreachable"
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}

	var version int64
	err := h.db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &resp.MigrationDirty)
	switch {
	case err == nil:
		v := uint(version)
		resp.MigrationVersion = &v
	case errors.Is(err, pgx.ErrNoRows):
	default:
		h.logger.Warn("Health check: reading schema version failed", zap.Error(err))
	}

	status := http.StatusOK
	if resp.MigrationDirty {
		resp.Status = "unavailable"
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
