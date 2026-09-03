import { describe, expect, it } from 'vitest';
import { expiryStatus } from './expiry';

const now = Date.parse('2026-09-15T12:00:00Z');

describe('expiryStatus', () => {
  it.each([
    [null, 'none', 'No expiry'],
    ['2026-09-15T11:00:00Z', 'expired', 'Expired today'],
    ['2026-09-10T12:00:00Z', 'expired', 'Expired 5d ago'],
    ['2026-09-15T18:00:00Z', 'soon', 'Expires today'],
    ['2026-09-20T12:00:01Z', 'soon', 'Expires in 5d'],
    ['2026-10-15T12:00:00Z', 'ok', 'Expires in 30d'],
  ])('%s', (expiresAt, state, label) => {
    expect(expiryStatus(expiresAt, now)).toMatchObject({ state, label });
  });
});
