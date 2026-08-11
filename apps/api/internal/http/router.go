package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	authmiddleware "github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
)

// NewRouter creates and configures a new chi router
func NewRouter(authHandlers *AuthHandlers, vaultHandlers *VaultHandlers, environmentHandlers *EnvironmentHandlers, secretHandlers *SecretHandlers, auditHandlers *AuditHandlers, memberHandlers *MemberHandlers, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", healthCheckHandler)

	// Auth routes
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandlers.HandleRegister)
		r.Post("/verify-email", authHandlers.HandleVerifyEmail)
		r.Post("/login", authHandlers.HandleLogin)
		r.Post("/refresh", authHandlers.HandleRefresh)
		r.Post("/logout", authHandlers.HandleLogout)
		// Protected routes - require authentication
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Get("/me", authHandlers.HandleMe)
		r.With(authmiddleware.AuthMiddleware(jwtSecret)).Post("/change-password", authHandlers.HandleChangePassword)
	})

	// Vault routes (protected)
	r.Route("/vaults", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Post("/", vaultHandlers.HandleCreateVault)
		r.Get("/", vaultHandlers.HandleListVaults)
		r.Get("/{id}", vaultHandlers.HandleGetVault)
		r.Put("/{id}", vaultHandlers.HandleUpdateVault)
		r.Delete("/{id}", vaultHandlers.HandleDeleteVault)

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
	})

	// Secret routes (protected)
	r.Route("/secrets", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Put("/{id}", secretHandlers.HandleUpdateSecret)
		r.Delete("/{id}", secretHandlers.HandleDeleteSecret)
		r.Post("/{id}/reveal", secretHandlers.HandleRevealSecret)
	})

	// Audit routes (protected)
	r.Route("/audit", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(jwtSecret))
		r.Get("/", auditHandlers.HandleQueryAuditLogs)
	})

	return r
}

// healthCheckHandler returns a simple health status
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
