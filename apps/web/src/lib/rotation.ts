/** Secrets due for rotation within this many days are highlighted. */
export const ROTATION_WARNING_DAYS = 7;

export type RotationState = 'overdue' | 'due' | 'ok';

const DAY_MS = 24 * 60 * 60 * 1000;

/** Describes when a secret is next due for rotation, e.g. "Due in 3d" or "Overdue 12d". */
export function rotationStatus(
  nextRotationAt: string,
  now = Date.now(),
): { state: RotationState; label: string } {
  const remaining = new Date(nextRotationAt).getTime() - now;
  if (remaining <= 0) {
    const days = Math.floor(-remaining / DAY_MS);
    return { state: 'overdue', label: days === 0 ? 'Due today' : `Overdue ${days}d` };
  }
  const days = Math.floor(remaining / DAY_MS);
  return {
    state: days < ROTATION_WARNING_DAYS ? 'due' : 'ok',
    label: days === 0 ? 'Due today' : `Due in ${days}d`,
  };
}
