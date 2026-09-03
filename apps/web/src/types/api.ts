import type { Camelize } from './camelize';
import type { components } from './openapi.gen';

/**
 * Request and response types come from docs/openapi.yaml (generated into openapi.gen.ts with
 * `npm run generate:api-types`) with keys camelCased the way api-client transforms them, so the
 * web app cannot drift from the API contract without failing the type check.
 */
type Schema<Name extends keyof components['schemas']> = Camelize<components['schemas'][Name]>;

// Authentication DTOs
export type LoginRequest = Schema<'LoginRequest'>;
export type RegisterRequest = Schema<'RegisterRequest'>;
export type RegisterResponse = Schema<'RegisterResponse'>;
export type VerifyEmailRequest = Schema<'VerifyEmailRequest'>;
export type VerifyEmailResponse = Schema<'MessageResponse'>;
export type ChangePasswordRequest = Schema<'ChangePasswordRequest'>;
export type ChangePasswordResponse = Schema<'MessageResponse'>;
export type AuthTokens = Schema<'AuthTokens'>;
export type User = Schema<'UserProfile'>;
export type UserOrganization = Schema<'UserOrganization'>;
export type OrganizationRole = Schema<'Role'>;

// Organization DTOs
export type Organization = Schema<'Organization'>;
export type OrganizationMember = Schema<'OrganizationMember'>;
export type UpdateOrganizationRequest = Schema<'UpdateOrganizationRequest'>;
export type Invite = Schema<'Invite'>;
export type CreateInviteRequest = Schema<'CreateInviteRequest'>;
export type CreateInviteResponse = Schema<'CreateInviteResponse'>;
export type InviteLookup = Schema<'InviteLookup'>;

// Vault DTOs
export type Vault = Schema<'Vault'>;
export type VaultCreateRequest = Schema<'CreateVaultRequest'>;
export type VaultUpdateRequest = Schema<'UpdateVaultRequest'>;

// Environment DTOs
export type Environment = Schema<'Environment'>;
/** Any name matching ENVIRONMENT_NAME_PATTERN in src/lib/environments.ts */
export type EnvironmentName = string;
export type EnvironmentCreateRequest = Schema<'EnvironmentRequest'>;
export type EnvironmentUpdateRequest = Schema<'EnvironmentRequest'>;

// Secret DTOs
export type Secret = Schema<'SecretMetadata'>;
export type SecretCreateRequest = Schema<'CreateSecretRequest'>;
export type SecretUpdateRequest = Schema<'UpdateSecretRequest'>;
export type SecretRevealResponse = Schema<'SecretReveal'>;
export type RotationPolicy = Schema<'RotationPolicy'>;

// Service token DTOs
export type ServiceToken = Schema<'ServiceToken'>;
export type Session = Schema<'Session'>;
export type SecretVersion = Schema<'SecretVersion'>;
export type CreatedServiceToken = Schema<'CreatedServiceToken'>;
export type CreateServiceTokenRequest = Schema<'CreateServiceTokenRequest'>;

// Access Control DTOs
export type VaultRole = Schema<'Role'>;
export type VaultMember = Schema<'Member'>;
export type VaultPermissions = Schema<'MemberPermissions'>;
export type AddMemberRequest = Schema<'AddMemberRequest'>;
export type UpdateMemberRequest = Schema<'UpdateMemberRequest'>;

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

// Audit DTOs
/** Every action the API records, with display labels. Kept identical to audit.Actions in the API. */
export const AUDIT_ACTIONS = [
  { value: 'login.success', label: 'Login Success' },
  { value: 'login.failure', label: 'Login Failure' },
  { value: 'login.locked', label: 'Login Locked' },
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
  { value: 'vault.key_rotated', label: 'Vault Key Rotated' },
  { value: 'env.created', label: 'Environment Created' },
  { value: 'env.updated', label: 'Environment Updated' },
  { value: 'env.deleted', label: 'Environment Deleted' },
  { value: 'org.updated', label: 'Organization Renamed' },
  { value: 'org.member_role_changed', label: 'Organization Role Changed' },
  { value: 'org.member_removed', label: 'Organization Member Removed' },
  { value: 'invite.created', label: 'Invitation Sent' },
  { value: 'invite.revoked', label: 'Invitation Revoked' },
  { value: 'invite.accepted', label: 'Invitation Accepted' },
  { value: 'token.created', label: 'Service Token Created' },
  { value: 'token.revoked', label: 'Service Token Revoked' },
  { value: 'env.exported', label: 'Environment Exported' },
  { value: 'user.password_changed', label: 'Password Changed' },
  { value: 'user.sessions_revoked', label: 'Sessions Signed Out' },
  { value: 'user.password_reset', label: 'Password Reset' },
  { value: 'secret.restored', label: 'Secret Restored' },
] as const;

export type AuditAction = (typeof AUDIT_ACTIONS)[number]['value'];

// AUDIT_ACTIONS must list exactly the actions in the OpenAPI AuditAction enum.
type SameMembers<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
const auditActionsMatchSpec: SameMembers<AuditAction, Schema<'AuditAction'>> = true;
void auditActionsMatchSpec;

export type AuditEvent = Schema<'AuditEvent'>;
export type AuditEventPage = Schema<'AuditEventPage'>;

export interface AuditFilters {
  organizationId?: string;
  vaultId?: string;
  environmentId?: string;
  userId?: string;
  /** Case-insensitive match on part of the user's email */
  userEmail?: string;
  action?: AuditAction;
  /** Leave out events with these actions */
  excludeAction?: AuditAction[];
  /** RFC 3339 timestamp or YYYY-MM-DD */
  startDate?: string;
  /** RFC 3339 timestamp or YYYY-MM-DD (the whole day is included) */
  endDate?: string;
  page?: number;
  limit?: number;
}

// Dashboard DTOs
export type DashboardStats = Schema<'DashboardStats'>;
export type DashboardAlert = Schema<'DashboardAlert'>;
export type DashboardAlertType = DashboardAlert['type'];
export type HealthStatus = Schema<'Health'>;

// API Error
export type ApiError = Schema<'Error'>;
