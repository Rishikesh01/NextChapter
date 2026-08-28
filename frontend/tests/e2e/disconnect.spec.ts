// Disconnect revokes the API token its own onboarding minted (ADR-0009 §5) —
// the connect/disconnect cycle must not strand a never-expiring credential.
import { expect, test } from './fixtures';

// Present on extension pages, where the evaluate callback runs.
declare const chrome: {
  storage: {
    local: { get: (key: string) => Promise<Record<string, unknown>> };
  };
};

test('Disconnect revokes the minted token', async ({
  context,
  extensionId,
}) => {
  const serverUrl = process.env.NC_E2E_SERVER ?? '';
  const page = await context.newPage();
  await page.goto(`chrome-extension://${extensionId}/options.html`);

  await page.getByLabel('Server URL').fill(serverUrl);
  await page.getByRole('button', { name: 'Connect' }).click();
  await expect(page.getByRole('status')).toHaveText(/Server reachable/);
  await page.getByRole('button', { name: /Create an account instead/ }).click();
  await page.getByLabel('Username').fill(`e2e-revoke-${String(Date.now())}`);
  await page.getByLabel('Password').fill('e2e-password');
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByText('Connected as')).toBeVisible();

  const token = await page.evaluate(async () => {
    const record = await chrome.storage.local.get('settings:v1');
    return (record['settings:v1'] as { apiToken: string }).apiToken;
  });
  const before = await fetch(`${serverUrl}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(before.status).toBe(200);

  await page.getByRole('button', { name: 'Disconnect' }).click();
  await expect(page.getByRole('status')).toHaveText(/Not checked yet/);

  const after = await fetch(`${serverUrl}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(after.status).toBe(401);
});
