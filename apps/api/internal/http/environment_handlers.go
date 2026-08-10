package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/environments"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/http/middleware"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/policy"
	"go.uber.org/zap"
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
	policyService      policy.PolicyService
	db                 *pgxpool.Pool
	logger             *zap.Logger
}

// NewEnvironmentHandlers creates a new instance of EnvironmentHandlers
func NewEnvironmentHandlers(environmentService environments.EnvironmentService, policyService policy.PolicyService, db *pgxpool.Pool, logger *zap.Logger) *EnvironmentHandlers {
	return &EnvironmentHandlers{
		environmentService: environmentService,
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

	// Verify vault membership
	if !h.checkVaultMembership(r.Context(), claims.UserID, vaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User does not have access to this vault")
		return
	}

	// Get organization ID for RBAC check
	orgID, err := h.getVaultOrgID(r.Context(), vaultID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to verify permissions")
		return
	}

	// Check RBAC permission
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionEnvWrite, orgID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
		return
	}

	// Parse request body
	var req CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Create environment
	environment, err := h.environmentService.CreateEnvironment(r.Context(), vaultID, req.Name, req.Description)
	if err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

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
		h.handleEnvironmentError(w, err)
		return
	}

	// Verify vault membership via the environment's vault
	if !h.checkVaultMembership(r.Context(), claims.UserID, environment.VaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User does not have access to this vault")
		return
	}

	// Get organization ID for RBAC check
	orgID, err := h.getVaultOrgID(r.Context(), environment.VaultID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to verify permissions")
		return
	}

	// Check RBAC permission
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionEnvRead, orgID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
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

	// Verify vault membership
	if !h.checkVaultMembership(r.Context(), claims.UserID, vaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User does not have access to this vault")
		return
	}

	// Get organization ID for RBAC check
	orgID, err := h.getVaultOrgID(r.Context(), vaultID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to verify permissions")
		return
	}

	// Check RBAC permission
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionEnvRead, orgID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
		return
	}

	// List environments
	environmentList, err := h.environmentService.ListEnvironmentsByVault(r.Context(), vaultID)
	if err != nil {
		h.handleEnvironmentError(w, err)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Get environment to verify membership
	environment, err := h.environmentService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

	// Verify vault membership via the environment's vault
	if !h.checkVaultMembership(r.Context(), claims.UserID, environment.VaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User does not have access to this vault")
		return
	}

	// Get organization ID for RBAC check
	orgID, err := h.getVaultOrgID(r.Context(), environment.VaultID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to verify permissions")
		return
	}

	// Check RBAC permission
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionEnvWrite, orgID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
		return
	}

	// Update environment
	updatedEnvironment, err := h.environmentService.UpdateEnvironment(r.Context(), envID, req.Name, req.Description)
	if err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

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
		h.handleEnvironmentError(w, err)
		return
	}

	// Verify vault membership via the environment's vault
	if !h.checkVaultMembership(r.Context(), claims.UserID, environment.VaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User does not have access to this vault")
		return
	}

	// Get organization ID for RBAC check
	orgID, err := h.getVaultOrgID(r.Context(), environment.VaultID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to verify permissions")
		return
	}

	// Check RBAC permission
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionEnvDelete, orgID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
		return
	}

	// Delete environment
	if err := h.environmentService.DeleteEnvironment(r.Context(), envID); err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkVaultMembership verifies if a user has access to a vault via organization membership
func (h *EnvironmentHandlers) checkVaultMembership(ctx context.Context, userID, vaultID uuid.UUID) bool {
	query := `
		SELECT 1
		FROM vaults v
		JOIN user_organizations uo ON v.organization_id = uo.organization_id
		WHERE v.id = $1 AND uo.user_id = $2 AND v.deleted_at IS NULL
	`
	var exists int
	err := h.db.QueryRow(ctx, query, vaultID, userID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false
		}
		h.logger.Error("Failed to check vault membership", zap.Error(err))
		return false
	}
	return true
}

// getVaultOrgID retrieves the organization ID for a given vault
func (h *EnvironmentHandlers) getVaultOrgID(ctx context.Context, vaultID uuid.UUID) (uuid.UUID, error) {
	query := `SELECT organization_id FROM vaults WHERE id = $1 AND deleted_at IS NULL`
	var orgID uuid.UUID
	err := h.db.QueryRow(ctx, query, vaultID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errors.New("vault not found")
		}
		return uuid.Nil, err
	}
	return orgID, nil
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
func (h *EnvironmentHandlers) handleEnvironmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, environments.ErrNotFound):
		h.respondError(w, http.StatusNotFound, "not_found", "Environment not found")
	case errors.Is(err, environments.ErrDuplicate):
		h.respondError(w, http.StatusConflict, "duplicate", "Environment already exists")
	default:
		h.logger.Error("Unexpected environment error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondJSON writes a JSON response
func (h *EnvironmentHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// respondError writes an error response
func (h *EnvironmentHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
