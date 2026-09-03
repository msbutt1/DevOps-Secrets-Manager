/** Secrets expiring within this many days are highlighted. */
export const EXPIRY_WARNING_DAYS = 14;

export type ExpiryState = 'none' | 'expired' | 'soon' | 'ok';

export interface ExpiryStatus {
  state: ExpiryState;
  /** Whole days until expiry (negative once expired); null without an expiry date */
  days: number | null;
  label: string;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** Classifies an expiry date relative to now. */
export function expiryStatus(expiresAt: string | null | undefined, now = Date.now()): ExpiryStatus {
  if (!expiresAt) return { state: 'none', days: null, label: 'No expiry' };
  const remaining = new Date(expiresAt).getTime() - now;
  if (remaining <= 0) {
    const days = Math.floor(-remaining / DAY_MS);
    return {
      state: 'expired',
      days: -days,
      label: days === 0 ? 'Expired today' : `Expired ${days}d ago`,
    };
  }
  const days = Math.floor(remaining / DAY_MS);
  if (days < EXPIRY_WARNING_DAYS) {
    return { state: 'soon', days, label: days === 0 ? 'Expires today' : `Expires in ${days}d` };
  }
  return { state: 'ok', days, label: `Expires in ${days}d` };
}
