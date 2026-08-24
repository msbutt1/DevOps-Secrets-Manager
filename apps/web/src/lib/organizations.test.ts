import { canCreateVaults, pickDefaultOrganization } from './organizations';
import type { Organization } from '@/types/api';

const org = (id: string, vaultCount: number): Organization => ({
  id,
  name: id,
  role: 'developer',
  memberCount: 1,
  vaultCount,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
});

describe('pickDefaultOrganization', () => {
  it('prefers the organization with the most vaults, keeping the first on ties', () => {
    expect(pickDefaultOrganization([])).toBeNull();
    expect(pickDefaultOrganization([org('personal', 0), org('team', 4)])?.id).toBe('team');
    expect(pickDefaultOrganization([org('a', 2), org('b', 2)])?.id).toBe('a');
  });
});

describe('canCreateVaults', () => {
  it('allows owners, admins and developers', () => {
    expect(['owner', 'admin', 'developer'].every((r) => canCreateVaults(r as never))).toBe(true);
    expect(canCreateVaults('oncall')).toBe(false);
    expect(canCreateVaults('viewer')).toBe(false);
    expect(canCreateVaults(undefined)).toBe(false);
  });
});
