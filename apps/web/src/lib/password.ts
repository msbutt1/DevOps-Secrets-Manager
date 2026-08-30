// Mirrors the API's password policy (apps/api/internal/validate) so the form can explain problems
// while typing. The API has the full 10,000-entry common-password list and has the final say.

export const MIN_PASSWORD_LENGTH = 12;
export const MAX_PASSWORD_BYTES = 72;

// The most common base words; the API checks the full list.
const COMMON_WORDS = new Set([
  'password',
  'passwort',
  'qwerty',
  'qwertyuiop',
  'asdfgh',
  'asdfghjkl',
  'zxcvbnm',
  'letmein',
  'welcome',
  'admin',
  'administrator',
  'iloveyou',
  'monkey',
  'dragon',
  'football',
  'baseball',
  'master',
  'sunshine',
  'princess',
  'shadow',
  'superman',
  'batman',
  'trustno',
  'abc',
  'login',
  'secret',
  'changeme',
  'default',
  'starwars',
  'whatever',
  'computer',
  'michael',
  'jennifer',
  'hello',
  'freedom',
  'charlie',
  'jordan',
  'hunter',
  'soccer',
  'hockey',
  'killer',
  'pokemon',
]);

const LEET: Record<string, string> = {
  '0': 'o',
  '1': 'i',
  '3': 'e',
  '4': 'a',
  '5': 's',
  '7': 't',
  '@': 'a',
  $: 's',
};

const unleet = (value: string) => value.replace(/[013457@$]/g, (c) => LEET[c]);
const trimNonLetters = (value: string) => value.replace(/^[^\p{L}]+|[^\p{L}]+$/gu, '');

function isSequence(value: string): boolean {
  let up = true;
  let down = true;
  for (let i = 1; i < value.length; i++) {
    const a = value.charCodeAt(i - 1);
    const b = value.charCodeAt(i);
    const digits = /\d/.test(value[i - 1]) && /\d/.test(value[i]);
    const step = digits ? (b - a + 10) % 10 : b - a;
    up = up && step === 1;
    down = down && (step === -1 || step === 9);
  }
  return up || down;
}

function personalParts(value: string): string[] {
  const local = value.toLowerCase().split('@')[0];
  return local.split(/[^\p{L}\p{N}]+/u).filter((part) => part.length >= 4);
}

/** Returns the first policy problem with the password, or null when it looks acceptable. */
export function passwordProblem(password: string, personal: string[] = []): string | null {
  if (new TextEncoder().encode(password).length > MAX_PASSWORD_BYTES) {
    return `Password must be at most ${MAX_PASSWORD_BYTES} bytes`;
  }
  if ([...password].length < MIN_PASSWORD_LENGTH) {
    return `Password must be at least ${MIN_PASSWORD_LENGTH} characters`;
  }
  const lower = password.toLowerCase();
  if (new Set([...lower]).size < 5) {
    return 'Password must use at least 5 different characters';
  }
  const trimmed = trimNonLetters(lower);
  const variants = [lower, trimmed, unleet(lower), unleet(trimmed)];
  if (isSequence(lower) || variants.some((v) => COMMON_WORDS.has(v))) {
    return 'Password is too common; choose a longer phrase that is not a well-known password';
  }
  const parts = personal.flatMap(personalParts);
  if (parts.some((part) => lower.includes(part) || unleet(lower).includes(part))) {
    return 'Password must not contain your name or email address';
  }
  return null;
}

export type PasswordStrength = 0 | 1 | 2 | 3 | 4;

export const STRENGTH_LABELS = ['Too weak', 'Weak', 'Fair', 'Good', 'Strong'] as const;

/** A rough strength estimate from length and character variety, for the meter only. */
export function passwordStrength(password: string, personal: string[] = []): PasswordStrength {
  if (!password || passwordProblem(password, personal)) return 0;
  let pool = 0;
  if (/[a-z]/.test(password)) pool += 26;
  if (/[A-Z]/.test(password)) pool += 26;
  if (/\d/.test(password)) pool += 10;
  if (/[^A-Za-z0-9]/.test(password)) pool += 33;
  const bits = [...password].length * Math.log2(Math.max(pool, 2));
  if (bits < 60) return 1;
  if (bits < 75) return 2;
  if (bits < 100) return 3;
  return 4;
}
