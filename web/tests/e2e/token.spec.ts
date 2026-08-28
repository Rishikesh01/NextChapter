// The mint-once extension token card — and proof the minted token really
// authenticates on the Bearer channel.
import { expect, test } from './fixtures';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

test('mint a token, shown exactly once, that authenticates via Bearer', async ({
  authedPage,
  user,
}) => {
  await authedPage.goto('/settings');
  await expect(authedPage.getByText(`Signed in as`)).toContainText(
    user.username,
  );

  await authedPage.getByLabel('Token label').fill('laptop-firefox');
  await authedPage.getByRole('button', { name: 'Create token' }).click();

  await expect(
    authedPage.getByText('Token laptop-firefox created.'),
  ).toBeVisible();
  await expect(authedPage.getByRole('alert')).toHaveText(
    /won.t be shown again/,
  );
  const token = await authedPage.getByLabel('Your new token').inputValue();
  expect(token.startsWith('nca_')).toBe(true);

  // The minted token works on the Bearer channel (what the extension does).
  const me = await fetch(`${server()}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(me.status).toBe(200);

  // "Done" returns to the mint form; the plaintext is gone for good.
  await authedPage.getByRole('button', { name: 'Done' }).click();
  await expect(
    authedPage.getByRole('button', { name: 'Create token' }),
  ).toBeVisible();
  await expect(authedPage.getByLabel('Your new token')).toHaveCount(0);
});
