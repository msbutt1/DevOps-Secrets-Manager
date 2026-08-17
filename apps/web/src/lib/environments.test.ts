import {
  isProductionEnvironment,
  validateEnvironmentName,
  ENVIRONMENT_NAME_HINT,
} from './environments';

describe('validateEnvironmentName', () => {
  it('accepts custom names', () => {
    for (const name of ['dev', 'production', 'qa', 'eu-west-1', 'preview_42', 'v1.2']) {
      expect(validateEnvironmentName(name)).toBeNull();
    }
  });

  it('rejects invalid names with the shared hint', () => {
    for (const name of ['Production', '-dev', 'has space', 'a'.repeat(65)]) {
      expect(validateEnvironmentName(name)).toBe(ENVIRONMENT_NAME_HINT);
    }
    expect(validateEnvironmentName('  ')).toBe('Enter an environment name');
  });

  it('rejects names already used in the vault', () => {
    expect(validateEnvironmentName('staging', ['staging'])).toMatch(/already has/);
  });
});

describe('isProductionEnvironment', () => {
  it('recognises production-like names', () => {
    for (const name of ['prod', 'production', 'prd', 'live', 'prod-eu', 'production.us']) {
      expect(isProductionEnvironment(name)).toBe(true);
    }
    for (const name of ['staging', 'preprod', 'products', null, undefined, '']) {
      expect(isProductionEnvironment(name)).toBe(false);
    }
  });
});
