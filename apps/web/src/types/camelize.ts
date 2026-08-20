/** Converts a snake_case string literal type to camelCase: `key_name` -> `keyName`. */
export type CamelCase<S extends string> = S extends `${infer Head}_${infer Tail}`
  ? `${Head}${Capitalize<CamelCase<Tail>>}`
  : S;

/**
 * Deeply renames object keys from snake_case to camelCase, mirroring what toCamelCase in
 * src/lib/case-transform.ts does to API responses at runtime.
 */
export type Camelize<T> = T extends readonly (infer Item)[]
  ? Camelize<Item>[]
  : T extends object
    ? { [K in keyof T as K extends string ? CamelCase<K> : K]: Camelize<T[K]> }
    : T;
