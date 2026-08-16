// Authentication DTOs
export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name: string;
}

export interface RegisterResponse {
  userId: string;
  message: string;
}

export interface VerifyEmailRequest {
  token: string;
}

export interface VerifyEmailResponse {
  message: string;
}

export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}

export interface ChangePasswordResponse {
  message: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface User {
  id: string;
  email: string;
  name: string;
  createdAt: string;
  organizations?: UserOrganization[];
}

export interface UserOrganization {
  id: string;
  name: string;
  role: OrganizationRole;
}

export type OrganizationRole = 'owner' | 'admin' | 'developer' | 'oncall' | 'viewer';

// Organization DTOs
export interface Organization {
  id: string;
  name: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
}

// Vault DTOs
export interface Vault {
  id: string;
  name: string;
  description?: string;
  organizationId: string;
  organizationName: string;
  createdAt: string;
  updatedAt: string;
  /** Creator's display name; empty when unknown */
  createdBy: string;
  createdById: string | null;
  userRole: VaultRole;
  secretCount?: number;
  envCount?: number;
}

export interface VaultCreateRequest {
  name: string;
  description?: string;
  organizationId?: string;
}

export interface VaultUpdateRequest {
  name?: string;
  description?: string;
}

export interface Environment {
  id: string;
  vaultId: string;
  name: EnvironmentName;
  description?: string;
  createdAt: string;
  updatedAt?: string;
  secretCount?: number;
}

export type EnvironmentName = 'dev' | 'staging' | 'prod';

export interface EnvironmentCreateRequest {
  name: EnvironmentName;
  description?: string;
}

export interface EnvironmentUpdateRequest {
  name?: EnvironmentName;
  description?: string;
}

// Secret DTOs
export interface Secret {
  id: string;
  environmentId: string;
  keyName: string;
  description?: string;
  lastUpdatedAt?: string;
  updatedAt?: string;
  /** Display name of the user who last changed the secret */
  lastUpdatedBy: string;
  lastUpdatedById: string | null;
  createdAt?: string;
  createdBy?: string;
  rotationPolicy: RotationPolicy | null;
  expiresAt: string | null;
  metadata?: Record<string, string>;
}

export interface SecretCreateRequest {
  keyName: string;
  value: string;
  description?: string;
  rotationIntervalDays?: number;
  expiresAt?: string;
  metadata?: Record<string, string>;
}

export interface SecretUpdateRequest {
  value?: string;
  description?: string;
  rotationIntervalDays?: number;
  expiresAt?: string;
  metadata?: Record<string, string>;
}

export interface SecretRevealResponse {
  value: string;
  expiresIn: number; // seconds until auto-hide
}

export type RotationPolicy = {
  intervalDays: number;
  /** When the value was last changed; null if it never has been since creation */
  lastRotatedAt: string | null;
  nextRotationAt: string;
};

// Access Control DTOs
export type VaultRole = 'owner' | 'admin' | 'developer' | 'oncall' | 'viewer';

export interface VaultMember {
  userId: string;
  email: string;
  name: string;
  role: VaultRole;
  permissions: VaultPermissions;
  addedAt: string;
  addedBy: string;
}

export interface VaultPermissions {
  canRead: boolean;
  canWrite: boolean;
  canReveal: boolean;
  canManageMembers: boolean;
  canDelete: boolean;
}

export const ROLE_PERMISSIONS: Record<VaultRole, VaultPermissions> = {
  owner: {
    canRead: true,
    canWrite: true,
    canReveal: true,
    canManageMembers: true,
    canDelete: true,
  },
  admin: {
    canRead: true,
    canWrite: true,
    canReveal: true,
    canManageMembers: true,
    canDelete: false,
  },
  developer: {
    canRead: true,
    canWrite: true,
    canReveal: false,
    canManageMembers: false,
    canDelete: false,
  },
  oncall: {
    canRead: true,
    canWrite: false,
    canReveal: true,
    canManageMembers: false,
    canDelete: false,
  },
  viewer: {
    canRead: true,
    canWrite: false,
    canReveal: false,
    canManageMembers: false,
    canDelete: false,
  },
};

export interface AddMemberRequest {
  email: string;
  role: VaultRole;
}

export interface UpdateMemberRequest {
  role: VaultRole;
}

// Audit DTOs
/** Every action the API records, with display labels. Kept identical to audit.Actions in the API. */
export const AUDIT_ACTIONS = [
  { value: 'login.success', label: 'Login Success' },
  { value: 'login.failure', label: 'Login Failure' },
  { value: 'secret.created', label: 'Secret Created' },
  { value: 'secret.updated', label: 'Secret Updated' },
  { value: 'secret.deleted', label: 'Secret Deleted' },
  { value: 'secret.revealed', label: 'Secret Revealed' },
  { value: 'member.added', label: 'Member Added' },
  { value: 'member.removed', label: 'Member Removed' },
  { value: 'member.role_changed', label: 'Role Changed' },
  { value: 'vault.created', label: 'Vault Created' },
  { value: 'vault.updated', label: 'Vault Updated' },
  { value: 'vault.deleted', label: 'Vault Deleted' },
  { value: 'env.created', label: 'Environment Created' },
  { value: 'env.updated', label: 'Environment Updated' },
  { value: 'env.deleted', label: 'Environment Deleted' },
] as const;

export type AuditAction = (typeof AUDIT_ACTIONS)[number]['value'];

export interface AuditEvent {
  id: string;
  timestamp: string;
  action: AuditAction;
  userId: string;
  userEmail: string;
  organizationId: string | null;
  vaultId: string | null;
  vaultName: string | null;
  environmentId: string | null;
  environmentName: string | null;
  targetType: string;
  targetId: string | null;
  targetName: string | null;
  ipAddress: string | null;
  userAgent: string | null;
  metadata: Record<string, unknown>;
}

export interface AuditFilters {
  organizationId?: string;
  vaultId?: string;
  environmentId?: string;
  userId?: string;
  /** Case-insensitive match on part of the user's email */
  userEmail?: string;
  action?: AuditAction;
  /** RFC 3339 timestamp or YYYY-MM-DD */
  startDate?: string;
  /** RFC 3339 timestamp or YYYY-MM-DD (the whole day is included) */
  endDate?: string;
  page?: number;
  limit?: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
}

// API Error
export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, string>;
}
