package http

import (
	"encoding/json"
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

const (
	defaultAuditPageSize = 50
	maxAuditPageSize     = 200
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

// AuditEventResponse is one audit log entry with the names needed to display it
type AuditEventResponse struct {
	ID              uuid.UUID              `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	Action          string                 `json:"action"`
	UserID          uuid.UUID              `json:"user_id"`
	UserEmail       string                 `json:"user_email"`
	OrganizationID  *uuid.UUID             `json:"organization_id"`
	VaultID         *uuid.UUID             `json:"vault_id"`
	VaultName       *string                `json:"vault_name"`
	EnvironmentID   *uuid.UUID             `json:"environment_id"`
	EnvironmentName *string                `json:"environment_name"`
	TargetType      string                 `json:"target_type"`
	TargetID        *uuid.UUID             `json:"target_id"`
	TargetName      *string                `json:"target_name"`
	IPAddress       *string                `json:"ip_address"`
	UserAgent       *string                `json:"user_agent"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PaginatedAuditResponse is a page of audit events
type PaginatedAuditResponse struct {
	Data    []AuditEventResponse `json:"data"`
	Total   int                  `json:"total"`
	Page    int                  `json:"page"`
	Limit   int                  `json:"limit"`
	HasMore bool                 `json:"has_more"`
}

// HandleQueryAuditLogs handles GET /audit.
//
// Callers see their own events, every event in organizations they own or administer, and
// events in vaults where they are an owner or admin. Query parameters (camelCase; snake_case
// aliases are accepted): page, limit, organizationId, vaultId, environmentId, userId,
// userEmail, action, startDate, endDate. Dates are RFC 3339 timestamps or YYYY-MM-DD days;
// a day as endDate includes that whole day.
func (h *AuditHandlers) HandleQueryAuditLogs(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	q := r.URL.Query()
	param := func(camel, snake string) string {
		if v := q.Get(camel); v != "" {
			return v
		}
		return q.Get(snake)
	}

	filters := audit.QueryFilters{ViewerID: &claims.UserID}

	uuidParams := []struct {
		camel, snake string
		target       **uuid.UUID
	}{
		{"organizationId", "org_id", &filters.OrgID},
		{"vaultId", "vault_id", &filters.VaultID},
		{"environmentId", "environment_id", &filters.EnvironmentID},
		{"userId", "user_id", &filters.UserID},
	}
	for _, p := range uuidParams {
		if raw := param(p.camel, p.snake); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid "+p.camel+" format")
				return
			}
			*p.target = &id
		}
	}

	if email := param("userEmail", "user_email"); email != "" {
		filters.UserEmail = &email
	}
	if action := q.Get("action"); action != "" {
		filters.Action = &action
	}
	if resourceType := param("targetType", "resource_type"); resourceType != "" {
		filters.ResourceType = &resourceType
	}

	if raw := param("startDate", "start_time"); raw != "" {
		start, _, err := parseAuditTime(raw)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid startDate (use RFC 3339 or YYYY-MM-DD)")
			return
		}
		filters.StartTime = &start
	}
	if raw := param("endDate", "end_time"); raw != "" {
		end, dayOnly, err := parseAuditTime(raw)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid endDate (use RFC 3339 or YYYY-MM-DD)")
			return
		}
		if dayOnly {
			end = end.AddDate(0, 0, 1)
		}
		filters.EndTime = &end
	}

	page, err := positiveInt(q.Get("page"), 1)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid page value")
		return
	}
	limit, err := positiveInt(q.Get("limit"), defaultAuditPageSize)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid limit value")
		return
	}
	if limit > maxAuditPageSize {
		limit = maxAuditPageSize
	}
	filters.Limit = limit
	filters.Offset = (page - 1) * limit

	entries, total, err := h.auditService.Query(r.Context(), filters)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit logs")
		return
	}

	data := make([]AuditEventResponse, 0, len(entries))
	for _, entry := range entries {
		data = append(data, toAuditEventResponse(entry))
	}

	h.respondJSON(w, http.StatusOK, PaginatedAuditResponse{
		Data:    data,
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: filters.Offset+len(data) < total,
	})
}

// parseAuditTime accepts RFC 3339 timestamps or YYYY-MM-DD days (UTC). dayOnly reports the latter.
func parseAuditTime(raw string) (t time.Time, dayOnly bool, err error) {
	if t, err = time.Parse(time.RFC3339, raw); err == nil {
		return t, false, nil
	}
	t, err = time.Parse(time.DateOnly, raw)
	return t, err == nil, err
}

func positiveInt(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, strconv.ErrSyntax
	}
	return n, nil
}

func toAuditEventResponse(entry *audit.AuditEntry) AuditEventResponse {
	metadata := entry.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return AuditEventResponse{
		ID:              entry.ID,
		Timestamp:       entry.Timestamp,
		Action:          entry.Action,
		UserID:          entry.UserID,
		UserEmail:       entry.UserEmail,
		OrganizationID:  entry.OrganizationID,
		VaultID:         entry.VaultID,
		VaultName:       entry.VaultName,
		EnvironmentID:   entry.EnvironmentID,
		EnvironmentName: entry.EnvironmentName,
		TargetType:      entry.ResourceType,
		TargetID:        entry.ResourceID,
		TargetName:      entry.TargetName,
		IPAddress:       entry.IPAddress,
		UserAgent:       entry.UserAgent,
		Metadata:        metadata,
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
