package audit

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Context keys for extracting IP and User-Agent
type contextKey string

const (
	ContextKeyIPAddress contextKey = "ip_address"
	ContextKeyUserAgent contextKey = "user_agent"
)

// RequestContext is HTTP middleware that stores the client IP and User-Agent in the request
// context so audit events record where an action came from. The IP is taken from the
// connection; set up a trusted proxy's real-IP handling in front of this if needed.
func RequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ctx = context.WithValue(ctx, ContextKeyIPAddress, host)
		} else if r.RemoteAddr != "" {
			ctx = context.WithValue(ctx, ContextKeyIPAddress, r.RemoteAddr)
		}
		if ua := r.UserAgent(); ua != "" {
			if len(ua) > 512 {
				ua = ua[:512]
			}
			ctx = context.WithValue(ctx, ContextKeyUserAgent, ua)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuditService defines the interface for audit operations
type AuditService interface {
	Record(ctx context.Context, event Event) error
	Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, int, error)
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

// Record appends an event to the audit log.
func (s *auditService) Record(ctx context.Context, event Event) error {
	// Extract IP address and User-Agent from context if available
	var ipAddress *string
	var userAgent *string

	if ip, ok := ctx.Value(ContextKeyIPAddress).(string); ok && net.ParseIP(ip) != nil {
		ipAddress = &ip
	}

	if ua, ok := ctx.Value(ContextKeyUserAgent).(string); ok && ua != "" {
		userAgent = &ua
	}

	var targetName *string
	if name := strings.TrimSpace(event.TargetName); name != "" {
		targetName = &name
	}

	entry := &AuditEntry{
		ID:             uuid.New(),
		Timestamp:      time.Now(),
		UserID:         event.UserID,
		OrganizationID: event.OrganizationID,
		VaultID:        event.VaultID,
		EnvironmentID:  event.EnvironmentID,
		Action:         event.Action,
		ResourceType:   event.TargetType,
		ResourceID:     event.TargetID,
		TargetName:     targetName,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Metadata:       event.Metadata,
	}

	// Append to repository
	if err := s.repo.Append(ctx, entry); err != nil {
		s.logger.Error("failed to append audit log",
			"error", err,
			"user_id", event.UserID,
			"action", event.Action,
			"resource_type", event.TargetType,
		)
		return err
	}

	return nil
}

// Query retrieves one page of audit logs and the total match count
func (s *auditService) Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, int, error) {
	entries, total, err := s.repo.Query(ctx, filters)
	if err != nil {
		s.logger.Error("failed to query audit logs", "error", err)
		return nil, 0, err
	}
	return entries, total, nil
}
