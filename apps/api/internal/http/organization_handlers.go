package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/organizations"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"log/slog"
)

// OrganizationResponse is an organization with the caller's role in it
type OrganizationResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	MemberCount int       `json:"member_count"`
	VaultCount  int       `json:"vault_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrganizationMemberResponse is a person in an organization
type OrganizationMemberResponse struct {
	UserID      uuid.UUID  `json:"user_id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	JoinedAt    time.Time  `json:"joined_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

// UpdateOrganizationRequest renames an organization
type UpdateOrganizationRequest struct {
	Name string `json:"name"`
}

// InviteResponse is an open invitation
type InviteResponse struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	Email            string     `json:"email"`
	Role             string     `json:"role"`
	InvitedBy        string     `json:"invited_by"`
	InvitedByID      *uuid.UUID `json:"invited_by_id"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
}

// CreateInviteResponse is a new invitation and whether its email went out
type CreateInviteResponse struct {
	InviteResponse
	EmailSent bool `json:"email_sent"`
}

// InviteLookupResponse describes an invitation to someone who has its token
type InviteLookupResponse struct {
	OrganizationName string    `json:"organization_name"`
	Email            string    `json:"email"`
	Role             string    `json:"role"`
	InvitedBy        string    `json:"invited_by"`
	ExpiresAt        time.Time `json:"expires_at"`
	AccountExists    bool      `json:"account_exists"`
}

// CreateInviteRequest invites someone by email
type CreateInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// InviteTokenRequest carries an invitation token in the body, so it never appears in URLs or access logs
type InviteTokenRequest struct {
	Token string `json:"token"`
}

// OrganizationHandlers handles organization HTTP requests
type OrganizationHandlers struct {
	service organizations.Service
	invites organizations.InviteService
	logger  *slog.Logger
}

// NewOrganizationHandlers creates a new instance of OrganizationHandlers
func NewOrganizationHandlers(service organizations.Service, invites organizations.InviteService, logger *slog.Logger) *OrganizationHandlers {
	return &OrganizationHandlers{service: service, invites: invites, logger: logger}
}

// HandleCreateInvite handles POST /orgs/{id}/invites
func (h *OrganizationHandlers) HandleCreateInvite(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	var req CreateInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !policy.IsValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid_request", invalidRoleMessage)
		return
	}
	invite, sent, err := h.invites.Create(r.Context(), callerID, orgID, req.Email, req.Role)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, CreateInviteResponse{InviteResponse: toInviteResponse(invite), EmailSent: sent})
}

// HandleListInvites handles GET /orgs/{id}/invites
func (h *OrganizationHandlers) HandleListInvites(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	invites, err := h.invites.ListOpen(r.Context(), callerID, orgID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	out := make([]InviteResponse, 0, len(invites))
	for _, inv := range invites {
		out = append(out, toInviteResponse(inv))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleRevokeInvite handles DELETE /orgs/{id}/invites/{inviteId}
func (h *OrganizationHandlers) HandleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	inviteID, err := uuid.Parse(chi.URLParam(r, "inviteId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid invitation ID format")
		return
	}
	if err := h.invites.Revoke(r.Context(), callerID, orgID, inviteID); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleLookupInvite handles POST /invites/lookup (no authentication)
func (h *OrganizationHandlers) HandleLookupInvite(w http.ResponseWriter, r *http.Request) {
	var req InviteTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}
	invite, accountExists, err := h.invites.Lookup(r.Context(), req.Token)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, InviteLookupResponse{
		OrganizationName: invite.OrganizationName, Email: invite.Email, Role: invite.Role,
		InvitedBy: invite.InvitedByName, ExpiresAt: invite.ExpiresAt, AccountExists: accountExists,
	})
}

// HandleAcceptInvite handles POST /invites/accept for a logged-in user
func (h *OrganizationHandlers) HandleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req InviteTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}
	org, err := h.invites.Accept(r.Context(), claims.UserID, req.Token)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// HandleList handles GET /orgs
func (h *OrganizationHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	orgs, err := h.service.List(r.Context(), claims.UserID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	out := make([]OrganizationResponse, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, toOrganizationResponse(o))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleGet handles GET /orgs/{id}
func (h *OrganizationHandlers) HandleGet(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	org, err := h.service.Get(r.Context(), callerID, orgID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// HandleUpdate handles PATCH /orgs/{id}
func (h *OrganizationHandlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	var req UpdateOrganizationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	org, err := h.service.Rename(r.Context(), callerID, orgID, req.Name)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// HandleListMembers handles GET /orgs/{id}/members
func (h *OrganizationHandlers) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	members, err := h.service.ListMembers(r.Context(), callerID, orgID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	out := make([]OrganizationMemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, toOrganizationMemberResponse(m))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleUpdateMember handles PUT /orgs/{id}/members/{userId}
func (h *OrganizationHandlers) HandleUpdateMember(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}
	var req UpdateMemberRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !policy.IsValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid_request", invalidRoleMessage)
		return
	}
	member, err := h.service.ChangeMemberRole(r.Context(), callerID, orgID, userID, req.Role)
	if err != nil {
		h.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrganizationMemberResponse(member))
}

// HandleRemoveMember handles DELETE /orgs/{id}/members/{userId}
func (h *OrganizationHandlers) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	callerID, orgID, ok := h.claimsAndOrg(w, r)
	if !ok {
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID format")
		return
	}
	if err := h.service.RemoveMember(r.Context(), callerID, orgID, userID); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OrganizationHandlers) claimsAndOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid organization ID format")
		return uuid.Nil, uuid.Nil, false
	}
	return claims.UserID, orgID, true
}

func (h *OrganizationHandlers) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, organizations.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Organization not found")
	case errors.Is(err, organizations.ErrMemberNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Member not found in this organization")
	case errors.Is(err, organizations.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Your organization role does not allow this action")
	case errors.Is(err, organizations.ErrInviteNotFound):
		writeError(w, http.StatusNotFound, "invalid_invite", "This invitation is invalid, expired or already used")
	case errors.Is(err, organizations.ErrInviteEmailMismatch):
		writeError(w, http.StatusForbidden, "invite_email_mismatch", "This invitation was sent to a different email address; log in with that account")
	case errors.Is(err, organizations.ErrAlreadyMember):
		writeError(w, http.StatusConflict, "already_member", err.Error())
	case errors.Is(err, organizations.ErrLastOwner), errors.Is(err, organizations.ErrInvalidName), errors.Is(err, organizations.ErrCannotRemoveSelf), errors.Is(err, organizations.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		h.logger.Error("Unexpected organization error", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

func toOrganizationResponse(o *organizations.Organization) OrganizationResponse {
	return OrganizationResponse{
		ID: o.ID, Name: o.Name, Role: o.Role,
		MemberCount: o.MemberCount, VaultCount: o.VaultCount,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

func toInviteResponse(inv *organizations.Invite) InviteResponse {
	return InviteResponse{
		ID: inv.ID, OrganizationID: inv.OrganizationID, OrganizationName: inv.OrganizationName,
		Email: inv.Email, Role: inv.Role, InvitedBy: inv.InvitedByName, InvitedByID: inv.InvitedByID,
		CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt,
	}
}

func toOrganizationMemberResponse(m *organizations.Member) OrganizationMemberResponse {
	return OrganizationMemberResponse{
		UserID: m.UserID, Email: m.Email, Name: m.Name, Role: m.Role,
		JoinedAt: m.JoinedAt, LastLoginAt: m.LastLoginAt,
	}
}
