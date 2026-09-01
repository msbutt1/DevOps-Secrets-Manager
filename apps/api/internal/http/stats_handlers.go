package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
)

// accessibleVaultsCTE selects the IDs of live vaults user $1 can access, limited to organization
// $2 when it is not NULL: vaults they are a
// member of while still in the vault's organization, and every vault in organizations they own
// or administer. It matches policy.EffectiveVaultRole.
const accessibleVaultsCTE = `
	accessible AS (
		SELECT v.id, v.organization_id FROM vaults v
		JOIN user_organizations uo ON uo.organization_id = v.organization_id AND uo.user_id = $1
		WHERE v.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR v.organization_id = $2::uuid)
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
	logger    *slog.Logger
	startedAt time.Time
	version   string
}

// NewStatsHandlers creates a new instance of StatsHandlers
func NewStatsHandlers(db *pgxpool.Pool, logger *slog.Logger, startedAt time.Time, version string) *StatsHandlers {
	return &StatsHandlers{db: db, logger: logger, startedAt: startedAt, version: version}
}

// HandleStats handles GET /stats
func (h *StatsHandlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	orgFilter, ok := organizationFilter(w, r)
	if !ok {
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
			COUNT(s.id) FILTER (WHERE s.expires_at > now() AND s.expires_at <= now() + $3::interval),
			COUNT(s.id) FILTER (WHERE s.rotation_interval_days > 0
				AND COALESCE(s.last_rotated_at, s.created_at) + make_interval(days => s.rotation_interval_days) <= now())
		FROM accessible a
		LEFT JOIN environments e ON e.vault_id = a.id AND e.deleted_at IS NULL
		LEFT JOIN secrets s ON s.environment_id = e.id AND s.deleted_at IS NULL
	`
	if err := h.db.QueryRow(r.Context(), secretsQuery, claims.UserID, orgFilter, expiringSoonWindow).Scan(
		&stats.Vaults, &stats.Environments, &stats.Secrets,
		&stats.SecretsExpired, &stats.SecretsExpiringSoon, &stats.SecretsRotationOverdue,
	); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to compute secret statistics", slog.Any("error", err))
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
				WHERE al.user_id = p.user_id AND al.action = 'login.success' AND al.timestamp > now() - $3::interval))
		FROM people p
	`
	if err := h.db.QueryRow(r.Context(), usersQuery, claims.UserID, orgFilter, activeUserWindow).Scan(&stats.Users, &stats.ActiveUsers); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to compute user statistics", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load statistics")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

const (
	// alertExpiringWindow is how far ahead an expiring secret raises an alert.
	alertExpiringWindow = 7 * 24 * time.Hour
	// inactiveMemberWindow is how long without a login before a member is flagged.
	inactiveMemberWindow = 90 * 24 * time.Hour
	maxAlerts            = 50
)

// Alert is something on the dashboard that needs attention
type Alert struct {
	// Type is secret_expired, secret_expiring, rotation_overdue or member_inactive.
	Type            string     `json:"type"`
	Severity        string     `json:"severity"` // high, medium or low
	Message         string     `json:"message"`
	VaultID         string     `json:"vault_id"`
	VaultName       string     `json:"vault_name"`
	EnvironmentName *string    `json:"environment_name"`
	TargetID        string     `json:"target_id"`
	TargetName      string     `json:"target_name"`
	DueAt           *time.Time `json:"due_at"`
}

// HandleAlerts handles GET /alerts: expired or soon-expiring secrets and overdue rotations in
// vaults the caller can access, and members without a recent login in vaults where the caller
// manages members. Results are ordered by severity, then due date, and capped at 50.
func (h *StatsHandlers) HandleAlerts(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	orgFilter, ok := organizationFilter(w, r)
	if !ok {
		return
	}

	query := `WITH` + accessibleVaultsCTE + `,
		secret_rows AS (
			SELECT v.id AS vault_id, v.name AS vault_name, e.name AS env_name, s.id AS secret_id, s.key_name,
				s.expires_at,
				CASE WHEN s.rotation_interval_days > 0
					THEN COALESCE(s.last_rotated_at, s.created_at) + make_interval(days => s.rotation_interval_days)
				END AS rotation_due
			FROM accessible a
			JOIN vaults v ON v.id = a.id
			JOIN environments e ON e.vault_id = v.id AND e.deleted_at IS NULL
			JOIN secrets s ON s.environment_id = e.id AND s.deleted_at IS NULL
		),
		managed AS (
			SELECT a.id, a.organization_id FROM accessible a
			JOIN user_organizations uo ON uo.organization_id = a.organization_id AND uo.user_id = $1
			LEFT JOIN vault_members vm ON vm.vault_id = a.id AND vm.user_id = $1
			WHERE uo.role IN ('owner', 'admin') OR vm.role IN ('owner', 'admin')
		),
		alerts AS (
			SELECT 'secret_expired' AS type, 1 AS rank, vault_id, vault_name, env_name, secret_id::text AS target_id, key_name AS target_name, expires_at AS due_at
			FROM secret_rows WHERE expires_at <= now()
			UNION ALL
			SELECT 'secret_expiring', 2, vault_id, vault_name, env_name, secret_id::text, key_name, expires_at
			FROM secret_rows WHERE expires_at > now() AND expires_at <= now() + $3::interval
			UNION ALL
			SELECT 'rotation_overdue', 2, vault_id, vault_name, env_name, secret_id::text, key_name, rotation_due
			FROM secret_rows WHERE rotation_due <= now()
			UNION ALL
			SELECT 'member_inactive', 3, v.id, v.name, NULL, u.id::text, u.email,
				(SELECT MAX(al.timestamp) FROM audit_logs al WHERE al.user_id = u.id AND al.action = 'login.success')
			FROM managed m
			JOIN vaults v ON v.id = m.id
			JOIN vault_members vm ON vm.vault_id = m.id
			JOIN user_organizations muo ON muo.organization_id = m.organization_id AND muo.user_id = vm.user_id
			JOIN users u ON u.id = vm.user_id
			WHERE u.id <> $1 AND NOT EXISTS (
				SELECT 1 FROM audit_logs al
				WHERE al.user_id = u.id AND al.action = 'login.success' AND al.timestamp > now() - $4::interval)
		)
		SELECT type, vault_id, vault_name, env_name, target_id, target_name, due_at
		FROM alerts
		ORDER BY rank, due_at NULLS FIRST, vault_name, target_name
		LIMIT $5
	`
	rows, err := h.db.Query(r.Context(), query, claims.UserID, orgFilter, alertExpiringWindow, inactiveMemberWindow, maxAlerts)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to load alerts", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load alerts")
		return
	}
	defer rows.Close()

	alerts := make([]Alert, 0)
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.Type, &a.VaultID, &a.VaultName, &a.EnvironmentName, &a.TargetID, &a.TargetName, &a.DueAt); err != nil {
			h.logger.ErrorContext(r.Context(), "Failed to scan alert", slog.Any("error", err))
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load alerts")
			return
		}
		a.Severity, a.Message = describeAlert(a)
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to read alerts", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load alerts")
		return
	}

	writeJSON(w, http.StatusOK, alerts)
}

func describeAlert(a Alert) (severity, message string) {
	switch a.Type {
	case "secret_expired":
		return "high", a.TargetName + " has expired"
	case "secret_expiring":
		return "medium", a.TargetName + " expires soon"
	case "rotation_overdue":
		return "medium", a.TargetName + " is overdue for rotation"
	default:
		if a.DueAt == nil {
			return "low", a.TargetName + " has never logged in"
		}
		return "low", a.TargetName + " has not logged in for over 90 days"
	}
}

// organizationFilter reads the optional organizationId (or organization_id) query parameter.
// It returns nil when absent and writes a 400 response for a malformed ID.
func organizationFilter(w http.ResponseWriter, r *http.Request) (*uuid.UUID, bool) {
	raw := r.URL.Query().Get("organizationId")
	if raw == "" {
		raw = r.URL.Query().Get("organization_id")
	}
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid organizationId format")
		return nil, false
	}
	return &id, true
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
		h.logger.WarnContext(r.Context(), "Health check: database ping failed", slog.Any("error", err))
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
		h.logger.WarnContext(r.Context(), "Health check: reading schema version failed", slog.Any("error", err))
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
