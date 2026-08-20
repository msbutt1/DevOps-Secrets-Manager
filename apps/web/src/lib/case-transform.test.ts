import { describe, it, expect } from 'vitest';
import { toCamelCase, toSnakeCase } from './case-transform';

describe('toCamelCase', () => {
  it('converts snake_case keys to camelCase', () => {
    const input = {
      user_name: 'john',
      user_email: 'john@example.com',
    };
    const expected = {
      userName: 'john',
      userEmail: 'john@example.com',
    };
    expect(toCamelCase(input)).toEqual(expected);
  });

  it('handles nested objects', () => {
    const input = {
      user_data: {
        first_name: 'John',
        last_name: 'Doe',
      },
    };
    const expected = {
      userData: {
        firstName: 'John',
        lastName: 'Doe',
      },
    };
    expect(toCamelCase(input)).toEqual(expected);
  });

  it('handles arrays of objects', () => {
    const input = [
      { user_id: 1, user_name: 'Alice' },
      { user_id: 2, user_name: 'Bob' },
    ];
    const expected = [
      { userId: 1, userName: 'Alice' },
      { userId: 2, userName: 'Bob' },
    ];
    expect(toCamelCase(input)).toEqual(expected);
  });

  it('handles null and undefined', () => {
    expect(toCamelCase(null)).toBeNull();
    expect(toCamelCase(undefined)).toBeUndefined();
  });

  it('passes through primitive values', () => {
    expect(toCamelCase('string')).toBe('string');
    expect(toCamelCase(123)).toBe(123);
    expect(toCamelCase(true)).toBe(true);
  });
});

describe('toSnakeCase', () => {
  it('converts camelCase keys to snake_case', () => {
    const input = {
      userName: 'john',
      userEmail: 'john@example.com',
    };
    const expected = {
      user_name: 'john',
      user_email: 'john@example.com',
    };
    expect(toSnakeCase(input)).toEqual(expected);
  });

  it('handles nested objects', () => {
    const input = {
      userData: {
        firstName: 'John',
        lastName: 'Doe',
      },
    };
    const expected = {
      user_data: {
        first_name: 'John',
        last_name: 'Doe',
      },
    };
    expect(toSnakeCase(input)).toEqual(expected);
  });

  it('handles arrays of objects', () => {
    const input = [
      { userId: 1, userName: 'Alice' },
      { userId: 2, userName: 'Bob' },
    ];
    const expected = [
      { user_id: 1, user_name: 'Alice' },
      { user_id: 2, user_name: 'Bob' },
    ];
    expect(toSnakeCase(input)).toEqual(expected);
  });

  it('handles null and undefined', () => {
    expect(toSnakeCase(null)).toBeNull();
    expect(toSnakeCase(undefined)).toBeUndefined();
  });

  it('passes through primitive values', () => {
    expect(toSnakeCase('string')).toBe('string');
    expect(toSnakeCase(123)).toBe(123);
    expect(toSnakeCase(true)).toBe(true);
  });
});

describe('free-form metadata', () => {
  it('keeps label keys exactly as the user wrote them in both directions', () => {
    const fromApi = toCamelCase<{ keyName: string; metadata: Record<string, string> }>({
      key_name: 'DATABASE_URL',
      metadata: { team_name: 'payments', costCenter: 'cc-1', Owner: 'ops' },
    });
    expect(fromApi.keyName).toBe('DATABASE_URL');
    expect(fromApi.metadata).toEqual({ team_name: 'payments', costCenter: 'cc-1', Owner: 'ops' });

    const toApi = toSnakeCase<Record<string, unknown>>({
      keyName: 'DATABASE_URL',
      metadata: { team_name: 'payments', costCenter: 'cc-1' },
    });
    expect(toApi).toEqual({
      key_name: 'DATABASE_URL',
      metadata: { team_name: 'payments', costCenter: 'cc-1' },
    });
  });
});
