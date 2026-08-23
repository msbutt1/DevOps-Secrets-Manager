import { expect, test } from '@playwright/test';
import { inviteLink, login, openVault, PASSWORD, registerAndVerify, unique } from './helpers';

test('an invited viewer joins through the invitation link and cannot reveal secrets', async ({
  browser,
}) => {
  const ownerEmail = `${unique('owner')}@example.test`;
  const viewerEmail = `${unique('viewer')}@example.test`;
  const vaultName = unique('team-vault');
  const secretValue = `viewer-must-not-see-${Date.now()}`;

  // Owner sets up a vault with a secret
  const ownerContext = await browser.newContext();
  const owner = await ownerContext.newPage();
  await registerAndVerify(owner, 'Team Owner', ownerEmail);
  await login(owner, ownerEmail);
  await owner.getByRole('link', { name: 'Vaults' }).first().click();
  await owner.getByRole('button', { name: /New Vault/ }).click();
  await owner.getByPlaceholder('e.g., production-secrets').fill(vaultName);
  await owner.getByRole('button', { name: 'Create', exact: true }).click();
  await openVault(owner, vaultName);
  await owner.getByRole('button', { name: /Add First Environment/ }).click();
  await owner.locator('#environment-name').fill('production');
  await owner.getByRole('button', { name: 'Create', exact: true }).click();
  await owner
    .getByRole('button', { name: /Add Secret/ })
    .first()
    .click();
  await owner.getByPlaceholder('e.g., DATABASE_URL').fill('DATABASE_URL');
  await owner.getByPlaceholder('Enter secret value...').fill(secretValue);
  await owner.getByRole('button', { name: 'Create Secret' }).click();
  await expect(owner.locator('tr', { hasText: 'DATABASE_URL' })).toBeVisible();

  // Owner invites the viewer from Settings
  await owner.getByRole('link', { name: 'Settings' }).first().click();
  await owner.getByRole('button', { name: 'Invite' }).first().click();
  await owner.getByLabel('Email:').fill(viewerEmail);
  await owner.getByLabel('Organization role:').selectOption('viewer');
  await owner.getByRole('button', { name: 'Send Invitation' }).click();
  await expect(owner.getByText('Invitation sent', { exact: true })).toBeVisible();

  // The viewer opens the link in their own browser and creates an account
  const viewerContext = await browser.newContext();
  const viewer = await viewerContext.newPage();
  await viewer.goto(await inviteLink(viewerEmail));
  await expect(viewer.getByText(viewerEmail)).toBeVisible();
  await viewer.getByRole('button', { name: 'Create Account' }).click();
  await expect(viewer.locator('#email')).toHaveValue(viewerEmail);
  await viewer.locator('#name').fill('Team Viewer');
  await viewer.locator('#password').fill(PASSWORD);
  await viewer.locator('#confirmPassword').fill(PASSWORD);
  await viewer.getByRole('button', { name: 'Register' }).click();
  await expect(viewer.getByRole('heading', { name: 'System Dashboard' })).toBeVisible();

  // The link is single use
  const reuse = await viewerContext.newPage();
  await reuse.goto(await inviteLink(viewerEmail));
  await expect(reuse.getByRole('alert')).toContainText(/invalid, expired or already used/);

  // Owner gives the new member the viewer role on the vault
  await openVault(owner, vaultName);
  await owner.getByRole('button', { name: 'Members' }).click();
  await owner.getByRole('button', { name: /Add Member/ }).click();
  await owner.getByPlaceholder('user@company.com').fill(viewerEmail);
  await owner.locator('select').first().selectOption('viewer');
  await owner.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(owner.getByRole('cell', { name: new RegExp(viewerEmail) })).toBeVisible();

  // The viewer sees the secret's name but has no way to reveal it
  await openVault(viewer, vaultName);
  const row = viewer.locator('tr', { hasText: 'DATABASE_URL' });
  await expect(row).toBeVisible();
  await expect(row.getByTitle('Reveal')).toHaveCount(0);
  await expect(row.getByTitle('Delete')).toHaveCount(0);
  await expect(viewer.getByText(secretValue)).toHaveCount(0);

  await ownerContext.close();
  await viewerContext.close();
});
