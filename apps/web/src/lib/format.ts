/** Formats a duration in seconds as e.g. "12 min", "5h 3m" or "3d 4h". */
export function formatUptime(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h`;
}

/** Formats how long ago a timestamp was, e.g. "just now", "5 min ago", "3h ago" or "2d ago". */
export function formatRelativeTime(timestamp: string, now: number = Date.now()): string {
  const minutes = Math.floor((now - new Date(timestamp).getTime()) / 60000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** Summarises a User-Agent header as a client and platform, e.g. "Firefox on Linux". */
export function describeUserAgent(userAgent: string | null | undefined): string {
  if (!userAgent) return 'Unknown client';
  const ua = userAgent;
  if (/^secrets-cli\//i.test(ua)) return 'secrets CLI';
  if (/^curl\//i.test(ua)) return 'curl';

  let client = 'Unknown browser';
  if (/Edg\//.test(ua)) client = 'Edge';
  else if (/OPR\//.test(ua)) client = 'Opera';
  else if (/Firefox\//.test(ua)) client = 'Firefox';
  else if (/Chrome\//.test(ua) || /HeadlessChrome\//.test(ua)) client = 'Chrome';
  else if (/Safari\//.test(ua)) client = 'Safari';

  let platform = '';
  if (/iPhone|iPad|iPod/.test(ua)) platform = 'iOS';
  else if (/Android/.test(ua)) platform = 'Android';
  else if (/Windows/.test(ua)) platform = 'Windows';
  else if (/Mac OS X|Macintosh/.test(ua)) platform = 'macOS';
  else if (/Linux|X11/.test(ua)) platform = 'Linux';

  if (client === 'Unknown browser' && !platform) return ua.length > 40 ? `${ua.slice(0, 40)}…` : ua;
  return platform ? `${client} on ${platform}` : client;
}
