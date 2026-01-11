package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Context keys for extracting IP and User-Agent
type contextKey string

const (
	ContextKeyIPAddress contextKey = "ip_address"
	ContextKeyUserAgent contextKey = "user_agent"
)

// AuditService defines the interface for audit operations
type AuditService interface {
	Log(ctx context.Context, userID uuid.UUID, action string, resourceType string, resourceID uuid.UUID, orgID *uuid.UUID, vaultID *uuid.UUID, metadata map[string]interface{}) error
	Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, error)
}

type auditService struct {
	repo   Repository
	logger *slog.Logger
}

// NewAuditService creates a new audit service
func NewAuditService(repo Repository, logger *slog.Logger) AuditService {
	return &auditService{
		repo:   repo,
		logger: logger,
	}
}

// Log creates a new audit log entry
func (s *auditService) Log(ctx context.Context, userID uuid.UUID, action string, resourceType string, resourceID uuid.UUID, orgID *uuid.UUID, vaultID *uuid.UUID, metadata map[string]interface{}) error {
	// Extract IP address and User-Agent from context if available
	var ipAddress *string
	var userAgent *string

	if ip, ok := ctx.Value(ContextKeyIPAddress).(string); ok && ip != "" {
		ipAddress = &ip
	}

	if ua, ok := ctx.Value(ContextKeyUserAgent).(string); ok && ua != "" {
		userAgent = &ua
	}

	// Create audit entry
	entry := &AuditEntry{
		ID:             uuid.New(),
		Timestamp:      time.Now(),
		UserID:         userID,
		OrganizationID: orgID,
		VaultID:        vaultID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     &resourceID,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Metadata:       metadata,
	}

	// Append to repository
	if err := s.repo.Append(ctx, entry); err != nil {
		s.logger.Error("failed to append audit log",
			"error", err,
			"user_id", userID,
			"action", action,
			"resource_type", resourceType,
		)
		return err
	}

	s.logger.Debug("audit log created",
		"audit_id", entry.ID,
		"user_id", userID,
		"action", action,
		"resource_type", resourceType,
	)

	return nil
}

// Query retrieves audit logs based on filters
func (s *auditService) Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, error) {
	entries, err := s.repo.Query(ctx, filters)
	if err != nil {
		s.logger.Error("failed to query audit logs",
			"error", err,
		)
		return nil, err
	}

	s.logger.Debug("audit logs queried",
		"count", len(entries),
	)

	return entries, nil
}
