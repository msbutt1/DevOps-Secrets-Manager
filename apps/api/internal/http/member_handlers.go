package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/vaults"
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
	AddedBy     string            `json:"added_by"` // display name; empty when unknown
	AddedByID   *uuid.UUID        `json:"added_by_id"`
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
	auditService  audit.AuditService
	policyService policy.PolicyService
	db            *pgxpool.Pool
	logger        *zap.Logger
}

// NewMemberHandlers creates a new instance of MemberHandlers
func NewMemberHandlers(vaultService vaults.VaultService, auditService audit.AuditService, policyService policy.PolicyService, db *pgxpool.Pool, logger *zap.Logger) *MemberHandlers {
	return &MemberHandlers{
		vaultService:  vaultService,
		auditService:  auditService,
		policyService: policyService,
		db:            db,
		logger:        logger,
	}
}

const invalidRoleMessage = "Invalid role. Must be one of: owner, admin, developer, oncall, viewer"

// HandleListMembers lists all members of a vault
func (h *MemberHandlers) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionMemberRead, "Vault", h.logPolicyError); !ok {
		return
	}

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

	vaultID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)

	if !policy.IsValidRole(req.Role) {
		h.respondError(w, http.StatusBadRequest, "invalid_request", invalidRoleMessage)
		return
	}

	callerRole, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionMemberManage, "Vault", h.logPolicyError)
	if !ok {
		return
	}

	// Only owners may grant the owner role, since owners can delete the vault
	if req.Role == policy.RoleOwner && callerRole != policy.RoleOwner {
		h.respondError(w, http.StatusForbidden, "forbidden", "Only vault owners can add another owner")
		return
	}

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

	if !h.checkOrganizationMembership(r.Context(), targetUser.ID, vault.OrganizationID) {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "User must be a member of the organization first")
		return
	}

	if _, err := h.getVaultMemberRole(r.Context(), targetUser.ID, vaultID); err == nil {
		h.respondError(w, http.StatusConflict, "conflict", "User is already a member of this vault")
		return
	}

	if err := h.addUserToVault(r.Context(), targetUser.ID, vaultID, req.Role, claims.UserID); err != nil {
		h.logger.Error("Failed to add user to vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to add member")
		return
	}

	member, err := h.getVaultMember(r.Context(), vaultID, targetUser.ID)
	if err != nil {
		h.logger.Error("Failed to load added member", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to load member")
		return
	}

	h.record(r, claims.UserID, audit.ActionMemberAdded, vaultID, member, map[string]interface{}{"role": member.Role})

	h.respondJSON(w, http.StatusCreated, member)
}

// HandleUpdateMember updates a member's role
func (h *MemberHandlers) HandleUpdateMember(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	vaultID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if !policy.IsValidRole(req.Role) {
		h.respondError(w, http.StatusBadRequest, "invalid_request", invalidRoleMessage)
		return
	}

	callerRole, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionMemberManage, "Vault", h.logPolicyError)
	if !ok {
		return
	}

	currentRole, err := h.getVaultMemberRole(r.Context(), targetUserID, vaultID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.respondError(w, http.StatusNotFound, "not_found", "Member not found in this vault")
			return
		}
		h.logger.Error("Failed to get member role", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to update member role")
		return
	}

	if (req.Role == policy.RoleOwner || currentRole == policy.RoleOwner) && callerRole != policy.RoleOwner {
		h.respondError(w, http.StatusForbidden, "forbidden", "Only vault owners can grant or change the owner role")
		return
	}

	if currentRole == policy.RoleOwner && req.Role != policy.RoleOwner && h.countVaultOwners(r.Context(), vaultID) <= 1 {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "A vault must keep at least one owner")
		return
	}

	if err := h.updateVaultMemberRole(r.Context(), targetUserID, vaultID, req.Role); err != nil {
		h.logger.Error("Failed to update user role", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to update member role")
		return
	}

	member, err := h.getVaultMember(r.Context(), vaultID, targetUserID)
	if err != nil {
		h.logger.Error("Failed to load updated member", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to load member")
		return
	}

	if currentRole != req.Role {
		h.record(r, claims.UserID, audit.ActionMemberRoleChanged, vaultID, member, map[string]interface{}{"old_role": currentRole, "new_role": req.Role})
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

	vaultID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid vault ID format")
		return
	}

	targetUserID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}

	callerRole, ok := authorizeVault(w, r, h.policyService, claims.UserID, vaultID, policy.ActionMemberManage, "Vault", h.logPolicyError)
	if !ok {
		return
	}

	// Prevent removing yourself
	if targetUserID == claims.UserID {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Cannot remove yourself from the vault")
		return
	}

	currentRole, err := h.getVaultMemberRole(r.Context(), targetUserID, vaultID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.respondError(w, http.StatusNotFound, "not_found", "Member not found in this vault")
			return
		}
		h.logger.Error("Failed to get member role", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to remove member")
		return
	}

	removed, err := h.getVaultMember(r.Context(), vaultID, targetUserID)
	if err != nil {
		h.logger.Error("Failed to load member", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to remove member")
		return
	}

	if currentRole == policy.RoleOwner {
		if callerRole != policy.RoleOwner {
			h.respondError(w, http.StatusForbidden, "forbidden", "Only vault owners can remove an owner")
			return
		}
		if h.countVaultOwners(r.Context(), vaultID) <= 1 {
			h.respondError(w, http.StatusBadRequest, "invalid_request", "A vault must keep at least one owner")
			return
		}
	}

	if err := h.removeUserFromVault(r.Context(), targetUserID, vaultID); err != nil {
		h.logger.Error("Failed to remove user from vault", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Failed to remove member")
		return
	}

	h.record(r, claims.UserID, audit.ActionMemberRemoved, vaultID, removed, map[string]interface{}{"role": currentRole})

	w.WriteHeader(http.StatusNoContent)
}

// Helper types and methods

type userInfo struct {
	ID    uuid.UUID
	Email string
	Name  string
}

// record writes a membership audit event targeting the member; failures are logged by the audit service.
func (h *MemberHandlers) record(r *http.Request, actorID uuid.UUID, action string, vaultID uuid.UUID, member MemberResponse, metadata map[string]interface{}) {
	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: actorID, Action: action,
		TargetType: "member", TargetID: &member.UserID, TargetName: member.Email,
		VaultID: &vaultID, Metadata: metadata,
	})
}

func (h *MemberHandlers) logPolicyError(err error) {
	h.logger.Error("Failed to check vault permissions", zap.Error(err))
}

func (h *MemberHandlers) checkOrganizationMembership(ctx context.Context, userID, organizationID uuid.UUID) bool {
	query := `SELECT 1 FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	var exists int
	err := h.db.QueryRow(ctx, query, userID, organizationID).Scan(&exists)
	return err == nil
}

const memberSelect = `
	SELECT u.id, u.email, u.name, vm.role, vm.created_at, vm.added_by, COALESCE(adder.name, '')
	FROM users u
	JOIN vault_members vm ON u.id = vm.user_id
	LEFT JOIN users adder ON adder.id = vm.added_by
	WHERE vm.vault_id = $1
`

func (h *MemberHandlers) scanMember(row pgx.Row) (MemberResponse, error) {
	var m MemberResponse
	if err := row.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.AddedAt, &m.AddedByID, &m.AddedBy); err != nil {
		return m, err
	}
	m.Permissions = h.getRolePermissions(m.Role)
	return m, nil
}

func (h *MemberHandlers) getVaultMembers(ctx context.Context, vaultID uuid.UUID) ([]MemberResponse, error) {
	rows, err := h.db.Query(ctx, memberSelect+` ORDER BY vm.created_at`, vaultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]MemberResponse, 0)
	for rows.Next() {
		m, err := h.scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (h *MemberHandlers) getVaultMember(ctx context.Context, vaultID, userID uuid.UUID) (MemberResponse, error) {
	return h.scanMember(h.db.QueryRow(ctx, memberSelect+` AND vm.user_id = $2`, vaultID, userID))
}

func (h *MemberHandlers) getUserByEmail(ctx context.Context, email string) (*userInfo, error) {
	query := `SELECT id, email, name FROM users WHERE lower(email) = lower($1) AND deleted_at IS NULL`
	var u userInfo
	err := h.db.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (h *MemberHandlers) addUserToVault(ctx context.Context, userID, vaultID uuid.UUID, role string, addedBy uuid.UUID) error {
	query := `INSERT INTO vault_members (user_id, vault_id, role, added_by, created_at) VALUES ($1, $2, $3, $4, NOW())`
	_, err := h.db.Exec(ctx, query, userID, vaultID, role, addedBy)
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

func (h *MemberHandlers) getVaultMemberRole(ctx context.Context, userID, vaultID uuid.UUID) (string, error) {
	query := `SELECT role FROM vault_members WHERE vault_id = $1 AND user_id = $2`
	var role string
	err := h.db.QueryRow(ctx, query, vaultID, userID).Scan(&role)
	return role, err
}

func (h *MemberHandlers) countVaultOwners(ctx context.Context, vaultID uuid.UUID) int {
	var count int
	if err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM vault_members WHERE vault_id = $1 AND role = 'owner'`, vaultID).Scan(&count); err != nil {
		h.logger.Error("Failed to count vault owners", zap.Error(err))
		return 0
	}
	return count
}

func (h *MemberHandlers) getRolePermissions(role string) MemberPermissions {
	p := policy.PermissionsFor(role)
	return MemberPermissions{
		CanRead:          p.CanRead,
		CanWrite:         p.CanWrite,
		CanReveal:        p.CanReveal,
		CanManageMembers: p.CanManageMembers,
		CanDelete:        p.CanDelete,
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
