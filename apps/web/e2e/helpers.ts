import { expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

export const PASSWORD = 'E2e-Test-Passw0rd!';

const apiLog =
  process.env.E2E_API_LOG ?? join(homedir(), '.local/share/devops-secrets-manager/api.log');

/** A unique, valid identifier for names and emails created by one test run. */
export const unique = (prefix: string) =>
  `${prefix}-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;

/** Finds the newest development verification link logged for the address. */
export async function verificationLink(email: string): Promise<string> {
  for (let attempt = 0; attempt < 50; attempt++) {
    const lines = readFileSync(apiLog, 'utf8').split('\n');
    for (let i = lines.length - 1; i >= 0; i--) {
      if (!lines[i].includes(email) || !lines[i].includes('"link"')) continue;
      const entry = JSON.parse(lines[i]) as { to?: string; link?: string };
      if (entry.to === email && entry.link?.includes('/verify-email')) {
        return entry.link;
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`No verification link for ${email} in ${apiLog}`);
}

/** Registers through the UI, verifies with the logged link and ends on the login page. */
export async function registerAndVerify(page: Page, name: string, email: string) {
  await page.goto('/register');
  await page.locator('#name').fill(name);
  await page.locator('#email').fill(email);
  await page.locator('#password').fill(PASSWORD);
  await page.locator('#confirmPassword').fill(PASSWORD);
  await page.getByRole('button', { name: 'Register' }).click();
  await expect(page.getByText(/Registration Successful/i)).toBeVisible();

  const link = new URL(await verificationLink(email));
  await page.goto(link.pathname + link.search);
  await expect(page.getByText('Email Verified!')).toBeVisible();
}

export async function login(page: Page, email: string, password = PASSWORD) {
  await page.goto('/login');
  await page.locator('#email').fill(email);
  await page.locator('#password').fill(password);
  await page.getByRole('button', { name: 'Login' }).click();
  await expect(page.getByRole('heading', { name: 'System Dashboard' })).toBeVisible();
}

/** Client-side navigation: a full page load would drop the in-memory session. */
export async function openVault(page: Page, vaultName: string) {
  await page.getByRole('link', { name: 'Vaults' }).first().click();
  await page.getByRole('link', { name: vaultName, exact: true }).first().click();
  await expect(page.getByText(/Created (by .* )?on/)).toBeVisible();
}
