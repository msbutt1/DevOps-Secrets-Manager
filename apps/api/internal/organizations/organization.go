// Package organizations manages teams: their names, members and member roles.
package organizations

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound means the organization does not exist or the caller is not a member.
	ErrNotFound = errors.New("organization not found")
	// ErrMemberNotFound means the user is not a member of the organization.
	ErrMemberNotFound = errors.New("member not found")
	// ErrForbidden means the caller's organization role does not allow the change.
	ErrForbidden = errors.New("forbidden")
	// ErrLastOwner protects organizations from losing their last owner.
	ErrLastOwner = errors.New("an organization must keep at least one owner")
	// ErrInvalidName is returned for empty or overlong names.
	ErrInvalidName = errors.New("organization names must be 1-100 characters")
	// ErrCannotRemoveSelf stops members from removing themselves.
	ErrCannotRemoveSelf = errors.New("you cannot remove yourself from the organization")
)

// Organization is a team with the caller's role in it.
type Organization struct {
	ID          uuid.UUID
	Name        string
	Role        string
	MemberCount int
	VaultCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Member is a person in an organization.
type Member struct {
	UserID      uuid.UUID
	Email       string
	Name        string
	Role        string
	JoinedAt    time.Time
	LastLoginAt *time.Time
}

// Repository is the data access for organizations.
type Repository interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*Organization, error)
	GetForUser(ctx context.Context, orgID, userID uuid.UUID) (*Organization, error)
	Rename(ctx context.Context, orgID uuid.UUID, name string) error
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*Member, error)
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error)
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error
	// RemoveMember deletes the membership and the user's memberships of the organization's vaults.
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
}
