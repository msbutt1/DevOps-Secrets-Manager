import { toCamelCase } from '@/lib/case-transform';
import type { Camelize } from './camelize';
import type { components } from './openapi.gen';

type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
const assertType = <T extends true>(value: T) => value;

describe('Camelize', () => {
  it('matches the runtime key transform', () => {
    type Raw = components['schemas']['Member'];
    const raw: Raw = {
      user_id: 'u1',
      email: 'a@example.test',
      name: 'A',
      role: 'viewer',
      permissions: {
        can_read: true,
        can_write: false,
        can_reveal: false,
        can_manage_members: false,
        can_delete: false,
      },
      added_at: '2026-01-01T00:00:00Z',
      added_by: '',
      added_by_id: null,
    };
    const camel = toCamelCase<Camelize<Raw>>(raw);
    expect(camel.permissions.canManageMembers).toBe(false);
    expect(camel.addedById).toBeNull();

    assertType<
      Equal<
        Camelize<{ key_name: string; nested: { can_read: boolean }[] }>,
        { keyName: string; nested: { canRead: boolean }[] }
      >
    >(true);
  });
});
