// Auth flows through the real UI against the real backend.
import { expect, test } from './fixtures';

test('register through the UI lands in the (empty) library', async ({
  page,
}) => {
  await page.goto('/register');
  await page.getByLabel('Username').fill(`e2e-reg-${String(Date.now())}`);
  await page.getByLabel('Password').fill('e2e-password');
  await page.getByRole('button', { name: 'Create account' }).click();

  await expect(page.getByRole('heading', { name: 'Library' })).toBeVisible();
  await expect(page.getByText('Nothing here yet')).toBeVisible();
});

test('login rejects wrong credentials, then accepts the right ones', async ({
  page,
  user,
}) => {
  await page.goto('/login');
  await page.getByLabel('Username').fill(user.username);
  await page.getByLabel('Password').fill('wrong-password');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('alert')).toHaveText(
    'Wrong username or password.',
  );
  // Values are kept for a typo fix, not wiped.
  await expect(page.getByLabel('Username')).toHaveValue(user.username);

  await page.getByLabel('Password').fill(user.password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('heading', { name: 'Library' })).toBeVisible();
});

test('an unauthenticated deep link redirects to login and returns after', async ({
  page,
  user,
}) => {
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();

  await page.getByLabel('Username').fill(user.username);
  await page.getByLabel('Password').fill(user.password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
});

test('sign out from the nav returns to login', async ({ authedPage }) => {
  await authedPage.goto('/');
  await authedPage.getByRole('button', { name: 'Sign out' }).click();
  await expect(
    authedPage.getByRole('heading', { name: 'Sign in' }),
  ).toBeVisible();
});
