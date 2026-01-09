package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/razlafan/devops-secret-manager/apps/api/internal/crypto"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const claimsContextKey contextKey = "claims"

var (
	ErrClaimsNotFound = errors.New("claims not found in context")
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// AuthMiddleware validates JWT tokens and stores claims in context
func AuthMiddleware(jwtSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "unauthorized", "Missing authorization header")
				return
			}

			// Extract Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid authorization header format")
				return
			}

			accessToken := parts[1]

			// Validate token
			claims, err := crypto.ValidateToken(accessToken, jwtSecret)
			if err != nil {
				if errors.Is(err, crypto.ErrExpiredToken) {
					respondError(w, http.StatusUnauthorized, "token_expired", "Token has expired")
					return
				}
				respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or malformed token")
				return
			}

			// Store claims in context
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)

			// Call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserClaims retrieves claims from context
func GetUserClaims(ctx context.Context) (*crypto.CustomClaims, error) {
	claims, ok := ctx.Value(claimsContextKey).(*crypto.CustomClaims)
	if !ok || claims == nil {
		return nil, ErrClaimsNotFound
	}
	return claims, nil
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, error string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   error,
		Message: message,
	})
}
