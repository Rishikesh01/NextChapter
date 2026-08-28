// Pins the ADR-0010 serving contract end to end against the embedded binary:
// deep links boot the SPA via the NoRoute fallback, while API-shaped paths
// keep the JSON envelope and /healthz stays JSON.
import { expect, test } from './fixtures';

test('a deep link into a client route boots the SPA (NoRoute index fallback)', async ({
  authedPage,
}) => {
  await authedPage.goto('/rules');
  await expect(
    authedPage.getByRole('heading', { name: 'Site rules' }),
  ).toBeVisible();

  await authedPage.goto('/settings');
  await expect(
    authedPage.getByRole('heading', { name: 'Settings' }),
  ).toBeVisible();
});

test('API-shaped 404s keep the JSON envelope; healthz stays JSON', async ({
  page,
}) => {
  await page.goto('/login');

  const apiNotFound = await page.evaluate(async () => {
    const response = await fetch('/auth/nonexistent');
    return {
      status: response.status,
      body: (await response.json()) as unknown,
    };
  });
  expect(apiNotFound.status).toBe(404);
  expect(apiNotFound.body).toMatchObject({ error: { code: 'not_found' } });

  const healthz = await page.evaluate(async () => {
    const response = await fetch('/healthz');
    return (await response.json()) as { status?: string };
  });
  expect(healthz.status).toBe('ok');
});

test('hashed assets are served immutable; index is no-cache', async ({
  request,
}) => {
  const index = await request.get('/');
  expect(index.status()).toBe(200);
  expect(index.headers()['cache-control']).toBe('no-cache');
  expect(index.headers()['content-type']).toContain('text/html');

  const html = await index.text();
  const assetMatch = /\/assets\/[^"]+\.js/.exec(html);
  expect(assetMatch).not.toBeNull();
  const asset = await request.get(assetMatch?.[0] ?? '');
  expect(asset.status()).toBe(200);
  expect(asset.headers()['cache-control']).toContain('immutable');
});
