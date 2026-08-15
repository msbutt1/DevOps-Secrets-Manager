package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/environments"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/secrets"
	"go.uber.org/zap"
)

// Request DTOs
type CreateSecretRequest struct {
	KeyName              string                 `json:"key_name"`
	Value                string                 `json:"value"`
	Description          *string                `json:"description"`
	RotationIntervalDays *int                   `json:"rotation_interval_days"`
	ExpiresAt            *time.Time             `json:"expires_at"`
	Metadata             map[string]interface{} `json:"metadata"`
}

type UpdateSecretRequest struct {
	Value                string                 `json:"value"`
	Description          *string                `json:"description"`
	RotationIntervalDays *int                   `json:"rotation_interval_days"`
	ExpiresAt            *time.Time             `json:"expires_at"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// Response DTOs
type SecretMetadataResponse struct {
	ID                   uuid.UUID              `json:"id"`
	EnvironmentID        uuid.UUID              `json:"environment_id"`
	KeyName              string                 `json:"key_name"`
	Description          *string                `json:"description,omitempty"`
	RotationIntervalDays *int                   `json:"rotation_interval_days,omitempty"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty"`
	LastRotatedAt        *time.Time             `json:"last_rotated_at,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy            uuid.UUID              `json:"created_by"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type SecretRevealResponse struct {
	ID      uuid.UUID `json:"id"`
	KeyName string    `json:"key_name"`
	Value   string    `json:"value"`
}

// SecretHandlers handles secret-related HTTP requests
type SecretHandlers struct {
	secretService secrets.SecretService
	envService    environments.EnvironmentService
	policyService policy.PolicyService
	db            *pgxpool.Pool
	logger        *zap.Logger
}

// NewSecretHandlers creates a new instance of SecretHandlers
func NewSecretHandlers(secretService secrets.SecretService, envService environments.EnvironmentService, policyService policy.PolicyService, db *pgxpool.Pool, logger *zap.Logger) *SecretHandlers {
	return &SecretHandlers{
		secretService: secretService,
		envService:    envService,
		policyService: policyService,
		db:            db,
		logger:        logger,
	}
}

// HandleListSecrets handles listing secrets for an environment
func (h *SecretHandlers) HandleListSecrets(w http.ResponseWriter, r *http.Request) {
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

	// Resolve the environment, then check the caller's role on the vault that owns it
	environment, err := h.envService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionSecretRead, "Environment", h.logPolicyError); !ok {
		return
	}

	// List secrets by environment
	secretList, err := h.secretService.ListSecretsByEnvironment(r.Context(), envID)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	// Convert to response DTOs
	responses := make([]SecretMetadataResponse, 0, len(secretList))
	for _, secret := range secretList {
		responses = append(responses, h.toSecretMetadataResponse(secret))
	}

	h.respondJSON(w, http.StatusOK, responses)
}

// HandleCreateSecret handles secret creation
func (h *SecretHandlers) HandleCreateSecret(w http.ResponseWriter, r *http.Request) {
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

	// Resolve the environment, then check the caller's role on the vault that owns it
	environment, err := h.envService.GetEnvironment(r.Context(), envID)
	if err != nil {
		h.handleEnvironmentError(w, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionSecretWrite, "Environment", h.logPolicyError); !ok {
		return
	}

	// Parse request body
	var req CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Create secret
	secret, err := h.secretService.CreateSecret(
		r.Context(),
		envID,
		req.KeyName,
		req.Value,
		req.Description,
		req.RotationIntervalDays,
		req.ExpiresAt,
		req.Metadata,
		claims.UserID,
	)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, h.toSecretMetadataResponse(secret))
}

// HandleUpdateSecret handles secret updates
func (h *SecretHandlers) HandleUpdateSecret(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse secret ID from URL parameter
	secretIDStr := chi.URLParam(r, "id")
	secretID, err := uuid.Parse(secretIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid secret ID format")
		return
	}

	// Get secret metadata
	secret, err := h.secretService.GetSecretMetadata(r.Context(), secretID)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	// Resolve the environment, then check the caller's role on the vault that owns it
	environment, err := h.envService.GetEnvironment(r.Context(), secret.EnvironmentID)
	if err != nil {
		if errors.Is(err, environments.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Secret not found")
			return
		}
		h.handleEnvironmentError(w, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionSecretWrite, "Secret", h.logPolicyError); !ok {
		return
	}

	// Parse request body
	var req UpdateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Update secret
	updatedSecret, err := h.secretService.UpdateSecret(
		r.Context(),
		secretID,
		req.Value,
		req.Description,
		req.RotationIntervalDays,
		req.ExpiresAt,
		req.Metadata,
		claims.UserID,
	)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, h.toSecretMetadataResponse(updatedSecret))
}

// HandleDeleteSecret handles secret deletion
func (h *SecretHandlers) HandleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse secret ID from URL parameter
	secretIDStr := chi.URLParam(r, "id")
	secretID, err := uuid.Parse(secretIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid secret ID format")
		return
	}

	// Get secret metadata
	secret, err := h.secretService.GetSecretMetadata(r.Context(), secretID)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	// Resolve the environment, then check the caller's role on the vault that owns it
	environment, err := h.envService.GetEnvironment(r.Context(), secret.EnvironmentID)
	if err != nil {
		if errors.Is(err, environments.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Secret not found")
			return
		}
		h.handleEnvironmentError(w, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionSecretDelete, "Secret", h.logPolicyError); !ok {
		return
	}

	// Delete secret
	if err := h.secretService.DeleteSecret(r.Context(), secretID, claims.UserID); err != nil {
		h.handleSecretError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleRevealSecret handles secret value revelation
func (h *SecretHandlers) HandleRevealSecret(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse secret ID from URL parameter
	secretIDStr := chi.URLParam(r, "id")
	secretID, err := uuid.Parse(secretIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid secret ID format")
		return
	}

	// Get secret metadata
	secret, err := h.secretService.GetSecretMetadata(r.Context(), secretID)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	// Resolve the environment, then check the caller's role on the vault that owns it
	environment, err := h.envService.GetEnvironment(r.Context(), secret.EnvironmentID)
	if err != nil {
		if errors.Is(err, environments.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Secret not found")
			return
		}
		h.handleEnvironmentError(w, err)
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, environment.VaultID, policy.ActionSecretReveal, "Secret", h.logPolicyError); !ok {
		return
	}

	// Reveal secret (this will audit log the reveal action)
	plaintextValue, err := h.secretService.RevealSecret(r.Context(), secretID, claims.UserID)
	if err != nil {
		h.handleSecretError(w, err)
		return
	}

	response := SecretRevealResponse{
		ID:      secret.ID,
		KeyName: secret.KeyName,
		Value:   plaintextValue,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// logPolicyError logs a failed permission lookup
func (h *SecretHandlers) logPolicyError(err error) {
	h.logger.Error("Failed to check vault permissions", zap.Error(err))
}

// toSecretMetadataResponse converts a Secret domain model to SecretMetadataResponse DTO
func (h *SecretHandlers) toSecretMetadataResponse(secret *secrets.Secret) SecretMetadataResponse {
	return SecretMetadataResponse{
		ID:                   secret.ID,
		EnvironmentID:        secret.EnvironmentID,
		KeyName:              secret.KeyName,
		Description:          secret.Description,
		RotationIntervalDays: secret.RotationIntervalDays,
		ExpiresAt:            secret.ExpiresAt,
		LastRotatedAt:        secret.LastRotatedAt,
		Metadata:             secret.Metadata,
		CreatedBy:            secret.CreatedBy,
		CreatedAt:            secret.CreatedAt,
		UpdatedAt:            secret.UpdatedAt,
	}
}

// handleSecretError maps secret service errors to appropriate HTTP responses
func (h *SecretHandlers) handleSecretError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		h.respondError(w, http.StatusNotFound, "not_found", "Secret not found")
	case errors.Is(err, secrets.ErrDuplicate):
		h.respondError(w, http.StatusConflict, "duplicate", "Secret with this key already exists in the environment")
	default:
		h.logger.Error("Unexpected secret error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// handleEnvironmentError maps environment service errors to appropriate HTTP responses
func (h *SecretHandlers) handleEnvironmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, environments.ErrNotFound):
		h.respondError(w, http.StatusNotFound, "not_found", "Environment not found")
	default:
		h.logger.Error("Unexpected environment error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondJSON writes a JSON response
func (h *SecretHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// respondError writes an error response
func (h *SecretHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
