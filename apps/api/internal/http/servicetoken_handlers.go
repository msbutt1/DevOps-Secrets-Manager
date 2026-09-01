package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/environments"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/middleware"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/secrets"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/servicetokens"
)

// ServiceTokenResponse describes a service token without its secret value
type ServiceTokenResponse struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Prefix          string     `json:"prefix"`
	EnvironmentID   uuid.UUID  `json:"environment_id"`
	EnvironmentName string     `json:"environment_name"`
	VaultID         uuid.UUID  `json:"vault_id"`
	VaultName       string     `json:"vault_name"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	LastUsedAt      *time.Time `json:"last_used_at"`
}

// CreatedServiceTokenResponse includes the plaintext token, shown only once
type CreatedServiceTokenResponse struct {
	ServiceTokenResponse
	Token string `json:"token"`
}

// CreateServiceTokenRequest names a new token and optionally limits its lifetime
type CreateServiceTokenRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days"`
}

// TokenSecret is one decrypted value returned to a service token
type TokenSecret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TokenSecretsResponse is everything a service token can read
type TokenSecretsResponse struct {
	VaultName       string        `json:"vault_name"`
	EnvironmentName string        `json:"environment_name"`
	TokenName       string        `json:"token_name"`
	Secrets         []TokenSecret `json:"secrets"`
}

// ServiceTokenHandlers manages service tokens and serves secrets to them
type ServiceTokenHandlers struct {
	tokens        servicetokens.Service
	secrets       secrets.SecretService
	envService    environments.EnvironmentService
	auditService  audit.AuditService
	policyService policy.PolicyService
	logger        *slog.Logger
}

// NewServiceTokenHandlers creates a new instance of ServiceTokenHandlers
func NewServiceTokenHandlers(tokens servicetokens.Service, secretService secrets.SecretService, envService environments.EnvironmentService, auditService audit.AuditService, policyService policy.PolicyService, logger *slog.Logger) *ServiceTokenHandlers {
	return &ServiceTokenHandlers{tokens: tokens, secrets: secretService, envService: envService, auditService: auditService, policyService: policyService, logger: logger}
}

func (h *ServiceTokenHandlers) logPolicyError(ctx context.Context, err error) {
	h.logger.ErrorContext(ctx, "Failed to check vault permissions", slog.Any("error", err))
}

// environmentForManager resolves the environment in the URL and checks the caller may manage its tokens.
func (h *ServiceTokenHandlers) environmentForManager(w http.ResponseWriter, r *http.Request) (*environments.Environment, uuid.UUID, bool) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return nil, uuid.Nil, false
	}
	envID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid environment ID format")
		return nil, uuid.Nil, false
	}
	env, err := h.envService.GetEnvironment(r.Context(), envID)
	if err != nil {
		if errors.Is(err, environments.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Environment not found")
		} else {
			h.logger.ErrorContext(r.Context(), "Failed to get environment", slog.Any("error", err))
			writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		}
		return nil, uuid.Nil, false
	}
	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, env.VaultID, policy.ActionTokenManage, "Environment", h.logPolicyError); !ok {
		return nil, uuid.Nil, false
	}
	return env, claims.UserID, true
}

// HandleCreate handles POST /envs/{id}/tokens
func (h *ServiceTokenHandlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	env, userID, ok := h.environmentForManager(w, r)
	if !ok {
		return
	}
	var req CreateServiceTokenRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	token, plaintext, err := h.tokens.Create(r.Context(), env.ID, userID, req.Name, req.ExpiresInDays)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: userID, Action: audit.ActionTokenCreated,
		TargetType: "service_token", TargetID: &token.ID, TargetName: token.Name,
		VaultID: &token.VaultID, EnvironmentID: &token.EnvironmentID,
		Metadata: map[string]interface{}{"prefix": token.Prefix, "expires_at": token.ExpiresAt},
	})
	writeJSON(w, http.StatusCreated, CreatedServiceTokenResponse{ServiceTokenResponse: toServiceTokenResponse(token), Token: plaintext})
}

// HandleList handles GET /envs/{id}/tokens
func (h *ServiceTokenHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	env, _, ok := h.environmentForManager(w, r)
	if !ok {
		return
	}
	tokens, err := h.tokens.List(r.Context(), env.ID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	out := make([]ServiceTokenResponse, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, toServiceTokenResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleRevoke handles DELETE /tokens/{id}
func (h *ServiceTokenHandlers) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	claims, err := middleware.GetUserClaims(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	tokenID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid token ID format")
		return
	}
	token, err := h.tokens.Get(r.Context(), tokenID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	if _, ok := authorizeVault(w, r, h.policyService, claims.UserID, token.VaultID, policy.ActionTokenManage, "Service token", h.logPolicyError); !ok {
		return
	}
	if err := h.tokens.Revoke(r.Context(), tokenID); err != nil {
		h.handleError(w, r, err)
		return
	}
	_ = h.auditService.Record(r.Context(), audit.Event{
		UserID: claims.UserID, Action: audit.ActionTokenRevoked,
		TargetType: "service_token", TargetID: &token.ID, TargetName: token.Name,
		VaultID: &token.VaultID, EnvironmentID: &token.EnvironmentID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// HandleTokenSecrets handles GET /token/secrets, authenticated with a service token instead of a JWT
func (h *ServiceTokenHandlers) HandleTokenSecrets(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	plaintext, found := strings.CutPrefix(auth, "Bearer ")
	if !found || !strings.HasPrefix(plaintext, servicetokens.Prefix) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "A service token is required (Authorization: Bearer "+servicetokens.Prefix+"...)")
		return
	}
	token, err := h.tokens.Authenticate(r.Context(), plaintext)
	if err != nil {
		if errors.Is(err, servicetokens.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid, revoked or expired service token")
			return
		}
		h.logger.ErrorContext(r.Context(), "Failed to authenticate service token", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		return
	}

	values, err := h.secrets.ExportEnvironment(r.Context(), token.EnvironmentID, audit.Event{
		ServiceTokenID: &token.ID,
		Metadata:       map[string]interface{}{"via": "service_token", "token_prefix": token.Prefix},
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to export environment for service token", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
		return
	}

	out := TokenSecretsResponse{VaultName: token.VaultName, EnvironmentName: token.EnvironmentName, TokenName: token.Name, Secrets: make([]TokenSecret, 0, len(values))}
	for _, v := range values {
		out.Secrets = append(out.Secrets, TokenSecret{Key: v.Key, Value: v.Value})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ServiceTokenHandlers) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, servicetokens.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Service token not found")
	case errors.Is(err, servicetokens.ErrInvalidName), errors.Is(err, servicetokens.ErrInvalidExpiry):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		h.logger.ErrorContext(r.Context(), "Unexpected service token error", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

func toServiceTokenResponse(t *servicetokens.Token) ServiceTokenResponse {
	return ServiceTokenResponse{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix,
		EnvironmentID: t.EnvironmentID, EnvironmentName: t.EnvironmentName,
		VaultID: t.VaultID, VaultName: t.VaultName, CreatedBy: t.CreatedByName,
		CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt,
	}
}
