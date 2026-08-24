import type { Organization, OrganizationRole } from '@/types/api';

/** Defaults to the organization with the most vaults, which is usually the team rather than a personal one. */
export const pickDefaultOrganization = (organizations: Organization[]): Organization | null =>
  organizations.reduce<Organization | null>(
    (best, org) => (!best || org.vaultCount > best.vaultCount ? org : best),
    null,
  );

/** Organization roles that may create vaults (mirrors policy.ActionOrgVaultCreate in the API). */
export const canCreateVaults = (role: OrganizationRole | undefined) =>
  role === 'owner' || role === 'admin' || role === 'developer';
