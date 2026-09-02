package crypto

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	// TokenIssuer and TokenAudience are set on every access token and required when
	// validating, so a token signed with the same secret for another purpose or service
	// (e.g. a shared secret reused elsewhere) is not accepted by this API.
	TokenIssuer   = "devops-secrets-manager"
	TokenAudience = "devops-secrets-manager-api"

	// clockLeeway tolerates small clock differences between API instances.
	clockLeeway = 30 * time.Second
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type CustomClaims struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	TokenType string    `json:"token_type"`
	// SessionID is the refresh token family the access token was issued for.
	SessionID uuid.UUID `json:"sid"`
	jwt.RegisteredClaims
}

// GenerateToken signs an HS256 token for the user's session.
func GenerateToken(userID, sessionID uuid.UUID, email, tokenType, secret string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    TokenIssuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{TokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken verifies an access token: HS256 signature, issuer, audience, expiry and type.
func ValidateToken(tokenString, secret string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	},
		// Pinning the algorithm rules out "none" and algorithm-confusion tokens
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(TokenIssuer),
		jwt.WithAudience(TokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockLeeway),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid || claims.TokenType != TokenTypeAccess || claims.UserID == uuid.Nil || claims.Subject != claims.UserID.String() {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
