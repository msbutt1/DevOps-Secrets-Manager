// Package servicetokens issues and checks read-only tokens that let machines read the secrets
// of a single environment.
package servicetokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Prefix marks service tokens so they can be told apart from user JWTs.
const Prefix = "dsm_st_"

// MaxExpiryDays is the longest lifetime a token can be given.
const MaxExpiryDays = 365

var (
	// ErrInvalidToken covers unknown, revoked and expired tokens alike.
	ErrInvalidToken = errors.New("invalid, revoked or expired service token")
	// ErrNotFound means the token does not exist in that vault.
	ErrNotFound = errors.New("service token not found")
	// ErrInvalidName is returned for empty or overlong names.
	ErrInvalidName = errors.New("token names must be 1-100 characters")
	// ErrInvalidExpiry is returned for expiries outside 1-365 days.
	ErrInvalidExpiry = fmt.Errorf("expires_in_days must be between 1 and %d", MaxExpiryDays)
)

// Token is a service token's metadata; the plaintext is only available when it is created.
type Token struct {
	ID              uuid.UUID
	EnvironmentID   uuid.UUID
	EnvironmentName string
	VaultID         uuid.UUID
	VaultName       string
	OrganizationID  uuid.UUID
	Name            string
	Prefix          string
	CreatedByID     *uuid.UUID
	CreatedByName   string
	CreatedAt       time.Time
	ExpiresAt       *time.Time
	LastUsedAt      *time.Time
}

// Hash returns the stored form of a token.
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Service stores and checks service tokens. Permission checks happen in the HTTP layer.
type Service interface {
	// Create issues a token and returns it with the plaintext, which is never stored.
	Create(ctx context.Context, envID, createdBy uuid.UUID, name string, expiresInDays *int) (*Token, string, error)
	List(ctx context.Context, envID uuid.UUID) ([]*Token, error)
	Get(ctx context.Context, id uuid.UUID) (*Token, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	// Authenticate returns the token for a plaintext value if it is valid, and records its use.
	Authenticate(ctx context.Context, plaintext string) (*Token, error)
}

type service struct {
	pool *pgxpool.Pool
}

// NewService creates the service token service.
func NewService(pool *pgxpool.Pool) Service {
	return &service{pool: pool}
}

const tokenSelect = `
	SELECT t.id, t.environment_id, e.name, v.id, v.name, v.organization_id, t.name, t.token_prefix,
		t.created_by, COALESCE(u.name, ''), t.created_at, t.expires_at, t.last_used_at
	FROM service_tokens t
	JOIN environments e ON e.id = t.environment_id AND e.deleted_at IS NULL
	JOIN vaults v ON v.id = e.vault_id AND v.deleted_at IS NULL
	LEFT JOIN users u ON u.id = t.created_by
`

const activeToken = `t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > now())`

func scan(row pgx.Row) (*Token, error) {
	var t Token
	err := row.Scan(&t.ID, &t.EnvironmentID, &t.EnvironmentName, &t.VaultID, &t.VaultName, &t.OrganizationID,
		&t.Name, &t.Prefix, &t.CreatedByID, &t.CreatedByName, &t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt)
	return &t, err
}

func (s *service) Create(ctx context.Context, envID, createdBy uuid.UUID, name string, expiresInDays *int) (*Token, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, "", ErrInvalidName
	}
	var expiresAt *time.Time
	if expiresInDays != nil {
		if *expiresInDays < 1 || *expiresInDays > MaxExpiryDays {
			return nil, "", ErrInvalidExpiry
		}
		t := time.Now().Add(time.Duration(*expiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, "", err
	}
	plaintext := Prefix + hex.EncodeToString(random)
	prefix := plaintext[:len(Prefix)+6]

	var id uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO service_tokens (environment_id, name, token_prefix, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		envID, name, prefix, Hash(plaintext), createdBy, expiresAt).Scan(&id); err != nil {
		return nil, "", fmt.Errorf("failed to create service token: %w", err)
	}
	token, err := s.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return token, plaintext, nil
}

func (s *service) List(ctx context.Context, envID uuid.UUID) ([]*Token, error) {
	rows, err := s.pool.Query(ctx, tokenSelect+` WHERE t.environment_id = $1 AND `+activeToken+` ORDER BY t.created_at DESC`, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list service tokens: %w", err)
	}
	defer rows.Close()
	tokens := make([]*Token, 0)
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*Token, error) {
	t, err := scan(s.pool.QueryRow(ctx, tokenSelect+` WHERE t.id = $1 AND `+activeToken, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (s *service) Revoke(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE service_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("failed to revoke service token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *service) Authenticate(ctx context.Context, plaintext string) (*Token, error) {
	if !strings.HasPrefix(plaintext, Prefix) {
		return nil, ErrInvalidToken
	}
	// The lookup is by SHA-256 hash, so comparing the secret itself never happens in Go code.
	t, err := scan(s.pool.QueryRow(ctx, tokenSelect+` WHERE t.token_hash = $1 AND `+activeToken, Hash(plaintext)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE service_tokens SET last_used_at = now() WHERE id = $1`, t.ID); err != nil {
		return nil, fmt.Errorf("failed to record token use: %w", err)
	}
	return t, nil
}
