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
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/clientip"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/organizations"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/secrets"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/servicetokens"
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
	// RevealAutoHideSeconds is returned with revealed values as the client auto-hide window
	// (default 30, clamped to 5-600).
	RevealAutoHideSeconds int
	// Version is reported by /health (set at build time; "dev" when empty).
	Version string
	// TrustedProxies are CIDR ranges whose X-Forwarded-For is believed (default: loopback and private ranges).
	TrustedProxies []string
	// DisableRateLimits turns rate limiting off; only for tests.
	DisableRateLimits bool
	// CORSAllowedOrigins lists web app origins allowed to call the API from another origin.
	CORSAllowedOrigins []string
}

// New builds the API's HTTP handler.
func New(pool *pgxpool.Pool, cfg Config) (http.Handler, error) {
	startedAt := time.Now()
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	switch {
	case cfg.RevealAutoHideSeconds == 0:
		cfg.RevealAutoHideSeconds = 30
	case cfg.RevealAutoHideSeconds < 5:
		cfg.RevealAutoHideSeconds = 5
	case cfg.RevealAutoHideSeconds > 600:
		cfg.RevealAutoHideSeconds = 600
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
	vaultService := vaults.NewVaultService(vaultRepo, cfg.MasterKEK)
	environmentService := environments.NewEnvironmentService(environmentRepo)
	auditService := audit.NewAuditService(auditRepo, cfg.SLogger)
	orgRepo := organizations.NewPostgresRepository(pool)
	inviteService := organizations.NewInviteService(pool, orgRepo, cfg.Email, auditService, cfg.SLogger)
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
		auditService,
		inviteService,
	)
	secretService := secrets.NewSecretService(secretRepo, environmentRepo, vaultRepo, auditService, cfg.MasterKEK)
	policyService := policy.NewPolicyService(pool)

	// Create handlers
	authHandlers := httphandler.NewAuthHandlers(authService, pool, cfg.Logger)
	vaultHandlers := httphandler.NewVaultHandlers(vaultService, auditService, policyService, pool, cfg.Logger)
	environmentHandlers := httphandler.NewEnvironmentHandlers(environmentService, auditService, policyService, pool, cfg.Logger)
	secretHandlers := httphandler.NewSecretHandlers(secretService, environmentService, policyService, pool, cfg.Logger, cfg.RevealAutoHideSeconds)
	auditHandlers := httphandler.NewAuditHandlers(auditService, policyService, pool, cfg.SLogger)
	memberHandlers := httphandler.NewMemberHandlers(vaultService, auditService, policyService, pool, cfg.Logger)

	statsHandlers := httphandler.NewStatsHandlers(pool, cfg.Logger, startedAt, cfg.Version)
	serviceTokenHandlers := httphandler.NewServiceTokenHandlers(servicetokens.NewService(pool), secretService, environmentService, auditService, policyService, cfg.Logger)
	organizationHandlers := httphandler.NewOrganizationHandlers(
		organizations.NewService(orgRepo, auditService), inviteService, cfg.Logger)

	if cfg.TrustedProxies == nil {
		cfg.TrustedProxies = clientip.DefaultTrustedProxies
	}
	resolver, err := clientip.NewResolver(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}

	return httphandler.NewRouter(authHandlers, vaultHandlers, environmentHandlers, secretHandlers, auditHandlers, memberHandlers, statsHandlers, organizationHandlers, serviceTokenHandlers, httphandler.RouterOptions{
		JWTSecret:          cfg.JWTSecret,
		ClientIP:           resolver,
		DisableRateLimits:  cfg.DisableRateLimits,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	}), nil
}
