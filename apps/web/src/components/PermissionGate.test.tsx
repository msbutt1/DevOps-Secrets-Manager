import { render, renderHook, screen } from '@testing-library/react';
import { PermissionGate, usePermission } from './PermissionGate';
import { ROLE_PERMISSIONS, type VaultRole } from '@/types/api';

const roles: VaultRole[] = ['owner', 'admin', 'developer', 'oncall', 'viewer'];

describe('PermissionGate', () => {
  it.each(roles)('shows reveal controls only to roles that can reveal (%s)', (role) => {
    render(
      <PermissionGate permission="canReveal" userRole={role}>
        <button>Reveal</button>
      </PermissionGate>,
    );
    const button = screen.queryByRole('button', { name: 'Reveal' });
    if (ROLE_PERMISSIONS[role].canReveal) {
      expect(button).toBeInTheDocument();
    } else {
      expect(button).not.toBeInTheDocument();
    }
  });

  it('denies by default when the role is unknown', () => {
    render(
      <PermissionGate permission="canRead" userRole={undefined} fallback={<span>No access</span>}>
        <span>Secret list</span>
      </PermissionGate>,
    );
    expect(screen.queryByText('Secret list')).not.toBeInTheDocument();
    expect(screen.getByText('No access')).toBeInTheDocument();
  });

  it('renders a disabled copy instead of hiding when asked', () => {
    const { container } = render(
      <PermissionGate permission="canDelete" userRole="admin" showDisabled>
        <button>Delete vault</button>
      </PermissionGate>,
    );
    expect(screen.getByRole('button', { name: 'Delete vault' })).toBeInTheDocument();
    expect(container.firstChild).toHaveClass('pointer-events-none');
  });

  it('restricts to explicit roles', () => {
    render(
      <PermissionGate roles={['owner']} userRole="admin">
        <span>Owners only</span>
      </PermissionGate>,
    );
    expect(screen.queryByText('Owners only')).not.toBeInTheDocument();
  });
});

describe('usePermission', () => {
  it('matches the shared matrix for every role', () => {
    for (const role of roles) {
      const { result } = renderHook(() => usePermission(role));
      expect(result.current.canReveal).toBe(ROLE_PERMISSIONS[role].canReveal);
      expect(result.current.canEdit).toBe(ROLE_PERMISSIONS[role].canWrite);
      expect(result.current.canDelete).toBe(ROLE_PERMISSIONS[role].canDelete);
      expect(result.current.canManageMembers).toBe(ROLE_PERMISSIONS[role].canManageMembers);
    }
  });
});
