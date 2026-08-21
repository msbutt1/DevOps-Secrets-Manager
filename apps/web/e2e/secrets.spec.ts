import { expect, test } from '@playwright/test';
import { login, openVault, registerAndVerify, unique } from './helpers';

test('owner creates a vault and secret, reveals it and finds the reveal in the audit log', async ({
  page,
}) => {
  const email = `${unique('owner')}@example.test`;
  const vaultName = unique('e2e-vault');
  const secretValue = `sk_e2e_${Date.now()}`;

  await registerAndVerify(page, 'E2E Owner', email);
  await login(page, email);

  // Create a vault
  await page.getByRole('link', { name: 'Vaults' }).first().click();
  await page.getByRole('button', { name: /New Vault/ }).click();
  await page.getByPlaceholder('e.g., production-secrets').fill(vaultName);
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await openVault(page, vaultName);

  // Add an environment
  await page.getByRole('button', { name: /Add First Environment/ }).click();
  await page.locator('#environment-name').fill('staging');
  await page.getByRole('button', { name: 'Create', exact: true }).click();
  await expect(page.getByRole('button', { name: /STAGING/ })).toBeVisible();

  // Add a secret
  await page
    .getByRole('button', { name: /Add Secret/ })
    .first()
    .click();
  await page.getByPlaceholder('e.g., DATABASE_URL').fill('PAYMENT_API_KEY');
  await page.getByPlaceholder('Enter secret value...').fill(secretValue);
  await page.getByRole('button', { name: 'Create Secret' }).click();
  const row = page.locator('tr', { hasText: 'PAYMENT_API_KEY' });
  await expect(row).toBeVisible();
  await expect(page.getByText(secretValue)).toHaveCount(0);

  // Reveal it
  await row.getByTitle('Reveal').click();
  await page.getByRole('button', { name: 'Reveal Secret Value' }).click();
  await expect(page.getByText(secretValue)).toBeVisible();
  await page.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(page.getByText(secretValue)).toHaveCount(0);

  // The audit log records the reveal with names
  await page.getByRole('link', { name: 'Audit Log' }).first().click();
  await page.locator('select').nth(1).selectOption('secret.revealed');
  const event = page.locator('tbody tr', { hasText: 'PAYMENT_API_KEY' });
  await expect(event).toHaveCount(1);
  await expect(event).toContainText(email);
  await expect(event).toContainText(vaultName);
  await expect(event).toContainText('staging');
});
