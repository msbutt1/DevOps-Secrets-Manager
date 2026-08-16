package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/vaults"
	"go.uber.org/zap"
)

// Request DTOs
type CreateVaultRequest struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description"`
}

type UpdateVaultRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// Response DTOs
type VaultResponse struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	Name             string     `json:"name"`
	Description      *string    `json:"description"`
	UserRole         string     `json:"user_role"`
	SecretCount      int        `json:"secret_count"`
	EnvCount         int        `json:"env_count"`
	CreatedByID      *uuid.UUID `json:"created_by_id"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// VaultHandlers handles vault-related HTTP requests
type VaultHandlers struct {
	vaultService  vaults.VaultService
	auditService  audit.AuditService
	policyService policy.PolicyService
	db            *pgxpool.Pool
	logger        *zap.Logger
}

// NewVaultHandlers creates a new instance of VaultHandlers
func NewVaultHandlers(vaultService vaults.VaultService, auditService audit.AuditService, policyService policy.PolicyService, db *pgxpool.Pool, logger *zap.Logger) *VaultHandlers {
	return &VaultHandlers{
		vaultService:  vaultService,
		auditService:  auditService,
		policyService: policyService,
		db:            db,
		logger:        logger,
	}
}

// HandleCreateVault handles vault creation requests
func (h *VaultHandlers) HandleCreateVault(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse request body
	var req CreateVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// If no organization_id provided, use user's first organization
	orgID := req.OrganizationID
	if orgID == uuid.Nil {
		orgIDs, err := h.getUserOrganizations(r.Context(), claims.UserID)
		if err != nil || len(orgIDs) == 0 {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "No organization found for user")
			return
		}
		orgID = orgIDs[0]
	}

	// Organization owners, admins and developers may create vaults; the creator becomes the vault owner
	allowed, err := h.policyService.CanOnOrg(r.Context(), claims.UserID, orgID, policy.ActionOrgVaultCreate)
	if err != nil {
		if errors.Is(err, policy.ErrNoRole) {
			h.respondError(w, http.StatusForbidden, "forbidden", "User is not a member of this organization")
			return
		}
		h.logPolicyError(err)
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to check permissions")
		return
	}
	if !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Your organization role does not allow creating vaults")
		return
	}

	// Create vault
	vault, err := h.vaultService.CreateVault(r.Context(), orgID, req.Name, req.Description, claims.UserID)
	if err != nil {
		h.handleVaultError(w, err)
		return
	}

	// Add creator as owner in vault_members
	_, err = h.db.Exec(r.Context(),
		`INSERT INTO vault_members (vault_id, user_id, role, added_by, created_at) VALUES ($1, $2, 'owner', $2, NOW())`,
		vault.ID, claims.UserID)
	if err != nil {
		h.logger.Error("Failed to add creator to vault_members", zap.Error(err))
		// Continue anyway - vault was created successfully
	}

	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: claims.UserID, Action: audit.ActionVaultCreated,
		TargetType: "vault", TargetID: &vault.ID, TargetName: vault.Name,
		OrganizationID: &vault.OrganizationID, VaultID: &vault.ID,
	})

	// Return response without encrypted_dek
	h.respondJSON(w, http.StatusCreated, h.toVaultResponse(r.Context(), vault, claims.UserID))
}

// HandleListVaults handles vault listing requests - returns only vaults user is a member of
func (h *VaultHandlers) HandleListVaults(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Vaults the user is a member of, plus every vault in organizations they own or administer.
	// Vault membership only counts while the user is still in the vault's organization.
	query := `
		SELECT v.id FROM vaults v
		JOIN user_organizations uo ON uo.organization_id = v.organization_id AND uo.user_id = $1
		WHERE v.deleted_at IS NULL
		  AND (uo.role IN ('owner', 'admin')
		       OR EXISTS (SELECT 1 FROM vault_members vm WHERE vm.vault_id = v.id AND vm.user_id = $1))
		ORDER BY v.created_at DESC
	`
	rows, err := h.db.Query(r.Context(), query, claims.UserID)
	if err != nil {
		h.logger.Error("Failed to query user vaults", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to list vaults")
		return
	}
	defer rows.Close()

	responses := make([]VaultResponse, 0)
	for rows.Next() {
		var vaultID uuid.UUID
		if err := rows.Scan(&vaultID); err != nil {
			h.logger.Error("Failed to scan vault ID", zap.Error(err))
			continue
		}

		vault, err := h.vaultService.GetVault(r.Context(), vaultID)
		if err != nil {
			h.logger.Error("Failed to get vault", zap.Error(err))
			continue
		}

		responses = append(responses, h.toVaultResponse(r.Context(), vault, claims.UserID))
	}

	h.respondJSON(w, http.StatusOK, responses)
}

// HandleGetVault handles vault retrieval requests
func (h *VaultHandlers) HandleGetVault(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse vault ID from URL parameter
	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionVaultRead, "Vault", h.logPolicyError); !ok {
		return
	}

	// Get vault
	vault, err := h.vaultService.GetVault(r.Context(), vaultID)
	if err != nil {
		h.handleVaultError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, h.toVaultResponse(r.Context(), vault, claims.UserID))
}

// HandleUpdateVault handles vault update requests
func (h *VaultHandlers) HandleUpdateVault(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse vault ID from URL parameter
	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	// Parse request body
	var req UpdateVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionVaultWrite, "Vault", h.logPolicyError); !ok {
		return
	}

	// Update vault
	updatedVault, err := h.vaultService.UpdateVault(r.Context(), vaultID, req.Name, req.Description)
	if err != nil {
		h.handleVaultError(w, err)
		return
	}

	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: claims.UserID, Action: audit.ActionVaultUpdated,
		TargetType: "vault", TargetID: &updatedVault.ID, TargetName: updatedVault.Name,
		OrganizationID: &updatedVault.OrganizationID, VaultID: &updatedVault.ID,
	})

	h.respondJSON(w, http.StatusOK, h.toVaultResponse(r.Context(), updatedVault, claims.UserID))
}

// HandleDeleteVault handles vault deletion requests
func (h *VaultHandlers) HandleDeleteVault(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse vault ID from URL parameter
	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionVaultDelete, "Vault", h.logPolicyError); !ok {
		return
	}

	vault, err := h.vaultService.GetVault(r.Context(), vaultID)
	if err != nil {
		h.handleVaultError(w, err)
		return
	}

	// Delete vault
	if err := h.vaultService.DeleteVault(r.Context(), vaultID); err != nil {
		h.handleVaultError(w, err)
		return
	}

	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: claims.UserID, Action: audit.ActionVaultDeleted,
		TargetType: "vault", TargetID: &vault.ID, TargetName: vault.Name,
		OrganizationID: &vault.OrganizationID, VaultID: &vault.ID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// getUserOrganizations returns all organization IDs the user belongs to
func (h *VaultHandlers) getUserOrganizations(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
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

// toVaultResponse converts a Vault domain model to VaultResponse DTO
func (h *VaultHandlers) toVaultResponse(ctx context.Context, vault *vaults.Vault, userID uuid.UUID) VaultResponse {
	// Get organization name
	orgName := h.getOrgName(ctx, vault.OrganizationID)

	// Get user's role for this vault from vault_members
	userRole := h.getVaultUserRole(ctx, vault.ID, userID)

	// Get counts
	envCount, secretCount := h.getVaultCounts(ctx, vault.ID)

	return VaultResponse{
		ID:               vault.ID,
		OrganizationID:   vault.OrganizationID,
		OrganizationName: orgName,
		Name:             vault.Name,
		Description:      vault.Description,
		UserRole:         userRole,
		SecretCount:      secretCount,
		EnvCount:         envCount,
		CreatedByID:      vault.CreatedBy,
		CreatedBy:        h.getCreatorName(ctx, vault.CreatedBy),
		CreatedAt:        vault.CreatedAt,
		UpdatedAt:        vault.UpdatedAt,
	}
}

// getOrgName fetches organization name
func (h *VaultHandlers) getOrgName(ctx context.Context, orgID uuid.UUID) string {
	var orgName string
	h.db.QueryRow(ctx, "SELECT name FROM organizations WHERE id = $1", orgID).Scan(&orgName)
	if orgName == "" {
		return "Unknown"
	}
	return orgName
}

// getCreatorName returns the vault creator's display name, or "" when unknown
func (h *VaultHandlers) getCreatorName(ctx context.Context, userID *uuid.UUID) string {
	if userID == nil {
		return ""
	}
	var name string
	if err := h.db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", *userID).Scan(&name); err != nil {
		return ""
	}
	return name
}

// getVaultUserRole gets the user's effective role on a vault (vault membership or inherited
// organization owner/admin role)
func (h *VaultHandlers) getVaultUserRole(ctx context.Context, vaultID, userID uuid.UUID) string {
	role, err := h.policyService.VaultRole(ctx, userID, vaultID)
	if err != nil {
		if !errors.Is(err, policy.ErrVaultNotFound) {
			h.logPolicyError(err)
		}
		return ""
	}
	return role
}

// logPolicyError logs a failed permission lookup
func (h *VaultHandlers) logPolicyError(err error) {
	h.logger.Error("Failed to check permissions", zap.Error(err))
}

// getVaultCounts returns environment and secret counts for a vault
func (h *VaultHandlers) getVaultCounts(ctx context.Context, vaultID uuid.UUID) (envCount, secretCount int) {
	// Count environments
	h.db.QueryRow(ctx, "SELECT COUNT(*) FROM environments WHERE vault_id = $1", vaultID).Scan(&envCount)

	// Count secrets across all environments in this vault
	h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM secrets s
		JOIN environments e ON s.environment_id = e.id
		WHERE e.vault_id = $1
	`, vaultID).Scan(&secretCount)

	return envCount, secretCount
}

// handleVaultError maps vault service errors to appropriate HTTP responses
func (h *VaultHandlers) handleVaultError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vaults.ErrNotFound):
		h.respondError(w, http.StatusNotFound, "not_found", "Vault not found")
	case errors.Is(err, vaults.ErrDuplicate):
		h.respondError(w, http.StatusConflict, "duplicate", "Vault already exists")
	default:
		h.logger.Error("Unexpected vault error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondJSON writes a JSON response
func (h *VaultHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// respondError writes an error response
func (h *VaultHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
