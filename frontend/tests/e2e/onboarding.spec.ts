// The cookie-flow regression test (ADR-0008 §6): drives the real options page
// through server connect → create account → automatic token mint → verify.
import { expect, test } from './fixtures';

test('onboarding: connect, create account, token minted automatically', async ({
  context,
  extensionId,
}) => {
  const page = await context.newPage();
  await page.goto(`chrome-extension://${extensionId}/options.html`);

  await page.getByLabel('Server URL').fill(process.env.NC_E2E_SERVER ?? '');
  await page.getByRole('button', { name: 'Connect' }).click();
  await expect(page.getByRole('status')).toHaveText(/Server reachable/);

  await page.getByRole('button', { name: /Create an account instead/ }).click();
  const username = `e2e-onboard-${String(Date.now())}`;
  await page.getByLabel('Username').fill(username);
  await page.getByLabel('Password').fill('e2e-password');
  await page.getByRole('button', { name: 'Create account' }).click();

  await expect(page.getByText(`Connected as`)).toBeVisible();
  await expect(page.getByText(username)).toBeVisible();
});

test('onboarding: paste-token fallback verifies before saving', async ({
  context,
  extensionId,
}) => {
  const page = await context.newPage();
  await page.goto(`chrome-extension://${extensionId}/options.html`);

  await page.getByLabel('Server URL').fill(process.env.NC_E2E_SERVER ?? '');
  await page.getByRole('button', { name: 'Connect' }).click();
  await expect(page.getByRole('status')).toHaveText(/Server reachable/);

  await page.getByRole('tab', { name: 'Paste token' }).click();
  await page.getByLabel('API token').fill('nca_definitely-not-a-real-token');
  await page.getByRole('button', { name: 'Save token' }).click();
  await expect(page.getByRole('alert')).toHaveText(/rejected this token/);

  await page.getByLabel('API token').fill(process.env.NC_E2E_TOKEN ?? '');
  await page.getByRole('button', { name: 'Save token' }).click();
  await expect(page.getByText('Connected as')).toBeVisible();
});
