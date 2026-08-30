package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
)

// vaultAccessResult is the outcome of an authorization check on a vault.
type vaultAccessResult int

const (
	accessAllowed vaultAccessResult = iota
	// accessNotFound: the vault is missing or the caller has no role on it. Responding 404
	// keeps IDs belonging to other teams from being confirmed.
	accessNotFound
	// accessForbidden: the caller can see the vault but their role lacks the permission.
	accessForbidden
	accessError
)

// checkVaultAccess resolves the caller's effective vault role and checks the action.
func checkVaultAccess(ctx context.Context, ps policy.PolicyService, userID, vaultID uuid.UUID, action policy.Action) (string, vaultAccessResult, error) {
	role, err := ps.VaultRole(ctx, userID, vaultID)
	if err != nil {
		if errors.Is(err, policy.ErrVaultNotFound) {
			return "", accessNotFound, nil
		}
		return "", accessError, err
	}
	if !policy.RoleAllows(role, action) {
		return role, accessForbidden, nil
	}
	return role, accessAllowed, nil
}

// authorizeVault checks the action and writes the error response when it is not allowed.
// resource names what was requested ("Vault", "Environment", "Secret") for the 404 message.
// It returns the caller's effective role and whether the handler may continue.
func authorizeVault(w http.ResponseWriter, r *http.Request, ps policy.PolicyService, userID, vaultID uuid.UUID, action policy.Action, resource string, logError func(context.Context, error)) (string, bool) {
	role, result, err := checkVaultAccess(r.Context(), ps, userID, vaultID, action)
	switch result {
	case accessAllowed:
		return role, true
	case accessNotFound:
		writeError(w, http.StatusNotFound, "not_found", resource+" not found")
	case accessForbidden:
		writeError(w, http.StatusForbidden, "forbidden", "Your role on this vault does not allow this action")
	default:
		logError(r.Context(), err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check permissions")
	}
	return role, false
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: code, Message: message})
}
