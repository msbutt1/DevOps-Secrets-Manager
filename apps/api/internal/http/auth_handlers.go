package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/auth"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/organizations"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/validate"
	"go.uber.org/zap"
)

// Request DTOs
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// UseCookie asks for the refresh token in an HttpOnly cookie instead of the body (web app).
	UseCookie bool `json:"use_cookie"`
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	InviteToken string `json:"invite_token"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	UseCookie    bool   `json:"use_cookie"`
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
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RegisterResponseDTO struct {
	UserID               uuid.UUID `json:"user_id"`
	Message              string    `json:"message"`
	VerificationRequired bool      `json:"verification_required"`
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
	cookie      RefreshCookie
}

// NewAuthHandlers creates a new instance of AuthHandlers
func NewAuthHandlers(authService auth.AuthService, db *pgxpool.Pool, logger *zap.Logger, cookie RefreshCookie) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
		db:          db,
		logger:      logger,
		cookie:      cookie,
	}
}

// HandleLogin handles user login requests
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	authResp, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	h.respondTokens(w, authResp, req.UseCookie)
}

// HandleRefresh handles token refresh requests
func (h *AuthHandlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	token, fromCookie := req.RefreshToken, false
	if token == "" {
		token, fromCookie = h.cookie.read(r)
	}

	authResp, err := h.authService.Refresh(r.Context(), token)
	if err != nil {
		// The cookie is left alone: another tab may have just rotated it, and clearing it
		// here would overwrite that tab's new cookie. A revoked token is useless anyway.
		h.handleAuthError(w, err)
		return
	}

	h.respondTokens(w, authResp, fromCookie || req.UseCookie)
}

// HandleLogout handles user logout requests
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	token := req.RefreshToken
	if cookieToken, ok := h.cookie.read(r); ok {
		// Always drop the cookie, even if the token turns out to be unknown
		h.cookie.clear(w)
		if token == "" {
			token = cookieToken
		}
	}

	if err := h.authService.Logout(r.Context(), token); err != nil {
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
	if !decodeJSON(w, r, &req) {
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Email, password, and name are required")
		return
	}
	if rejectInvalid(w, validate.First(validate.Name("name", req.Name), validate.Email(req.Email), validate.PasswordLength(req.Password))) {
		return
	}

	// Register user
	result, err := h.authService.Register(r.Context(), auth.RegisterRequest{
		Email:       req.Email,
		Password:    req.Password,
		Name:        req.Name,
		InviteToken: req.InviteToken,
	})
	if err != nil {
		h.handleAuthError(w, err)
		return
	}

	message := "Registration successful. Please check your email to verify your account."
	if !result.VerificationRequired {
		message = "Account created and invitation accepted. You can now log in."
	}
	h.respondJSON(w, http.StatusCreated, RegisterResponseDTO{
		UserID:               result.UserID,
		Message:              message,
		VerificationRequired: result.VerificationRequired,
	})
}

// HandleVerifyEmail handles email verification requests
func (h *AuthHandlers) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if !decodeJSON(w, r, &req) {
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
	if !decodeJSON(w, r, &req) {
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

	if rejectInvalid(w, validate.PasswordLength(req.NewPassword)) {
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
	var locked *auth.LockedError
	switch {
	case errors.As(err, &locked):
		w.Header().Set("Retry-After", strconv.Itoa(int(locked.RetryAfter().Seconds())))
		h.respondError(w, http.StatusTooManyRequests, "account_locked", "Too many failed login attempts. Try again in "+locked.RetryAfter().String()+".")
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
	case validate.IsValidationError(err):
		h.respondError(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, organizations.ErrInviteNotFound):
		h.respondError(w, http.StatusBadRequest, "invalid_invite", "This invitation is invalid, expired or already used")
	case errors.Is(err, organizations.ErrInviteEmailMismatch):
		h.respondError(w, http.StatusBadRequest, "invite_email_mismatch", "Register with the email address the invitation was sent to")
	default:
		h.logger.Error("Unexpected auth error", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

// respondTokens writes new tokens, moving the refresh token into the cookie when asked.
func (h *AuthHandlers) respondTokens(w http.ResponseWriter, authResp *auth.AuthResponse, useCookie bool) {
	dto := AuthResponseDTO{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    authResp.ExpiresIn,
	}
	if useCookie {
		h.cookie.set(w, authResp.RefreshToken)
		dto.RefreshToken = ""
	}
	h.respondJSON(w, http.StatusOK, dto)
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
