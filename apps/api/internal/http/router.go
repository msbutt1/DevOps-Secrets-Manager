package http

import (
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/clientip"
	authmiddleware "github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
)

// RouterOptions configures cross-cutting behaviour of the router.
type RouterOptions struct {
	JWTSecret string
	// ClientIP resolves client addresses behind trusted proxies (required).
	ClientIP *clientip.Resolver
	// DisableRateLimits turns rate limiting off; only for tests.
	DisableRateLimits bool
	// CORSAllowedOrigins lists browser origins allowed to call the API cross-origin.
	CORSAllowedOrigins []string
	// Logger receives access logs and recovered panics (required).
	Logger *slog.Logger
}

// maxJSONBodyBytes caps request bodies read by the API.
const maxJSONBodyBytes = 1 << 20

// NewRouter creates and configures a new chi router
func NewRouter(authHandlers *AuthHandlers, vaultHandlers *VaultHandlers, environmentHandlers *EnvironmentHandlers, secretHandlers *SecretHandlers, auditHandlers *AuditHandlers, memberHandlers *MemberHandlers, statsHandlers *StatsHandlers, organizationHandlers *OrganizationHandlers, serviceTokenHandlers *ServiceTokenHandlers, opts RouterOptions) *chi.Mux {
	r := chi.NewRouter()
	jwtSecret := opts.JWTSecret
	rl := rateLimiter{disabled: opts.DisableRateLimits}

	// Middleware
	r.Use(requestLogging(opts.Logger, opts.ClientIP))
	r.Use(securityHeaders)
	r.Use(corsMiddleware(opts.CORSAllowedOrigins))
	r.Use(opts.ClientIP.Middleware)
	r.Use(audit.RequestContext)

	// Health check endpoint
	r.Get("/health", statsHandlers.HandleHealth)

	// Auth routes
	r.Route("/auth", func(r chi.Router) {
		r.With(rl.limit("register", 10, time.Hour, byIP)).Post("/register", authHandlers.HandleRegister)
		r.With(rl.limit("verify", 20, time.Minute, byIP)).Post("/verify-email", authHandlers.HandleVerifyEmail)
		r.With(
			rl.limit("resend-verification-ip", 10, time.Hour, byIP),
			rl.limit("resend-verification-email", 3, time.Hour, byLoginEmail),
		).Post("/resend-verification", authHandlers.HandleResendVerification)
		r.With(
			rl.limit("forgot-password-ip", 10, time.Hour, byIP),
			rl.limit("forgot-password-email", 3, time.Hour, byLoginEmail),
		).Post("/forgot-password", authHandlers.HandleForgotPassword)
		r.With(rl.limit("reset-password", 20, time.Hour, byIP)).Post("/reset-password", authHandlers.HandleResetPassword)
		r.With(
			rl.limit("login-ip", 20, time.Minute, byIP),
			rl.limit("login-account", 10, 5*time.Minute, byLoginEmail),
		).Post("/login", authHandlers.HandleLogin)
		r.With(rl.limit("refresh", 30, time.Minute, byIP)).Post("/refresh", authHandlers.HandleRefresh)
		r.Post("/logout", authHandlers.HandleLogout)
		// Protected routes - require authentication
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Get("/me", authHandlers.HandleMe)
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Post("/change-password", authHandlers.HandleChangePassword)
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Get("/sessions", authHandlers.HandleListSessions)
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Delete("/sessions/{id}", authHandlers.HandleRevokeSession)
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Post("/sessions/revoke-others", authHandlers.HandleRevokeOtherSessions)
	})

	// Organization routes (protected)
	r.Route("/orgs", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Get("/", organizationHandlers.HandleList)
		r.Get("/{id}", organizationHandlers.HandleGet)
		r.Patch("/{id}", organizationHandlers.HandleUpdate)
		r.Get("/{id}/members", organizationHandlers.HandleListMembers)
		r.Put("/{id}/members/{userId}", organizationHandlers.HandleUpdateMember)
		r.Delete("/{id}/members/{userId}", organizationHandlers.HandleRemoveMember)
		r.Get("/{id}/invites", organizationHandlers.HandleListInvites)
		r.Post("/{id}/invites", organizationHandlers.HandleCreateInvite)
		r.Delete("/{id}/invites/{inviteId}", organizationHandlers.HandleRevokeInvite)
	})

	// Invitation links: looking one up needs only the token; accepting needs a session
	r.With(rl.limit("invite-lookup", 20, time.Minute, byIP)).Post("/invites/lookup", organizationHandlers.HandleLookupInvite)
	r.With(authmiddleware.AuthMiddleware(jwtSecret)).Post("/invites/accept", organizationHandlers.HandleAcceptInvite)

	// Vault routes (protected)
	r.Route("/vaults", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Post("/", vaultHandlers.HandleCreateVault)
		r.Get("/", vaultHandlers.HandleListVaults)
		r.Get("/{id}", vaultHandlers.HandleGetVault)
		r.Put("/{id}", vaultHandlers.HandleUpdateVault)
		r.Delete("/{id}", vaultHandlers.HandleDeleteVault)
		r.With(rl.limit("rotate-key", 10, time.Hour, byUser)).Post("/{id}/rotate-key", vaultHandlers.HandleRotateKey)

		// Environment routes nested under vaults
		r.Post("/{vault_id}/envs", environmentHandlers.HandleCreateEnvironment)
		r.Get("/{vault_id}/envs", environmentHandlers.HandleListEnvironments)

		// Member routes nested under vaults
		r.Get("/{id}/members", memberHandlers.HandleListMembers)
		r.Post("/{id}/members", memberHandlers.HandleAddMember)
		r.Put("/{id}/members/{userId}", memberHandlers.HandleUpdateMember)
		r.Delete("/{id}/members/{userId}", memberHandlers.HandleRemoveMember)
	})

	// Environment routes (protected)
	r.Route("/envs", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Get("/{id}", environmentHandlers.HandleGetEnvironment)
		r.Put("/{id}", environmentHandlers.HandleUpdateEnvironment)
		r.Delete("/{id}", environmentHandlers.HandleDeleteEnvironment)

		// Secret routes nested under environments
		r.Get("/{id}/secrets", secretHandlers.HandleListSecrets)
		r.Post("/{id}/secrets", secretHandlers.HandleCreateSecret)
		r.With(rl.limit("export", 30, time.Minute, byUser)).Post("/{id}/export", secretHandlers.HandleExportEnvironment)
		r.Post("/{id}/import", secretHandlers.HandleImportEnvironment)
		r.With(rl.limit("export", 30, time.Minute, byUser)).Post("/{id}/copy-from", secretHandlers.HandleCopySecrets)

		// Service tokens scoped to the environment
		r.Get("/{id}/tokens", serviceTokenHandlers.HandleList)
		r.Post("/{id}/tokens", serviceTokenHandlers.HandleCreate)
	})

	// Secret routes (protected)
	r.Route("/secrets", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Put("/{id}", secretHandlers.HandleUpdateSecret)
		r.Delete("/{id}", secretHandlers.HandleDeleteSecret)
		r.With(rl.limit("reveal", 60, time.Minute, byUser)).Post("/{id}/reveal", secretHandlers.HandleRevealSecret)
		r.Get("/{id}/versions", secretHandlers.HandleListVersions)
		r.With(rl.limit("reveal", 60, time.Minute, byUser)).Post("/{id}/versions/{version}/reveal", secretHandlers.HandleRevealVersion)
		r.Post("/{id}/versions/{version}/restore", secretHandlers.HandleRestoreVersion)
	})

	// Dashboard statistics (protected)
	r.With(authmiddleware.AuthMiddleware(jwtSecret)).Get("/stats", statsHandlers.HandleStats)
	r.With(authmiddleware.AuthMiddleware(jwtSecret)).Get("/alerts", statsHandlers.HandleAlerts)

	// Service tokens: revocation by users, reading by the token itself
	r.With(authmiddleware.AuthMiddleware(jwtSecret)).Delete("/tokens/{id}", serviceTokenHandlers.HandleRevoke)
	r.With(
		rl.limit("token-ip", 60, time.Minute, byIP),
		rl.limit("token", 30, time.Minute, byBearer),
	).Get("/token/secrets", serviceTokenHandlers.HandleTokenSecrets)

	// Audit routes (protected)
	r.Route("/audit", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Get("/", auditHandlers.HandleQueryAuditLogs)
	})

	return r
}
