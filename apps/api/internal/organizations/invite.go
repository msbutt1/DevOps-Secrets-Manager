package organizations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/email"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/users"
)

// InviteTTL is how long an invitation link stays valid.
const InviteTTL = 7 * 24 * time.Hour

var (
	// ErrInviteNotFound covers unknown, expired, revoked and already used invitations alike.
	ErrInviteNotFound = errors.New("invitation not found or no longer valid")
	// ErrInviteEmailMismatch means the invitation was sent to a different address.
	ErrInviteEmailMismatch = errors.New("this invitation was sent to a different email address")
	// ErrAlreadyMember means the invited person is already in the organization.
	ErrAlreadyMember = errors.New("that person is already a member of this organization")
	// ErrInvalidEmail is returned for addresses that are obviously not email addresses.
	ErrInvalidEmail = errors.New("enter a valid email address")
)

// Invite is an open or past invitation to an organization.
type Invite struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	OrganizationName string
	Email            string
	Role             string
	InvitedByID      *uuid.UUID
	InvitedByName    string
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

// HashInviteToken returns the stored form of an invitation token.
func HashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const inviteSelect = `
	SELECT i.id, i.organization_id, o.name, i.email, i.role, i.invited_by, COALESCE(u.name, ''), i.created_at, i.expires_at
	FROM organization_invites i
	JOIN organizations o ON o.id = i.organization_id AND o.deleted_at IS NULL
	LEFT JOIN users u ON u.id = i.invited_by
`

const openInvite = `i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()`

func scanInvite(row pgx.Row) (*Invite, error) {
	var inv Invite
	if err := row.Scan(&inv.ID, &inv.OrganizationID, &inv.OrganizationName, &inv.Email, &inv.Role,
		&inv.InvitedByID, &inv.InvitedByName, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
		return nil, err
	}
	return &inv, nil
}

// InviteService creates, lists, revokes and accepts invitations.
type InviteService interface {
	// Create stores an invitation and emails the link. emailSent is false when the invitation was
	// created but delivery failed (for example no SMTP in production); it can be sent again.
	Create(ctx context.Context, callerID, orgID uuid.UUID, email, role string) (invite *Invite, emailSent bool, err error)
	ListOpen(ctx context.Context, callerID, orgID uuid.UUID) ([]*Invite, error)
	Revoke(ctx context.Context, callerID, orgID, inviteID uuid.UUID) error
	// Lookup describes an open invitation for the accept page, without needing to log in.
	Lookup(ctx context.Context, token string) (*Invite, bool, error)
	// Accept adds the logged-in caller to the organization.
	Accept(ctx context.Context, callerID uuid.UUID, token string) (*Organization, error)
	// ClaimForNewUser runs inside the registration transaction: it checks the invitation is open
	// and addressed to email, adds the new user to the organization and marks it accepted.
	ClaimForNewUser(ctx context.Context, tx storage.Querier, token, email string, userID uuid.UUID) (*Invite, error)
	// RecordAccepted audits an invitation claimed during registration, after the transaction commits.
	RecordAccepted(ctx context.Context, invite *Invite, userID uuid.UUID)
}

type inviteService struct {
	db     storage.Querier
	orgs   Repository
	mail   email.EmailService
	audit  audit.AuditService
	logger *slog.Logger
}

// NewInviteService creates the invitation service.
func NewInviteService(db storage.Querier, orgs Repository, mail email.EmailService, auditService audit.AuditService, logger *slog.Logger) InviteService {
	return &inviteService{db: db, orgs: orgs, mail: mail, audit: auditService, logger: logger}
}

func (s *inviteService) Create(ctx context.Context, callerID, orgID uuid.UUID, address, role string) (*Invite, bool, error) {
	address = users.NormalizeEmail(address)
	if len(address) < 3 || len(address) > 255 || !strings.Contains(address, "@") || strings.ContainsAny(address, " \r\n") {
		return nil, false, ErrInvalidEmail
	}
	org, err := s.orgs.GetForUser(ctx, orgID, callerID)
	if err != nil {
		return nil, false, err
	}
	if !policy.OrgRoleAllows(org.Role, policy.ActionOrgManage) || (role == policy.RoleOwner && org.Role != policy.RoleOwner) {
		return nil, false, ErrForbidden
	}

	var exists bool
	if err := s.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM user_organizations uo JOIN users u ON u.id = uo.user_id
		               WHERE uo.organization_id = $1 AND u.email = $2)`, orgID, address).Scan(&exists); err != nil {
		return nil, false, fmt.Errorf("failed to check membership: %w", err)
	}
	if exists {
		return nil, false, ErrAlreadyMember
	}

	token, err := newInviteToken()
	if err != nil {
		return nil, false, err
	}
	// Re-inviting replaces the previous open invitation, so the old link stops working.
	if _, err := s.db.Exec(ctx, `
		UPDATE organization_invites SET revoked_at = now()
		WHERE organization_id = $1 AND email = $2 AND accepted_at IS NULL AND revoked_at IS NULL`, orgID, address); err != nil {
		return nil, false, fmt.Errorf("failed to replace previous invitation: %w", err)
	}
	var id uuid.UUID
	err = s.db.QueryRow(ctx, `
		INSERT INTO organization_invites (organization_id, email, role, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orgID, address, role, HashInviteToken(token), callerID, time.Now().Add(InviteTTL)).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, false, fmt.Errorf("an invitation was created at the same time; try again: %w", err)
		}
		return nil, false, fmt.Errorf("failed to create invitation: %w", err)
	}

	inv, err := scanInvite(s.db.QueryRow(ctx, inviteSelect+` WHERE i.id = $1`, id))
	if err != nil {
		return nil, false, err
	}

	emailSent := true
	if err := s.mail.SendInviteEmail(ctx, email.Invite{
		To: address, OrganizationName: org.Name, InviterName: inv.InvitedByName, Role: role,
		Token: token, ExpiresAt: inv.ExpiresAt,
	}); err != nil {
		// The invitation exists and can be re-sent; the caller is told delivery failed.
		emailSent = false
		s.logger.ErrorContext(ctx, "failed to send invitation email", slog.String("invite_id", inv.ID.String()), slog.Any("error", err))
	}

	_ = s.audit.Record(ctx, audit.Event{
		UserID: callerID, Action: audit.ActionInviteCreated,
		TargetType: "invite", TargetID: &inv.ID, TargetName: address,
		OrganizationID: &orgID, Metadata: map[string]interface{}{"role": role},
	})
	return inv, emailSent, nil
}

func (s *inviteService) ListOpen(ctx context.Context, callerID, orgID uuid.UUID) ([]*Invite, error) {
	org, err := s.orgs.GetForUser(ctx, orgID, callerID)
	if err != nil {
		return nil, err
	}
	if !policy.OrgRoleAllows(org.Role, policy.ActionOrgManage) {
		return nil, ErrForbidden
	}
	rows, err := s.db.Query(ctx, inviteSelect+` WHERE i.organization_id = $1 AND `+openInvite+` ORDER BY i.created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations: %w", err)
	}
	defer rows.Close()
	invites := make([]*Invite, 0)
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (s *inviteService) Revoke(ctx context.Context, callerID, orgID, inviteID uuid.UUID) error {
	org, err := s.orgs.GetForUser(ctx, orgID, callerID)
	if err != nil {
		return err
	}
	if !policy.OrgRoleAllows(org.Role, policy.ActionOrgManage) {
		return ErrForbidden
	}
	var address string
	err = s.db.QueryRow(ctx, `
		UPDATE organization_invites i SET revoked_at = now()
		WHERE i.id = $1 AND i.organization_id = $2 AND `+openInvite+`
		RETURNING i.email`, inviteID, orgID).Scan(&address)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInviteNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to revoke invitation: %w", err)
	}
	_ = s.audit.Record(ctx, audit.Event{
		UserID: callerID, Action: audit.ActionInviteRevoked,
		TargetType: "invite", TargetID: &inviteID, TargetName: address, OrganizationID: &orgID,
	})
	return nil
}

func (s *inviteService) findOpen(ctx context.Context, q storage.Querier, token string, lock bool) (*Invite, error) {
	query := inviteSelect + ` WHERE i.token_hash = $1 AND ` + openInvite
	if lock {
		query += ` FOR UPDATE OF i`
	}
	inv, err := scanInvite(q.QueryRow(ctx, query, HashInviteToken(strings.TrimSpace(token))))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	return inv, err
}

func (s *inviteService) Lookup(ctx context.Context, token string) (*Invite, bool, error) {
	inv, err := s.findOpen(ctx, s.db, token, false)
	if err != nil {
		return nil, false, err
	}
	var accountExists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`, inv.Email).Scan(&accountExists); err != nil {
		return nil, false, err
	}
	return inv, accountExists, nil
}

// claim adds the user to the invite's organization and marks the invite accepted.
func (s *inviteService) claim(ctx context.Context, q storage.Querier, inv *Invite, userID uuid.UUID) error {
	tag, err := q.Exec(ctx, `
		INSERT INTO user_organizations (user_id, organization_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, organization_id) DO NOTHING`, userID, inv.OrganizationID, inv.Role)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyMember
	}
	if _, err := q.Exec(ctx, `UPDATE organization_invites SET accepted_at = now(), accepted_by = $2 WHERE id = $1`, inv.ID, userID); err != nil {
		return fmt.Errorf("failed to mark invitation accepted: %w", err)
	}
	return nil
}

func (s *inviteService) RecordAccepted(ctx context.Context, inv *Invite, userID uuid.UUID) {
	_ = s.audit.Record(ctx, audit.Event{
		UserID: userID, Action: audit.ActionInviteAccepted,
		TargetType: "invite", TargetID: &inv.ID, TargetName: inv.Email,
		OrganizationID: &inv.OrganizationID, Metadata: map[string]interface{}{"role": inv.Role},
	})
}

func (s *inviteService) Accept(ctx context.Context, callerID uuid.UUID, token string) (*Organization, error) {
	pool, ok := s.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, errors.New("invite service needs a database that supports transactions")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	inv, err := s.findOpen(ctx, tx, token, true)
	if err != nil {
		return nil, err
	}
	var callerEmail string
	if err := tx.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, callerID).Scan(&callerEmail); err != nil {
		return nil, err
	}
	if users.NormalizeEmail(callerEmail) != inv.Email {
		return nil, ErrInviteEmailMismatch
	}
	if err := s.claim(ctx, tx, inv, callerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.RecordAccepted(ctx, inv, callerID)
	return s.orgs.GetForUser(ctx, inv.OrganizationID, callerID)
}

func (s *inviteService) ClaimForNewUser(ctx context.Context, tx storage.Querier, token, address string, userID uuid.UUID) (*Invite, error) {
	inv, err := s.findOpen(ctx, tx, token, true)
	if err != nil {
		return nil, err
	}
	if users.NormalizeEmail(address) != inv.Email {
		return nil, ErrInviteEmailMismatch
	}
	if err := s.claim(ctx, tx, inv, userID); err != nil {
		return nil, err
	}
	return inv, nil
}
