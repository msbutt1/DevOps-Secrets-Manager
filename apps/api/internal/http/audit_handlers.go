package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
)

// AuditHandlers handles audit-related HTTP requests
type AuditHandlers struct {
	auditService  audit.AuditService
	policyService policy.PolicyService
	db            *pgxpool.Pool
	logger        *slog.Logger
}

// NewAuditHandlers creates a new instance of AuditHandlers
func NewAuditHandlers(auditService audit.AuditService, policyService policy.PolicyService, db *pgxpool.Pool, logger *slog.Logger) *AuditHandlers {
	return &AuditHandlers{
		auditService:  auditService,
		policyService: policyService,
		db:            db,
		logger:        logger,
	}
}

// AuditEntryResponse represents the JSON response for an audit log entry
type AuditEntryResponse struct {
	ID             uuid.UUID              `json:"id"`
	Timestamp      time.Time              `json:"timestamp"`
	UserID         uuid.UUID              `json:"user_id"`
	OrganizationID *uuid.UUID             `json:"organization_id"`
	VaultID        *uuid.UUID             `json:"vault_id"`
	Action         string                 `json:"action"`
	ResourceType   string                 `json:"resource_type"`
	ResourceID     *uuid.UUID             `json:"resource_id"`
	IPAddress      *string                `json:"ip_address"`
	UserAgent      *string                `json:"user_agent"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// HandleQueryAuditLogs handles GET /audit requests
func (h *AuditHandlers) HandleQueryAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Get org_id - if not provided, use user's first organization
	var orgID uuid.UUID
	orgIDStr := query.Get("org_id")
	if orgIDStr == "" {
		// Auto-detect from user's organizations
		orgs, err := h.getUserOrganizations(r.Context(), claims.UserID)
		if err != nil || len(orgs) == 0 {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "No organization found for user")
			return
		}
		orgID = orgs[0]
	} else {
		var err error
		orgID, err = uuid.Parse(orgIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid org_id format")
			return
		}
	}

	// Check RBAC permission
	allowed, err := h.policyService.CanOnOrg(r.Context(), claims.UserID, orgID, policy.ActionOrgAuditRead)
	if errors.Is(err, policy.ErrNoRole) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User is not a member of this organization")
		return
	}
	if err != nil {
		h.logger.Error("failed to check audit read permission", "error", err)
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to check permissions")
		return
	}
	if !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions to read audit logs")
		return
	}

	// Build query filters
	filters := audit.QueryFilters{
		OrgID: &orgID,
	}

	// Parse optional filters
	if userIDStr := query.Get("user_id"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid user_id format")
			return
		}
		filters.UserID = &userID
	}

	if vaultIDStr := query.Get("vault_id"); vaultIDStr != "" {
		vaultID, err := uuid.Parse(vaultIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault_id format")
			return
		}
		filters.VaultID = &vaultID
	}

	if action := query.Get("action"); action != "" {
		filters.Action = &action
	}

	if resourceType := query.Get("resource_type"); resourceType != "" {
		filters.ResourceType = &resourceType
	}

	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid start_time format (use RFC3339)")
			return
		}
		filters.StartTime = &startTime
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid end_time format (use RFC3339)")
			return
		}
		filters.EndTime = &endTime
	}

	// Parse limit with default and max
	limit := 100 // default
	if limitStr := query.Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 0 {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid limit value")
			return
		}
		limit = parsedLimit
	}
	if limit > 1000 {
		limit = 1000 // max limit
	}
	filters.Limit = limit

	// Parse offset
	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid offset value")
			return
		}
		offset = parsedOffset
	}
	filters.Offset = offset

	// Query audit logs
	entries, err := h.auditService.Query(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to query audit logs", "error", err)
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit logs")
		return
	}

	// Convert to response DTOs
	responses := make([]AuditEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, h.toAuditEntryResponse(entry))
	}

	h.respondJSON(w, http.StatusOK, responses)
}

// toAuditEntryResponse converts an AuditEntry to AuditEntryResponse
func (h *AuditHandlers) toAuditEntryResponse(entry *audit.AuditEntry) AuditEntryResponse {
	return AuditEntryResponse{
		ID:             entry.ID,
		Timestamp:      entry.Timestamp,
		UserID:         entry.UserID,
		OrganizationID: entry.OrganizationID,
		VaultID:        entry.VaultID,
		Action:         entry.Action,
		ResourceType:   entry.ResourceType,
		ResourceID:     entry.ResourceID,
		IPAddress:      entry.IPAddress,
		UserAgent:      entry.UserAgent,
		Metadata:       entry.Metadata,
	}
}

// respondJSON writes a JSON response
func (h *AuditHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

// respondError writes an error response
func (h *AuditHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}

// getUserOrganizations returns the organization IDs for a user
func (h *AuditHandlers) getUserOrganizations(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT organization_id FROM user_organizations WHERE user_id = $1`
	rows, err := h.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgIDs []uuid.UUID
	for rows.Next() {
		var orgID uuid.UUID
		if err := rows.Scan(&orgID); err != nil {
			return nil, err
		}
		orgIDs = append(orgIDs, orgID)
	}
	return orgIDs, rows.Err()
}
