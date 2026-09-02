import { describeUserAgent, formatRelativeTime, formatUptime } from './format';

describe('formatUptime', () => {
  it('formats minutes, hours and days', () => {
    expect(formatUptime(0)).toBe('0 min');
    expect(formatUptime(59 * 60)).toBe('59 min');
    expect(formatUptime(5 * 3600 + 3 * 60)).toBe('5h 3m');
    expect(formatUptime(3 * 86400 + 4 * 3600 + 59)).toBe('3d 4h');
  });
});

describe('formatRelativeTime', () => {
  const now = Date.parse('2026-09-15T12:00:00Z');
  it('describes recent and older timestamps', () => {
    expect(formatRelativeTime('2026-09-15T11:59:40Z', now)).toBe('just now');
    expect(formatRelativeTime('2026-09-15T11:55:00Z', now)).toBe('5 min ago');
    expect(formatRelativeTime('2026-09-15T09:00:00Z', now)).toBe('3h ago');
    expect(formatRelativeTime('2026-09-13T12:00:00Z', now)).toBe('2d ago');
  });
});

describe('describeUserAgent', () => {
  it.each([
    ['Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0', 'Firefox on Linux'],
    [
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0 Safari/537.36',
      'Chrome on Windows',
    ],
    [
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/139.0 Safari/537.36 Edg/139.0',
      'Edge on Windows',
    ],
    [
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile/15E148 Safari/604.1',
      'Safari on iOS',
    ],
    [
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 Version/17.5 Safari/605.1.15',
      'Safari on macOS',
    ],
    ['secrets-cli/1.0.0', 'secrets CLI'],
    ['curl/8.5.0', 'curl'],
    ['', 'Unknown client'],
    [null, 'Unknown client'],
  ])('%s', (ua, expected) => {
    expect(describeUserAgent(ua)).toBe(expected);
  });
});
