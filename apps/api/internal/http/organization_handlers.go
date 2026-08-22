package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/organizations"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"go.uber.org/zap"
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

// OrganizationHandlers handles organization HTTP requests
type OrganizationHandlers struct {
	service organizations.Service
	logger  *zap.Logger
}

// NewOrganizationHandlers creates a new instance of OrganizationHandlers
func NewOrganizationHandlers(service organizations.Service, logger *zap.Logger) *OrganizationHandlers {
	return &OrganizationHandlers{service: service, logger: logger}
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
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
	case errors.Is(err, organizations.ErrLastOwner), errors.Is(err, organizations.ErrInvalidName), errors.Is(err, organizations.ErrCannotRemoveSelf):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		h.logger.Error("Unexpected organization error", zap.Error(err))
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

func toOrganizationMemberResponse(m *organizations.Member) OrganizationMemberResponse {
	return OrganizationMemberResponse{
		UserID: m.UserID, Email: m.Email, Name: m.Name, Role: m.Role,
		JoinedAt: m.JoinedAt, LastLoginAt: m.LastLoginAt,
	}
}
