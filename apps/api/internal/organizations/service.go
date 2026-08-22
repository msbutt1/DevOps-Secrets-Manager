package organizations

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/policy"
)

// Service applies the organization rules: members can see their organizations and colleagues;
// owners and admins can rename the organization and manage members; only owners can grant,
// change or remove the owner role; an organization always keeps an owner.
type Service interface {
	List(ctx context.Context, callerID uuid.UUID) ([]*Organization, error)
	Get(ctx context.Context, callerID, orgID uuid.UUID) (*Organization, error)
	Rename(ctx context.Context, callerID, orgID uuid.UUID, name string) (*Organization, error)
	ListMembers(ctx context.Context, callerID, orgID uuid.UUID) ([]*Member, error)
	ChangeMemberRole(ctx context.Context, callerID, orgID, userID uuid.UUID, role string) (*Member, error)
	RemoveMember(ctx context.Context, callerID, orgID, userID uuid.UUID) error
}

type service struct {
	repo  Repository
	audit audit.AuditService
}

// NewService creates the organization service
func NewService(repo Repository, auditService audit.AuditService) Service {
	return &service{repo: repo, audit: auditService}
}

func (s *service) List(ctx context.Context, callerID uuid.UUID) ([]*Organization, error) {
	return s.repo.ListForUser(ctx, callerID)
}

func (s *service) Get(ctx context.Context, callerID, orgID uuid.UUID) (*Organization, error) {
	return s.repo.GetForUser(ctx, orgID, callerID)
}

// manager returns the caller's organization when they are an owner or admin.
func (s *service) manager(ctx context.Context, callerID, orgID uuid.UUID) (*Organization, error) {
	org, err := s.repo.GetForUser(ctx, orgID, callerID)
	if err != nil {
		return nil, err
	}
	if !policy.OrgRoleAllows(org.Role, policy.ActionOrgManage) {
		return nil, ErrForbidden
	}
	return org, nil
}

func (s *service) Rename(ctx context.Context, callerID, orgID uuid.UUID, name string) (*Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, ErrInvalidName
	}
	org, err := s.manager(ctx, callerID, orgID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Rename(ctx, orgID, name); err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		UserID: callerID, Action: audit.ActionOrgUpdated,
		TargetType: "organization", TargetID: &orgID, TargetName: name,
		OrganizationID: &orgID, Metadata: map[string]interface{}{"old_name": org.Name},
	})
	return s.repo.GetForUser(ctx, orgID, callerID)
}

func (s *service) ListMembers(ctx context.Context, callerID, orgID uuid.UUID) ([]*Member, error) {
	if _, err := s.repo.GetForUser(ctx, orgID, callerID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, orgID)
}

func (s *service) ChangeMemberRole(ctx context.Context, callerID, orgID, userID uuid.UUID, role string) (*Member, error) {
	org, err := s.manager(ctx, callerID, orgID)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if target.Role == role {
		return target, nil
	}
	if (role == policy.RoleOwner || target.Role == policy.RoleOwner) && org.Role != policy.RoleOwner {
		return nil, ErrForbidden
	}
	if target.Role == policy.RoleOwner {
		if owners, err := s.repo.CountOwners(ctx, orgID); err != nil {
			return nil, err
		} else if owners <= 1 {
			return nil, ErrLastOwner
		}
	}
	if err := s.repo.UpdateMemberRole(ctx, orgID, userID, role); err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		UserID: callerID, Action: audit.ActionOrgMemberRoleChanged,
		TargetType: "member", TargetID: &userID, TargetName: target.Email,
		OrganizationID: &orgID, Metadata: map[string]interface{}{"old_role": target.Role, "new_role": role},
	})
	return s.repo.GetMember(ctx, orgID, userID)
}

func (s *service) RemoveMember(ctx context.Context, callerID, orgID, userID uuid.UUID) error {
	org, err := s.manager(ctx, callerID, orgID)
	if err != nil {
		return err
	}
	if userID == callerID {
		return ErrCannotRemoveSelf
	}
	target, err := s.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if target.Role == policy.RoleOwner {
		if org.Role != policy.RoleOwner {
			return ErrForbidden
		}
		if owners, err := s.repo.CountOwners(ctx, orgID); err != nil {
			return err
		} else if owners <= 1 {
			return ErrLastOwner
		}
	}
	if err := s.repo.RemoveMember(ctx, orgID, userID); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Event{
		UserID: callerID, Action: audit.ActionOrgMemberRemoved,
		TargetType: "member", TargetID: &userID, TargetName: target.Email,
		OrganizationID: &orgID, Metadata: map[string]interface{}{"role": target.Role},
	})
	return nil
}
