package crypto

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testJWTSecret = "unit-test-jwt-secret-3c7e9a1f5b2d8e4c6a0f"

func sign(t *testing.T, method jwt.SigningMethod, key any, mutate func(*CustomClaims)) string {
	t.Helper()
	userID := uuid.New()
	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Email:     "user@example.test",
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    TokenIssuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{TokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	if mutate != nil {
		mutate(&claims)
	}
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGenerateTokenRoundTrip(t *testing.T) {
	userID := uuid.New()
	token, err := GenerateToken(userID, "user@example.test", TokenTypeAccess, testJWTSecret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token, testJWTSecret)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if claims.UserID != userID || claims.Issuer != TokenIssuer || claims.Subject != userID.String() || claims.ID == "" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != TokenAudience {
		t.Fatalf("unexpected audience: %v", claims.Audience)
	}
}

func TestValidateTokenRejects(t *testing.T) {
	hs256 := jwt.SigningMethodHS256
	key := []byte(testJWTSecret)

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{"wrong secret", sign(t, hs256, []byte("another-secret-that-is-long-enough-000"), nil), ErrInvalidToken},
		{"missing issuer", sign(t, hs256, key, func(c *CustomClaims) { c.Issuer = "" }), ErrInvalidToken},
		{"other issuer", sign(t, hs256, key, func(c *CustomClaims) { c.Issuer = "someone-else" }), ErrInvalidToken},
		{"missing audience", sign(t, hs256, key, func(c *CustomClaims) { c.Audience = nil }), ErrInvalidToken},
		{"other audience", sign(t, hs256, key, func(c *CustomClaims) { c.Audience = jwt.ClaimStrings{"billing-api"} }), ErrInvalidToken},
		{"missing expiry", sign(t, hs256, key, func(c *CustomClaims) { c.ExpiresAt = nil }), ErrInvalidToken},
		{"expired", sign(t, hs256, key, func(c *CustomClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }), ErrExpiredToken},
		{"issued in the future", sign(t, hs256, key, func(c *CustomClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }), ErrInvalidToken},
		{"refresh token type", sign(t, hs256, key, func(c *CustomClaims) { c.TokenType = TokenTypeRefresh }), ErrInvalidToken},
		{"subject mismatch", sign(t, hs256, key, func(c *CustomClaims) { c.Subject = uuid.NewString() }), ErrInvalidToken},
		{"HS512 with the same secret", sign(t, jwt.SigningMethodHS512, key, nil), ErrInvalidToken},
		{"alg none", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, nil), ErrInvalidToken},
		{"garbage", "not.a.token", ErrInvalidToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateToken(tt.token, testJWTSecret); !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
		})
	}
}
