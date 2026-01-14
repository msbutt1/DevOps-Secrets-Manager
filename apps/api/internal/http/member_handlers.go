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
	"github.com/razlafan/devops-secret-manager/apps/api/internal/http/middleware"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/policy"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/vaults"
	"go.uber.org/zap"
)

// Request DTOs
type AddMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UpdateMemberRequest struct {
	Role string `json:"role"`
}

// Response DTOs
type MemberResponse struct {
	UserID      uuid.UUID         `json:"user_id"`
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Role        string            `json:"role"`
	Permissions MemberPermissions `json:"permissions"`
	AddedAt     time.Time         `json:"added_at"`
	AddedBy     string            `json:"added_by"`
}

type MemberPermissions struct {
	CanRead          bool `json:"can_read"`
	CanWrite         bool `json:"can_write"`
	CanReveal        bool `json:"can_reveal"`
	CanManageMembers bool `json:"can_manage_members"`
	CanDelete        bool `json:"can_delete"`
}

// MemberHandlers handles member-related HTTP requests
type MemberHandlers struct {
	vaultService  vaults.VaultService
	policyService policy.PolicyService
	db            *pgxpool.Pool
	logger        *zap.Logger
}

// NewMemberHandlers creates a new instance of MemberHandlers
func NewMemberHandlers(vaultService vaults.VaultService, policyService policy.PolicyService, db *pgxpool.Pool, logger *zap.Logger) *MemberHandlers {
	return &MemberHandlers{
		vaultService:  vaultService,
		policyService: policyService,
		db:            db,
		logger:        logger,
	}
}

// HandleListMembers lists all members of a vault
func (h *MemberHandlers) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	// Check if user has permission to view members (must be member of vault)
	if !h.checkVaultMembership(r.Context(), claims.UserID, vaultID) {
		h.respondError(w, http.StatusForbidden, "forbidden", "User is not a member of this vault")
		return
	}

	// Get all members of the vault
	members, err := h.getVaultMembers(r.Context(), vaultID)
	if err != nil {
		h.logger.Error("Failed to get vault members", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get members")
		return
	}

	h.respondJSON(w, http.StatusOK, members)
}

// HandleAddMember adds a new member to the vault
func (h *MemberHandlers) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate role
	validRoles := map[string]bool{"owner": true, "admin": true, "developer": true, "oncall": true, "viewer": true}
	if !validRoles[req.Role] {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid role. Must be one of: owner, admin, developer, oncall, viewer")
		return
	}

	// Get vault to get organization ID
	vault, err := h.vaultService.GetVault(r.Context(), vaultID)
	if err != nil {
		if errors.Is(err, vaults.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Vault not found")
			return
		}
		h.logger.Error("Failed to get vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		return
	}

	// Check if user has permission to manage members
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionMemberManage, vault.OrganizationID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions to manage members")
		return
	}

	// Find user by email
	targetUser, err := h.getUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.respondError(w, http.StatusNotFound, "not_found", "User not found with that email")
			return
		}
		h.logger.Error("Failed to find user", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to find user")
		return
	}

	// Verify user is in the organization
	if !h.checkOrganizationMembership(r.Context(), targetUser.ID, vault.OrganizationID) {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "User must be a member of the organization first")
		return
	}

	// Check if user is already a member of this vault
	if h.checkVaultMembership(r.Context(), targetUser.ID, vaultID) {
		h.respondError(w, http.StatusConflict, "conflict", "User is already a member of this vault")
		return
	}

	// Add user to vault
	err = h.addUserToVault(r.Context(), targetUser.ID, vaultID, req.Role)
	if err != nil {
		h.logger.Error("Failed to add user to vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to add member")
		return
	}

	// Get the adder's name
	adderName := h.getUserName(r.Context(), claims.UserID)

	member := MemberResponse{
		UserID:      targetUser.ID,
		Email:       targetUser.Email,
		Name:        targetUser.Name,
		Role:        req.Role,
		Permissions: h.getRolePermissions(req.Role),
		AddedAt:     time.Now(),
		AddedBy:     adderName,
	}

	h.respondJSON(w, http.StatusCreated, member)
}

// HandleUpdateMember updates a member's role
func (h *MemberHandlers) HandleUpdateMember(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate role
	validRoles := map[string]bool{"owner": true, "admin": true, "developer": true, "oncall": true, "viewer": true}
	if !validRoles[req.Role] {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid role")
		return
	}

	// Get vault to get organization ID
	vault, err := h.vaultService.GetVault(r.Context(), vaultID)
	if err != nil {
		if errors.Is(err, vaults.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Vault not found")
			return
		}
		h.logger.Error("Failed to get vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		return
	}

	// Check if user has permission to manage members
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionMemberManage, vault.OrganizationID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions to manage members")
		return
	}

	// Update user's role in vault
	err = h.updateVaultMemberRole(r.Context(), targetUserID, vaultID, req.Role)
	if err != nil {
		h.logger.Error("Failed to update user role", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to update member role")
		return
	}

	// Get updated member info
	targetUser, _ := h.getUserByID(r.Context(), targetUserID)
	adderName := h.getUserName(r.Context(), claims.UserID)

	member := MemberResponse{
		UserID:      targetUserID,
		Email:       targetUser.Email,
		Name:        targetUser.Name,
		Role:        req.Role,
		Permissions: h.getRolePermissions(req.Role),
		AddedAt:     time.Now(),
		AddedBy:     adderName,
	}

	h.respondJSON(w, http.StatusOK, member)
}

// HandleRemoveMember removes a member from the vault
func (h *MemberHandlers) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultIDStr := chi.URLParam(r, "id")
	vaultID, err := uuid.Parse(vaultIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}

	// Get vault to get organization ID
	vault, err := h.vaultService.GetVault(r.Context(), vaultID)
	if err != nil {
		if errors.Is(err, vaults.ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Vault not found")
			return
		}
		h.logger.Error("Failed to get vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		return
	}

	// Check if user has permission to manage members
	allowed, err := h.policyService.Can(r.Context(), claims.UserID, policy.ActionMemberManage, vault.OrganizationID)
	if err != nil || !allowed {
		h.respondError(w, http.StatusForbidden, "forbidden", "Insufficient permissions to manage members")
		return
	}

	// Prevent removing yourself
	if targetUserID == claims.UserID {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Cannot remove yourself from the vault")
		return
	}

	// Remove user from vault
	err = h.removeUserFromVault(r.Context(), targetUserID, vaultID)
	if err != nil {
		h.logger.Error("Failed to remove user from vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to remove member")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper types and methods

type userInfo struct {
	ID    uuid.UUID
	Email string
	Name  string
}

func (h *MemberHandlers) checkOrganizationMembership(ctx context.Context, userID, organizationID uuid.UUID) bool {
	query := `SELECT 1 FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	var exists int
	err := h.db.QueryRow(ctx, query, userID, organizationID).Scan(&exists)
	return err == nil
}

func (h *MemberHandlers) getVaultMembers(ctx context.Context, vaultID uuid.UUID) ([]MemberResponse, error) {
	query := `
		SELECT u.id, u.email, u.name, vm.role, vm.created_at
		FROM users u
		JOIN vault_members vm ON u.id = vm.user_id
		WHERE vm.vault_id = $1
		ORDER BY vm.created_at
	`
	rows, err := h.db.Query(ctx, query, vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []MemberResponse
	for rows.Next() {
		var m MemberResponse
		var addedAt time.Time
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &addedAt); err != nil {
			return nil, err
		}
		m.AddedAt = addedAt
		m.AddedBy = "System"
		m.Permissions = h.getRolePermissions(m.Role)
		members = append(members, m)
	}
	return members, rows.Err()
}

func (h *MemberHandlers) getUserByEmail(ctx context.Context, email string) (*userInfo, error) {
	query := `SELECT id, email, name FROM users WHERE email = $1`
	var u userInfo
	err := h.db.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (h *MemberHandlers) getUserByID(ctx context.Context, id uuid.UUID) (*userInfo, error) {
	query := `SELECT id, email, name FROM users WHERE id = $1`
	var u userInfo
	err := h.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (h *MemberHandlers) getUserName(ctx context.Context, userID uuid.UUID) string {
	var name string
	h.db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", userID).Scan(&name)
	if name == "" {
		return "Unknown"
	}
	return name
}

func (h *MemberHandlers) addUserToVault(ctx context.Context, userID, vaultID uuid.UUID, role string) error {
	query := `INSERT INTO vault_members (user_id, vault_id, role, created_at) VALUES ($1, $2, $3, NOW())`
	_, err := h.db.Exec(ctx, query, userID, vaultID, role)
	return err
}

func (h *MemberHandlers) updateVaultMemberRole(ctx context.Context, userID, vaultID uuid.UUID, role string) error {
	query := `UPDATE vault_members SET role = $1 WHERE user_id = $2 AND vault_id = $3`
	_, err := h.db.Exec(ctx, query, role, userID, vaultID)
	return err
}

func (h *MemberHandlers) removeUserFromVault(ctx context.Context, userID, vaultID uuid.UUID) error {
	query := `DELETE FROM vault_members WHERE user_id = $1 AND vault_id = $2`
	_, err := h.db.Exec(ctx, query, userID, vaultID)
	return err
}

func (h *MemberHandlers) checkVaultMembership(ctx context.Context, userID, vaultID uuid.UUID) bool {
	query := `SELECT 1 FROM vault_members WHERE vault_id = $1 AND user_id = $2`
	var exists int
	err := h.db.QueryRow(ctx, query, vaultID, userID).Scan(&exists)
	return err == nil
}

func (h *MemberHandlers) getVaultMemberRole(ctx context.Context, userID, vaultID uuid.UUID) (string, error) {
	query := `SELECT role FROM vault_members WHERE vault_id = $1 AND user_id = $2`
	var role string
	err := h.db.QueryRow(ctx, query, vaultID, userID).Scan(&role)
	return role, err
}

func (h *MemberHandlers) getRolePermissions(role string) MemberPermissions {
	switch role {
	case "owner":
		return MemberPermissions{CanRead: true, CanWrite: true, CanReveal: true, CanManageMembers: true, CanDelete: true}
	case "admin":
		return MemberPermissions{CanRead: true, CanWrite: true, CanReveal: true, CanManageMembers: true, CanDelete: false}
	case "developer":
		return MemberPermissions{CanRead: true, CanWrite: true, CanReveal: true, CanManageMembers: false, CanDelete: false}
	case "oncall":
		return MemberPermissions{CanRead: true, CanWrite: false, CanReveal: true, CanManageMembers: false, CanDelete: false}
	default: // viewer
		return MemberPermissions{CanRead: true, CanWrite: false, CanReveal: false, CanManageMembers: false, CanDelete: false}
	}
}

func (h *MemberHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *MemberHandlers) respondError(w http.ResponseWriter, status int, errorCode string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}
