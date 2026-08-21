import { ReactNode } from 'react';
import { ROLE_PERMISSIONS } from '@/types/api';
import type { VaultRole, VaultPermissions } from '@/types/api';

type PermissionKey = keyof VaultPermissions;

interface PermissionGateProps {
  children: ReactNode;
  /** Required permission to show children */
  permission?: PermissionKey;
  /** Required roles (any of these) */
  roles?: VaultRole[];
  /** Current user's role in the vault */
  userRole?: VaultRole;
  /** Fallback content when permission denied */
  fallback?: ReactNode;
  /** Whether to show disabled version instead of hiding */
  showDisabled?: boolean;
}

// The matrix is shared with the API (a Go test compares them)
const rolePermissions = ROLE_PERMISSIONS;

export const PermissionGate = ({
  children,
  permission,
  roles,
  userRole,
  fallback = null,
  showDisabled = false,
}: PermissionGateProps) => {
  // If no role provided, deny by default
  if (!userRole) {
    return showDisabled ? (
      <div className="opacity-50 pointer-events-none">{children}</div>
    ) : (
      <>{fallback}</>
    );
  }

  // Check role-based access
  if (roles && roles.length > 0) {
    if (!roles.includes(userRole)) {
      return showDisabled ? (
        <div className="opacity-50 pointer-events-none">{children}</div>
      ) : (
        <>{fallback}</>
      );
    }
  }

  // Check permission-based access
  if (permission) {
    const permissions = rolePermissions[userRole];
    if (!permissions[permission]) {
      return showDisabled ? (
        <div className="opacity-50 pointer-events-none">{children}</div>
      ) : (
        <>{fallback}</>
      );
    }
  }

  return <>{children}</>;
};

// Hook for checking permissions programmatically
export const usePermission = (userRole?: VaultRole) => {
  const hasPermission = (permission: PermissionKey): boolean => {
    if (!userRole) return false;
    return rolePermissions[userRole][permission];
  };

  const hasRole = (allowedRoles: VaultRole[]): boolean => {
    if (!userRole) return false;
    return allowedRoles.includes(userRole);
  };

  const canCreate = userRole ? rolePermissions[userRole].canWrite : false;
  const canEdit = userRole ? rolePermissions[userRole].canWrite : false;
  const canDelete = userRole ? rolePermissions[userRole].canDelete : false;
  const canReveal = userRole ? rolePermissions[userRole].canReveal : false;
  const canManageMembers = userRole ? rolePermissions[userRole].canManageMembers : false;

  return {
    hasPermission,
    hasRole,
    canCreate,
    canEdit,
    canDelete,
    canReveal,
    canManageMembers,
  };
};
