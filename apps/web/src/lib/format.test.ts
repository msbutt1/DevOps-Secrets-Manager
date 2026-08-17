import { formatUptime } from './format';

describe('formatUptime', () => {
  it('formats minutes, hours and days', () => {
    expect(formatUptime(0)).toBe('0 min');
    expect(formatUptime(59 * 60)).toBe('59 min');
    expect(formatUptime(5 * 3600 + 3 * 60)).toBe('5h 3m');
    expect(formatUptime(3 * 86400 + 4 * 3600 + 59)).toBe('3d 4h');
  });
});
