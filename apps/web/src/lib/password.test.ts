import { describe, expect, it } from 'vitest';
import { passwordProblem, passwordStrength } from './password';

const personal = ['salaar@demo.dev', 'Salaar Butt'];

describe('passwordProblem', () => {
  it('accepts passwords the API accepts', () => {
    for (const pw of ['Demo-Passw0rd!2026', 'correct horse battery staple', 'vK8#qLz2@wNp']) {
      expect(passwordProblem(pw, personal)).toBeNull();
    }
  });

  it.each([
    ['Sh0rt-Pass!', 'at least 12 characters'],
    ['Password123456', 'too common'],
    ['P@ssw0rd2024!!', 'too common'],
    ['qwertyuiop123', 'too common'],
    ['1234567890123', 'too common'],
    ['aaaaaaaaaaaaaaaa', '5 different characters'],
    ['Salaar-is-great-2026', 'name or email'],
    ['ab1-Cd2_'.repeat(9) + 'x', 'at most 72 bytes'],
  ])('rejects %s', (pw, message) => {
    expect(passwordProblem(pw, personal)).toContain(message);
  });
});

describe('passwordStrength', () => {
  it('rates rejected passwords as too weak and long varied ones as strong', () => {
    expect(passwordStrength('password1234')).toBe(0);
    expect(passwordStrength('plain-lowercase')).toBeLessThan(
      passwordStrength('Longer-Mixed-Case-Phrase-42'),
    );
    expect(passwordStrength('Longer-Mixed-Case-Phrase-42')).toBe(4);
  });
});
