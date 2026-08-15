package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL-backed audit repository
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{
		pool: pool,
	}
}

// Append inserts a new audit log entry into the database (append-only)
func (r *postgresRepository) Append(ctx context.Context, entry *AuditEntry) error {
	query := `
		INSERT INTO audit_logs (
			id, timestamp, user_id, organization_id, vault_id, environment_id, action,
			resource_type, resource_id, target_name, ip_address, user_agent, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, timestamp
	`

	// Convert metadata to JSONB
	var metadataJSON []byte
	var err error
	if entry.Metadata != nil {
		metadataJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	err = r.pool.QueryRow(
		ctx,
		query,
		entry.ID,
		entry.Timestamp,
		entry.UserID,
		entry.OrganizationID,
		entry.VaultID,
		entry.EnvironmentID,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		entry.TargetName,
		entry.IPAddress,
		entry.UserAgent,
		metadataJSON,
	).Scan(&entry.ID, &entry.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to append audit log: %w", err)
	}

	return nil
}

// Query returns one page of audit logs matching the filters, newest first, and the total
// number of matching rows.
func (r *postgresRepository) Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, int, error) {
	var conditions []string
	var args []interface{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filters.ViewerID != nil {
		viewer := arg(*filters.ViewerID)
		conditions = append(conditions, `(
			a.user_id = `+viewer+`
			OR a.organization_id IN (
				SELECT organization_id FROM user_organizations
				WHERE user_id = `+viewer+` AND role IN ('owner', 'admin'))
			OR a.vault_id IN (
				SELECT vm.vault_id FROM vault_members vm
				JOIN vaults mv ON mv.id = vm.vault_id
				JOIN user_organizations muo ON muo.organization_id = mv.organization_id AND muo.user_id = vm.user_id
				WHERE vm.user_id = `+viewer+` AND vm.role IN ('owner', 'admin'))
		)`)
	}
	if filters.UserID != nil {
		conditions = append(conditions, "a.user_id = "+arg(*filters.UserID))
	}
	if filters.UserEmail != nil {
		conditions = append(conditions, "u.email ILIKE "+arg("%"+escapeLike(*filters.UserEmail)+"%"))
	}
	if filters.OrgID != nil {
		conditions = append(conditions, "a.organization_id = "+arg(*filters.OrgID))
	}
	if filters.VaultID != nil {
		conditions = append(conditions, "a.vault_id = "+arg(*filters.VaultID))
	}
	if filters.EnvironmentID != nil {
		conditions = append(conditions, "a.environment_id = "+arg(*filters.EnvironmentID))
	}
	if filters.Action != nil {
		conditions = append(conditions, "a.action = "+arg(*filters.Action))
	}
	if filters.ResourceType != nil {
		conditions = append(conditions, "a.resource_type = "+arg(*filters.ResourceType))
	}
	if filters.StartTime != nil {
		conditions = append(conditions, "a.timestamp >= "+arg(*filters.StartTime))
	}
	if filters.EndTime != nil {
		conditions = append(conditions, "a.timestamp < "+arg(*filters.EndTime))
	}

	from := `
		FROM audit_logs a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN vaults v ON v.id = a.vault_id
		LEFT JOIN environments e ON e.id = a.environment_id
	`
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*)"+from+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	query := `
		SELECT a.id, a.timestamp, a.user_id, a.organization_id, a.vault_id, a.environment_id, a.action,
		       a.resource_type, a.resource_id, a.target_name, host(a.ip_address), a.user_agent, a.metadata,
		       u.email, v.name, e.name` + from + where + " ORDER BY a.timestamp DESC, a.id"
	if filters.Limit > 0 {
		query += " LIMIT " + arg(filters.Limit)
	}
	if filters.Offset > 0 {
		query += " OFFSET " + arg(filters.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]*AuditEntry, 0)
	for rows.Next() {
		var entry AuditEntry
		var metadataJSON []byte

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.UserID,
			&entry.OrganizationID,
			&entry.VaultID,
			&entry.EnvironmentID,
			&entry.Action,
			&entry.ResourceType,
			&entry.ResourceID,
			&entry.TargetName,
			&entry.IPAddress,
			&entry.UserAgent,
			&metadataJSON,
			&entry.UserEmail,
			&entry.VaultName,
			&entry.EnvironmentName,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit entry: %w", err)
		}

		// Unmarshal metadata if present
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &entry.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return entries, total, nil
}

// escapeLike escapes LIKE wildcards in user input.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
