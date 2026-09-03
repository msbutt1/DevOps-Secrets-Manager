import { describe, expect, it } from 'vitest';
import { formatDotenv, formatLine, parseDotenv, type Pair } from './dotenv';

describe('dotenv', () => {
  it('quotes only when needed, like the CLI', () => {
    expect(formatLine('URL', 'postgres://u@db:5432/app')).toBe('URL=postgres://u@db:5432/app');
    expect(formatLine('EMPTY', '')).toBe('EMPTY=');
    expect(formatLine('GREETING', 'hello world #1')).toBe("GREETING='hello world #1'");
    expect(formatLine('QUOTE', "it's")).toBe('QUOTE="it\'s"');
    expect(formatLine('PEM', 'a\nb\\c"d')).toBe('PEM="a\\nb\\\\c\\"d"');
  });

  it('round-trips awkward values', () => {
    const original: Pair[] = [
      ['PLAIN', 'abc123'],
      ['SPACES', '  leading and trailing  '],
      ['HASH', 'pa#ss word'],
      ['DOLLAR', '$HOME and ${PATH}'],
      ['QUOTES', 'it\'s "quoted"'],
      ['MULTILINE', '-----BEGIN KEY-----\nabc\n-----END KEY-----'],
      ['BACKSLASH', 'C:\\path\\to'],
      ['EQUALS', 'a=b=c'],
      ['UNICODE', 'pässwörd ✓'],
      ['EMPTY', ''],
    ];
    expect(parseDotenv(formatDotenv(original))).toEqual(original);
  });

  it('parses common syntax', () => {
    const content =
      '# comment\n\nexport API_KEY=sk_test_123\nDB_URL = postgres://x # inline comment\nSINGLE=\'literal \\n $value\'\nDOUBLE="line1\\nline2"\nMULTI="first\nsecond"\r\nAPI_KEY=override\n';
    expect(parseDotenv(content)).toEqual([
      ['API_KEY', 'override'],
      ['DB_URL', 'postgres://x'],
      ['SINGLE', 'literal \\n $value'],
      ['DOUBLE', 'line1\nline2'],
      ['MULTI', 'first\nsecond'],
    ]);
  });

  it('reports errors with line numbers', () => {
    expect(() => parseDotenv('A=1\nnot a pair\n')).toThrow('line 2: expected KEY=value');
    expect(() => parseDotenv('\n1BAD=x')).toThrow('line 2');
    expect(() => parseDotenv('A="never closed\nB=2')).toThrow('unterminated');
    expect(() => parseDotenv("A='open")).toThrow('unterminated');
  });
});
