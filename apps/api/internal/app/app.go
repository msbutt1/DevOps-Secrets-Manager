// Package app wires repositories, services and HTTP handlers into the API router.
// The server and the integration tests both build the API through New.
package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/auth"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/email"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/environments"
	httphandler "github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/secrets"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/tokens"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/users"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/vaults"
	"go.uber.org/zap"
)

// Config holds everything the API needs besides the database pool.
type Config struct {
	MasterKEK       []byte
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Email           email.EmailService
	Logger          *zap.Logger
	SLogger         *slog.Logger
}

// New builds the API's HTTP handler.
func New(pool *pgxpool.Pool, cfg Config) http.Handler {
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 7 * 24 * time.Hour
	}

	// Create repositories
	userRepo := users.NewPostgresRepository(pool)
	refreshTokenRepo := tokens.NewPostgresRepository(pool)
	verificationTokenRepo := email.NewVerificationTokenRepository(pool)
	vaultRepo := vaults.NewPostgresRepository(pool)
	environmentRepo := environments.NewPostgresRepository(pool)
	secretRepo := secrets.NewPostgresRepository(pool)
	auditRepo := audit.NewPostgresRepository(pool)

	// Create services
	authService := auth.NewAuthService(
		userRepo,
		refreshTokenRepo,
		verificationTokenRepo,
		cfg.Email,
		pool,
		cfg.JWTSecret,
		cfg.AccessTokenTTL,
		cfg.RefreshTokenTTL,
		cfg.SLogger,
	)
	vaultService := vaults.NewVaultService(vaultRepo, cfg.MasterKEK)
	environmentService := environments.NewEnvironmentService(environmentRepo)
	auditService := audit.NewAuditService(auditRepo, cfg.SLogger)
	secretService := secrets.NewSecretService(secretRepo, environmentRepo, vaultRepo, auditService, cfg.MasterKEK)
	policyService := policy.NewPolicyService(pool)

	// Create handlers
	authHandlers := httphandler.NewAuthHandlers(authService, pool, cfg.Logger)
	vaultHandlers := httphandler.NewVaultHandlers(vaultService, policyService, pool, cfg.Logger)
	environmentHandlers := httphandler.NewEnvironmentHandlers(environmentService, policyService, pool, cfg.Logger)
	secretHandlers := httphandler.NewSecretHandlers(secretService, environmentService, policyService, pool, cfg.Logger)
	auditHandlers := httphandler.NewAuditHandlers(auditService, policyService, pool, cfg.SLogger)
	memberHandlers := httphandler.NewMemberHandlers(vaultService, policyService, pool, cfg.Logger)

	return httphandler.NewRouter(authHandlers, vaultHandlers, environmentHandlers, secretHandlers, auditHandlers, memberHandlers, cfg.JWTSecret)
}
