package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/auth"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"go.uber.org/zap"
)

// Request DTOs
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Response DTOs
type AuthResponseDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RegisterResponseDTO struct {
	UserID  uuid.UUID `json:"user_id"`
	Message string    `json:"message"`
}

type VerifyEmailResponseDTO struct {
	Message string `json:"message"`
}

type ChangePasswordResponseDTO struct {
	Message string `json:"message"`
}

type UserProfileDTO struct {
	ID            uuid.UUID             `json:"id"`
	Email         string                `json:"email"`
	Name          string                `json:"name"`
	CreatedAt     time.Time             `json:"created_at"`
	Organizations []UserOrganizationDTO `json:"organizations"`
}

type UserOrganizationDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Role string    `json:"role"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// AuthHandlers handles authentication-related HTTP requests
type AuthHandlers struct {
	authService auth.AuthService
	db          *pgxpool.Pool
	logger      *zap.Logger
}

// NewAuthHandlers creates a new instance of AuthHandlers
func NewAuthHandlers(authService auth.AuthService, db *pgxpool.Pool, logger *zap.Logger) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
		db:          db,
		logger:      logger,
	}
}

// HandleLogin handles user login requests
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	authResp, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, AuthResponseDTO{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    authResp.ExpiresIn,
	})
}

// HandleRefresh handles token refresh requests
func (h *AuthHandlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	authResp, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, AuthResponseDTO{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    authResp.ExpiresIn,
	})
}

// HandleLogout handles user logout requests
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		h.handleAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleMe handles user profile requests
func (h *AuthHandlers) HandleMe(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (set by AuthMiddleware)
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Get user profile using userID from claims
	profile, err := h.authService.Me(r.Context(), claims.UserID)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	// Get user's organizations
	orgs, err := h.getUserOrganizations(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("Failed to get user organizations", zap.Error(err))
		// Don't fail the request, just return empty organizations
		orgs = []UserOrganizationDTO{}
	}

	h.respondJSON(w, http.StatusOK, UserProfileDTO{
		ID:            profile.ID,
		Email:         profile.Email,
		Name:          profile.Name,
		CreatedAt:     profile.CreatedAt,
		Organizations: orgs,
	})
}

// getUserOrganizations fetches all organizations for a user
func (h *AuthHandlers) getUserOrganizations(ctx context.Context, userID uuid.UUID) ([]UserOrganizationDTO, error) {
	query := `
		SELECT o.id, o.name, uo.role
		FROM organizations o
		JOIN user_organizations uo ON o.id = uo.organization_id
		WHERE uo.user_id = $1
		ORDER BY o.name
	`

	rows, err := h.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []UserOrganizationDTO
	for rows.Next() {
		var org UserOrganizationDTO
		if err := rows.Scan(&org.ID, &org.Name, &org.Role); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}

	if orgs == nil {
		orgs = []UserOrganizationDTO{}
	}

	return orgs, rows.Err()
}

// HandleRegister handles user registration requests
func (h *AuthHandlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Email, password, and name are required")
		return
	}

	// Register user
	userID, err := h.authService.Register(r.Context(), auth.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, RegisterResponseDTO{
		UserID:  *userID,
		Message: "Registration successful. Please check your email to verify your account.",
	})
}

// HandleVerifyEmail handles email verification requests
func (h *AuthHandlers) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate input
	if req.Token == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}

	// Verify email
	if err := h.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, VerifyEmailResponseDTO{
		Message: "Email verified successfully. You can now log in.",
	})
}

// HandleChangePassword handles password change requests
func (h *AuthHandlers) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (set by AuthMiddleware)
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate input
	if req.CurrentPassword == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Current password is required")
		return
	}

	if req.NewPassword == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "New password is required")
		return
	}

	if len(req.NewPassword) < 8 {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "New password must be at least 8 characters")
		return
	}

	// Change password
	if err := h.authService.ChangePassword(r.Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, ChangePasswordResponseDTO{
		Message: "Password changed successfully",
	})
}

// handleAuthError maps auth service errors to appropriate HTTP responses
func (h *AuthHandlers) handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		h.respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
	case errors.Is(err, auth.ErrInvalidToken):
		h.respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or malformed token")
	case errors.Is(err, auth.ErrTokenExpired):
		h.respondError(w, http.StatusUnauthorized, "token_expired", "Token has expired")
	case errors.Is(err, auth.ErrTokenRevoked):
		h.respondError(w, http.StatusUnauthorized, "token_revoked", "Token has been revoked")
	case errors.Is(err, auth.ErrUserNotFound):
		h.respondError(w, http.StatusNotFound, "user_not_found", "User not found")
	case errors.Is(err, auth.ErrEmailNotVerified):
		h.respondError(w, http.StatusForbidden, "email_not_verified", "Please verify your email address before logging in")
	case errors.Is(err, auth.ErrUserAlreadyExists):
		h.respondError(w, http.StatusConflict, "user_already_exists", "An account with this email already exists")
	case errors.Is(err, auth.ErrInvalidVerification):
		h.respondError(w, http.StatusBadRequest, "invalid_verification", "Invalid or expired verification token")
	case errors.Is(err, auth.ErrInvalidCurrentPassword):
		h.respondError(w, http.StatusBadRequest, "invalid_current_password", "Current password is incorrect")
	case errors.Is(err, auth.ErrPasswordTooShort):
		h.respondError(w, http.StatusBadRequest, "password_too_short", "Password must be at least 8 characters")
	default:
		h.logger.Error("Unexpected auth error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondJSON writes a JSON response
func (h *AuthHandlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

// respondError writes an error response
func (h *AuthHandlers) respondError(w http.ResponseWriter, status int, error string, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}
