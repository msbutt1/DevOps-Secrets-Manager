import { describe, expect, it } from 'vitest';
import { rotationStatus } from './rotation';

const now = Date.parse('2026-09-15T12:00:00Z');

describe('rotationStatus', () => {
  it.each([
    ['2026-09-03T12:00:00Z', 'overdue', 'Overdue 12d'],
    ['2026-09-15T08:00:00Z', 'overdue', 'Due today'],
    ['2026-09-18T13:00:00Z', 'due', 'Due in 3d'],
    ['2026-11-01T12:00:00Z', 'ok', 'Due in 47d'],
  ])('%s', (next, state, label) => {
    expect(rotationStatus(next, now)).toEqual({ state, label });
  });
});
