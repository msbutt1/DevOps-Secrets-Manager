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
			id, timestamp, user_id, organization_id, vault_id, action,
			resource_type, resource_id, ip_address, user_agent, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		entry.IPAddress,
		entry.UserAgent,
		metadataJSON,
	).Scan(&entry.ID, &entry.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to append audit log: %w", err)
	}

	return nil
}

// Query retrieves audit logs based on the provided filters
func (r *postgresRepository) Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, error) {
	query := `
		SELECT id, timestamp, user_id, organization_id, vault_id, action,
		       resource_type, resource_id, ip_address, user_agent, metadata
		FROM audit_logs
	`

	var conditions []string
	var args []interface{}
	argIndex := 1

	// Build dynamic WHERE clause
	if filters.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, *filters.UserID)
		argIndex++
	}

	if filters.OrgID != nil {
		conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIndex))
		args = append(args, *filters.OrgID)
		argIndex++
	}

	if filters.VaultID != nil {
		conditions = append(conditions, fmt.Sprintf("vault_id = $%d", argIndex))
		args = append(args, *filters.VaultID)
		argIndex++
	}

	if filters.Action != nil {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIndex))
		args = append(args, *filters.Action)
		argIndex++
	}

	if filters.ResourceType != nil {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argIndex))
		args = append(args, *filters.ResourceType)
		argIndex++
	}

	if filters.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIndex))
		args = append(args, *filters.StartTime)
		argIndex++
	}

	if filters.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIndex))
		args = append(args, *filters.EndTime)
		argIndex++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ordering
	query += " ORDER BY timestamp DESC"

	// Add pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var metadataJSON []byte

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.UserID,
			&entry.OrganizationID,
			&entry.VaultID,
			&entry.Action,
			&entry.ResourceType,
			&entry.ResourceID,
			&entry.IPAddress,
			&entry.UserAgent,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit entry: %w", err)
		}

		// Unmarshal metadata if present
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return entries, nil
}
