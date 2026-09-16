import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { login, openVault, registerAndVerify, unique } from './helpers';

/** Runs axe on the current page and returns violations as readable lines. */
async function violations(page: Page) {
  // Dialogs fade in; scanning mid-animation reports the half-transparent text as low contrast
  await page.waitForTimeout(250);
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze();
  return results.violations.map(
    (v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(' ')).join('; ')}`,
  );
}

test('the signed-out pages have no accessibility violations', async ({ page }) => {
  for (const path of [
    '/login',
    '/register',
    '/forgot-password',
    '/reset-password?token=x',
    '/legal',
  ]) {
    await page.goto(path);
    expect(await violations(page), `on ${path}`).toEqual([]);
  }
});

test('the signed-in pages and dialogs have no accessibility violations', async ({ page }) => {
  const email = `${unique('a11y')}@example.test`;
  const vaultName = unique('a11y-vault');
  await registerAndVerify(page, 'Axe Tester', email);
  await login(page, email);
  expect(await violations(page), 'on the dashboard').toEqual([]);

  await page.getByRole('link', { name: 'Vaults' }).first().click();
  await page.getByRole('button', { name: /New Vault/ }).click();
  expect(await violations(page), 'in the new vault dialog').toEqual([]);
  await page.getByPlaceholder('e.g., production-secrets').fill(vaultName);
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await openVault(page, vaultName);

  await page.getByRole('button', { name: 'Add First Environment' }).click();
  await page.locator('#environment-name').fill('production');
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await page.getByRole('button', { name: /Add Secret/ }).click();
  expect(await violations(page), 'in the secret dialog').toEqual([]);
  await page.locator('#secret-name').fill('API_KEY');
  await page.locator('#secret-value').fill('sk_a11y_value');
  await page.getByRole('button', { name: 'Create Secret' }).click();
  expect(await violations(page), 'on the vault page').toEqual([]);

  await page.getByRole('link', { name: 'Audit Log' }).first().click();
  expect(await violations(page), 'on the audit log').toEqual([]);
  await page.getByRole('link', { name: 'Settings' }).first().click();
  expect(await violations(page), 'on settings').toEqual([]);
  await page.getByRole('link', { name: 'Organization' }).first().click();
  expect(await violations(page), 'on the organization page').toEqual([]);
});
