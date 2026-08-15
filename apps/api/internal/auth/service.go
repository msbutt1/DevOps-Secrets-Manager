package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/email"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/tokens"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/users"

	"github.com/google/uuid"
)

// Domain errors
var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidToken           = errors.New("invalid token")
	ErrTokenRevoked           = errors.New("token has been revoked")
	ErrTokenExpired           = errors.New("token has expired")
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailNotVerified       = errors.New("email not verified")
	ErrUserAlreadyExists      = errors.New("user already exists")
	ErrInvalidVerification    = errors.New("invalid or expired verification token")
	ErrInvalidCurrentPassword = errors.New("current password is incorrect")
	ErrPasswordTooShort       = errors.New("password must be at least 8 characters")
)

// AuthResponse represents the response returned after successful authentication
type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// UserProfile represents the user profile information
type UserProfile struct {
	ID    uuid.UUID
	Email string
	Name  string
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string
	Password string
	Name     string
}

// AuthService defines the interface for authentication operations
type AuthService interface {
	Login(ctx context.Context, email, password string) (*AuthResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
	Register(ctx context.Context, req RegisterRequest) (*uuid.UUID, error)
	VerifyEmail(ctx context.Context, token string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}

type authService struct {
	userRepo              users.Repository
	refreshTokenRepo      tokens.RefreshTokenRepository
	verificationTokenRepo email.VerificationTokenRepository
	emailService          email.EmailService
	pool                  *pgxpool.Pool
	jwtSecret             string
	accessTokenTTL        time.Duration
	refreshTokenTTL       time.Duration
	verificationTokenTTL  time.Duration
	logger                *slog.Logger
	auditService          audit.AuditService
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo users.Repository,
	refreshTokenRepo tokens.RefreshTokenRepository,
	verificationTokenRepo email.VerificationTokenRepository,
	emailService email.EmailService,
	pool *pgxpool.Pool,
	jwtSecret string,
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	logger *slog.Logger,
	auditService audit.AuditService,
) AuthService {
	return &authService{
		userRepo:              userRepo,
		refreshTokenRepo:      refreshTokenRepo,
		verificationTokenRepo: verificationTokenRepo,
		emailService:          emailService,
		pool:                  pool,
		jwtSecret:             jwtSecret,
		accessTokenTTL:        accessTokenTTL,
		refreshTokenTTL:       refreshTokenTTL,
		verificationTokenTTL:  24 * time.Hour, // 24 hours
		logger:                logger,
		auditService:          auditService,
	}
}

// Login authenticates a user and returns access and refresh tokens
func (s *authService) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Verify password
	if err := crypto.VerifyPassword(password, user.PasswordHash); err != nil {
		s.recordLogin(ctx, user, audit.ActionLoginFailure, "invalid_password")
		return nil, ErrInvalidCredentials
	}

	// Check if email is verified
	if !user.EmailVerified {
		s.recordLogin(ctx, user, audit.ActionLoginFailure, "email_not_verified")
		return nil, ErrEmailNotVerified
	}

	// Generate access token
	accessToken, err := crypto.GenerateToken(
		user.ID,
		user.Email,
		crypto.TokenTypeAccess,
		s.jwtSecret,
		s.accessTokenTTL,
	)
	if err != nil {
		return nil, err
	}

	// Generate refresh token (UUID)
	refreshTokenPlaintext := uuid.New().String()

	// Hash refresh token with SHA-256
	tokenHash := s.hashToken(refreshTokenPlaintext)

	// Create token family for rotation
	tokenFamily := uuid.New()

	// Store refresh token in database
	refreshTokenEntity := &tokens.RefreshToken{
		ID:          uuid.New(),
		UserID:      user.ID,
		TokenHash:   tokenHash,
		TokenFamily: tokenFamily,
		ExpiresAt:   time.Now().Add(s.refreshTokenTTL),
		CreatedAt:   time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshTokenEntity); err != nil {
		return nil, err
	}

	s.recordLogin(ctx, user, audit.ActionLoginSuccess, "")

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenPlaintext,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}

// Refresh validates a refresh token and issues new access and refresh tokens
func (s *authService) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// Hash incoming token
	tokenHash := s.hashToken(refreshToken)

	// Get token from repository
	storedToken, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, tokens.ErrTokenNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	// Check if token is revoked
	if storedToken.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}

	// Check if token is expired
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Revoke old token
	if err := s.refreshTokenRepo.RevokeByID(ctx, storedToken.ID); err != nil {
		return nil, err
	}

	// Generate new access token
	accessToken, err := crypto.GenerateToken(
		user.ID,
		user.Email,
		crypto.TokenTypeAccess,
		s.jwtSecret,
		s.accessTokenTTL,
	)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token (UUID)
	newRefreshTokenPlaintext := uuid.New().String()

	// Hash new refresh token
	newTokenHash := s.hashToken(newRefreshTokenPlaintext)

	// Create new refresh token with same token_family for rotation tracking
	newRefreshTokenEntity := &tokens.RefreshToken{
		ID:          uuid.New(),
		UserID:      user.ID,
		TokenHash:   newTokenHash,
		TokenFamily: storedToken.TokenFamily, // Use same token family
		ExpiresAt:   time.Now().Add(s.refreshTokenTTL),
		CreatedAt:   time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, newRefreshTokenEntity); err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshTokenPlaintext,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}

// Logout revokes a refresh token
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	// Hash token
	tokenHash := s.hashToken(refreshToken)

	// Get token from repository
	storedToken, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, tokens.ErrTokenNotFound) {
			return ErrInvalidToken
		}
		return err
	}

	// Revoke token
	if err := s.refreshTokenRepo.RevokeByID(ctx, storedToken.ID); err != nil {
		return err
	}

	return nil
}

// Me retrieves the current user profile by user ID
func (s *authService) Me(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	// Get user by ID
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &UserProfile{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}, nil
}

// recordLogin writes a login audit event for a known account; failures are logged by the audit service.
func (s *authService) recordLogin(ctx context.Context, user *users.User, action, reason string) {
	var metadata map[string]interface{}
	if reason != "" {
		metadata = map[string]interface{}{"reason": reason}
	}
	_ = s.auditService.Record(ctx, audit.Event{
		UserID:     user.ID,
		Action:     action,
		TargetType: "user",
		TargetID:   &user.ID,
		TargetName: user.Email,
		Metadata:   metadata,
	})
}

// hashToken hashes a token using SHA-256
func (s *authService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Register creates a new user account and sends a verification email
func (s *authService) Register(ctx context.Context, req RegisterRequest) (*uuid.UUID, error) {
	// Hash the password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Start a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create user
	userID := uuid.New()
	user := &users.User{
		ID:            userID,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Name:          req.Name,
		EmailVerified: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	// Create auto-organization for the user
	orgID := uuid.New()
	orgQuery := `
		INSERT INTO organizations (id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, orgQuery, orgID, req.Name+"'s Organization", time.Now(), time.Now())
	if err != nil {
		return nil, err
	}

	// Add user to organization as owner
	userOrgQuery := `
		INSERT INTO user_organizations (user_id, organization_id, role, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, userOrgQuery, userID, orgID, "owner", time.Now())
	if err != nil {
		return nil, err
	}

	// Generate verification token
	verificationToken, err := s.generateVerificationToken()
	if err != nil {
		return nil, err
	}

	// Hash the verification token
	tokenHash := s.hashToken(verificationToken)

	// Store verification token
	token := &email.VerificationToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.verificationTokenTTL),
		CreatedAt: time.Now(),
	}

	if err := s.verificationTokenRepo.Create(ctx, token); err != nil {
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Send verification email (async, don't block on this)
	go func() {
		if err := s.emailService.SendVerificationEmail(context.Background(), req.Email, req.Name, verificationToken); err != nil {
			// Log error but don't fail registration
			s.logger.Error("failed to send verification email", slog.String("user_id", userID.String()), slog.Any("error", err))
		}
	}()

	return &userID, nil
}

// VerifyEmail verifies a user's email using a verification token
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	// Hash the token
	tokenHash := s.hashToken(token)

	// Get verification token
	verificationToken, err := s.verificationTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, email.ErrTokenNotFound) {
			return ErrInvalidVerification
		}
		return err
	}

	// Check if token is already used
	if verificationToken.UsedAt != nil {
		return ErrInvalidVerification
	}

	// Check if token is expired
	if time.Now().After(verificationToken.ExpiresAt) {
		return ErrInvalidVerification
	}

	// Mark token as used
	if err := s.verificationTokenRepo.MarkAsUsed(ctx, verificationToken.ID); err != nil {
		return err
	}

	// Mark user's email as verified
	if err := s.userRepo.MarkEmailVerified(ctx, verificationToken.UserID); err != nil {
		return err
	}

	return nil
}

// generateVerificationToken generates a secure random verification token
func (s *authService) generateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ChangePassword changes a user's password after verifying their current password
func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	// Validate new password length
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}

	// Get user by ID
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// Verify current password
	if err := crypto.VerifyPassword(currentPassword, user.PasswordHash); err != nil {
		return ErrInvalidCurrentPassword
	}

	// Hash new password
	newPasswordHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password in database
	if err := s.userRepo.UpdatePassword(ctx, userID, newPasswordHash); err != nil {
		return err
	}

	return nil
}
