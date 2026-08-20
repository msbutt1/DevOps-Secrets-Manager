/**
 * Converts a snake_case string to camelCase
 */
function snakeToCamel(str: string): string {
  return str.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
}

/**
 * Converts a camelCase string to snake_case
 */
function camelToSnake(str: string): string {
  return str.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

/**
 * Checks if a value is a plain object (not an array, null, Date, etc.)
 */
function isPlainObject(value: unknown): value is Record<string, unknown> {
  return (
    value !== null && typeof value === 'object' && !Array.isArray(value) && !(value instanceof Date)
  );
}

/**
 * Keys whose values are user-defined maps (secret labels, audit metadata). Their own keys are
 * data, not field names, so they are passed through unchanged in both directions.
 */
const FREE_FORM_KEYS = new Set(['metadata']);

/**
 * Convert object keys from snake_case to camelCase (for API responses)
 */
export function toCamelCase<T>(obj: unknown): T {
  // Handle null/undefined
  if (obj === null || obj === undefined) {
    return obj as T;
  }

  // Handle arrays - recursively transform each element
  if (Array.isArray(obj)) {
    return obj.map((item) => toCamelCase(item)) as T;
  }

  // Handle plain objects - transform keys and values recursively
  if (isPlainObject(obj)) {
    const result: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(obj)) {
      const camelKey = snakeToCamel(key);
      result[camelKey] = FREE_FORM_KEYS.has(key) ? value : toCamelCase(value);
    }

    return result as T;
  }

  // Primitive values - pass through unchanged
  return obj as T;
}

/**
 * Convert object keys from camelCase to snake_case (for API requests)
 */
export function toSnakeCase<T>(obj: unknown): T {
  // Handle null/undefined
  if (obj === null || obj === undefined) {
    return obj as T;
  }

  // Handle arrays - recursively transform each element
  if (Array.isArray(obj)) {
    return obj.map((item) => toSnakeCase(item)) as T;
  }

  // Handle plain objects - transform keys and values recursively
  if (isPlainObject(obj)) {
    const result: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(obj)) {
      const snakeKey = camelToSnake(key);
      result[snakeKey] = FREE_FORM_KEYS.has(snakeKey) ? value : toSnakeCase(value);
    }

    return result as T;
  }

  // Primitive values - pass through unchanged
  return obj as T;
}
