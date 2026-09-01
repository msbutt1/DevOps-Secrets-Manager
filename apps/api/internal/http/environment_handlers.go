package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/environments"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/validate"
)

// Request DTOs
type CreateEnvironmentRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateEnvironmentRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// Response DTOs
type EnvironmentResponse struct {
	ID          uuid.UUID `json:"id"`
	VaultID     uuid.UUID `json:"vault_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SecretCount int       `json:"secret_count"`
}

// EnvironmentHandlers handles environment-related HTTP requests
type EnvironmentHandlers struct {
	environmentService environments.EnvironmentService
	auditService       audit.AuditService
	policyService      policy.PolicyService
	db                 *pgxpool.Pool
	logger             *slog.Logger
}

// NewEnvironmentHandlers creates a new instance of EnvironmentHandlers
func NewEnvironmentHandlers(environmentService environments.EnvironmentService, auditService audit.AuditService, policyService policy.PolicyService, db *pgxpool.Pool, logger *slog.Logger) *EnvironmentHandlers {
	return &EnvironmentHandlers{
		environmentService: environmentService,
		auditService:       auditService,
		policyService:      policyService,
		db:                 db,
		logger:             logger,
	}
}

// HandleCreateEnvironment handles environment creation requests
func (h *EnvironmentHandlers) HandleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse vault ID from URL parameter
	vaultIDStr := chi.URLParam(r, "vault_id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionEnvWrite, "Vault", h.logPolicyError); !ok {
		return
	}

	// Parse request body
	var req CreateEnvironmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if rejectInvalid(w, validate.Description(req.Description)) {
		return
	}

	// Create environment
	environment, err := h.environmentService.CreateEnvironment(r.Context(), vaultID, req.Name, req.Description)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	h.record(r, claims.UserID, audit.ActionEnvCreated, environment)

	h.respondJSON(w, http.StatusCreated, h.toEnvironmentResponse(environment))
}

// HandleGetEnvironment handles environment retrieval requests
func (h *EnvironmentHandlers) HandleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse environment ID from URL parameter
	envIDStr := chi.URLParam(r, "id")
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid environment ID format")
		return
	}

	// Get environment
	environment, err := h.environmentService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionEnvRead, "Environment", h.logPolicyError); !ok {
		return
	}

	h.respondJSON(w, http.StatusOK, h.toEnvironmentResponse(environment))
}

// HandleListEnvironments handles environment listing requests
func (h *EnvironmentHandlers) HandleListEnvironments(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse vault ID from URL parameter
	vaultIDStr := chi.URLParam(r, "vault_id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionEnvRead, "Vault", h.logPolicyError); !ok {
		return
	}

	// List environments
	environmentList, err := h.environmentService.ListEnvironmentsByVault(r.Context(), vaultID)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	// Convert to response DTOs
	responses := make([]EnvironmentResponse, 0, len(environmentList))
	for _, environment := range environmentList {
		responses = append(responses, h.toEnvironmentResponse(environment))
	}

	h.respondJSON(w, http.StatusOK, responses)
}

// HandleUpdateEnvironment handles environment update requests
func (h *EnvironmentHandlers) HandleUpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse environment ID from URL parameter
	envIDStr := chi.URLParam(r, "id")
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid environment ID format")
		return
	}

	// Parse request body
	var req UpdateEnvironmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if rejectInvalid(w, validate.Description(req.Description)) {
		return
	}

	// Get environment to verify membership
	environment, err := h.environmentService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionEnvWrite, "Environment", h.logPolicyError); !ok {
		return
	}

	// Update environment
	updatedEnvironment, err := h.environmentService.UpdateEnvironment(r.Context(), envID, req.Name, req.Description)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	h.record(r, claims.UserID, audit.ActionEnvUpdated, updatedEnvironment)

	h.respondJSON(w, http.StatusOK, h.toEnvironmentResponse(updatedEnvironment))
}

// HandleDeleteEnvironment handles environment deletion requests
func (h *EnvironmentHandlers) HandleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse environment ID from URL parameter
	envIDStr := chi.URLParam(r, "id")
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid environment ID format")
		return
	}

	// Get environment to verify membership
	environment, err := h.environmentService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionEnvDelete, "Environment", h.logPolicyError); !ok {
		return
	}

	// Delete environment
	if err := h.environmentService.DeleteEnvironment(r.Context(), envID); err != nil {
		h.handleEnvironmentError(w, r, err)
		return
	}

	h.record(r, claims.UserID, audit.ActionEnvDeleted, environment)

	w.WriteHeader(http.StatusNoContent)
}

// record writes an environment audit event; the audit service logs failures itself.
func (h *EnvironmentHandlers) record(r *http.Request, userID uuid.UUID, action string, env *environments.Environment) {
	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: userID, Action: action,
		TargetType: "environment", TargetID: &env.ID, TargetName: env.Name,
		VaultID: &env.VaultID, EnvironmentID: &env.ID,
	})
}

// logPolicyError logs a failed permission lookup
func (h *EnvironmentHandlers) logPolicyError(ctx context.Context, err error) {
	h.logger.ErrorContext(ctx, "Failed to check vault permissions", slog.Any("error", err))
}

// toEnvironmentResponse converts an Environment domain model to EnvironmentResponse DTO
func (h *EnvironmentHandlers) toEnvironmentResponse(environment *environments.Environment) EnvironmentResponse {
	return EnvironmentResponse{
		ID:          environment.ID,
		VaultID:     environment.VaultID,
		Name:        environment.Name,
		Description: environment.Description,
		CreatedAt:   environment.CreatedAt,
		UpdatedAt:   environment.UpdatedAt,
		SecretCount: environment.SecretCount,
	}
}

// handleEnvironmentError maps environment service errors to appropriate HTTP responses
func (h *EnvironmentHandlers) handleEnvironmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, environments.ErrNotFound):
		h.respondError(w, http.StatusNotFound, "not_found", "Environment not found")
	case errors.Is(err, environments.ErrDuplicate):
		h.respondError(w, http.StatusConflict, "duplicate", "Environment already exists")
	case errors.Is(err, environments.ErrInvalidName):
		h.respondError(w, http.StatusBadRequest, "invalid_name", err.Error())
	default:
		h.logger.ErrorContext(r.Context(), "Unexpected environment error", slog.Any("error", err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondJSON writes a JSON response
func (h *EnvironmentHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", slog.Any("error", err))
	}
}

// respondError writes an error response
func (h *EnvironmentHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
