/**
 * Environment names: lowercase letters, digits, dots, hyphens and underscores, starting with a
 * letter or digit, at most 64 characters. Identical to NamePattern in the API
 * (apps/api/internal/environments/name.go); a Go test compares the two.
 */
export const ENVIRONMENT_NAME_PATTERN = /^[a-z0-9][a-z0-9._-]{0,63}$/;

export const ENVIRONMENT_NAME_HINT =
  'Lowercase letters, digits, dots, hyphens and underscores (max 64 characters)';

/** Common names offered as shortcuts in the create form. Any valid name is allowed. */
export const SUGGESTED_ENVIRONMENTS: { name: string; description: string }[] = [
  { name: 'development', description: 'Local development and testing' },
  { name: 'staging', description: 'Pre-production testing' },
  { name: 'production', description: 'Live systems' },
];

/** Returns an error message, or null when the name is valid and not already used. */
export function validateEnvironmentName(name: string, existing: string[] = []): string | null {
  const trimmed = name.trim();
  if (!trimmed) return 'Enter an environment name';
  if (!ENVIRONMENT_NAME_PATTERN.test(trimmed)) return ENVIRONMENT_NAME_HINT;
  if (existing.includes(trimmed)) return `This vault already has an environment named "${trimmed}"`;
  return null;
}

/** Whether a name looks like a production environment, for warnings and highlighting. */
export function isProductionEnvironment(name: string | null | undefined): boolean {
  return !!name && /^(prod|production|prd|live)([._-].*)?$/.test(name);
}
