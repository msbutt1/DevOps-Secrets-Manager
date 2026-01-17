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
  createdBy: string;
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
  lastUpdatedAt: string;
  lastUpdatedBy: string;
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
  lastRotatedAt: string;
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
  owner: { canRead: true, canWrite: true, canReveal: true, canManageMembers: true, canDelete: true },
  admin: { canRead: true, canWrite: true, canReveal: true, canManageMembers: true, canDelete: false },
  developer: { canRead: true, canWrite: true, canReveal: false, canManageMembers: false, canDelete: false },
  oncall: { canRead: true, canWrite: false, canReveal: true, canManageMembers: false, canDelete: false },
  viewer: { canRead: true, canWrite: false, canReveal: false, canManageMembers: false, canDelete: false },
};

export interface AddMemberRequest {
  email: string;
  role: VaultRole;
}

export interface UpdateMemberRequest {
  role: VaultRole;
}

// Audit DTOs
export type AuditAction = 
  | 'login.success'
  | 'login.failure'
  | 'secret.created'
  | 'secret.updated'
  | 'secret.deleted'
  | 'secret.revealed'
  | 'member.added'
  | 'member.removed'
  | 'member.role_changed'
  | 'vault.created'
  | 'vault.updated'
  | 'vault.deleted'
  | 'env.created'
  | 'env.deleted'
  | 'token.refreshed';

export interface AuditEvent {
  id: string;
  vaultId: string;
  vaultName: string;
  environmentId: string | null;
  environmentName: string | null;
  userId: string;
  userEmail: string;
  action: AuditAction;
  targetId: string | null;
  targetName: string | null;
  metadata: Record<string, unknown>;
  ipAddress: string;
  userAgent: string;
  timestamp: string;
}

export interface AuditFilters {
  vaultId?: string;
  environmentId?: string;
  userId?: string;
  action?: AuditAction;
  startDate?: string;
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
