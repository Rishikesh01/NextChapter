// The library grid: rollups from real captures, filters, empty states.
import { expect, test } from './fixtures';
import { apiFetch, seedCapture } from './api-helpers';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

test('cards show the per-series rollups from real captures', async ({
  authedPage,
  user,
}) => {
  const seriesID = await seedCapture(server(), user, {
    title: 'Solo Leveling',
    host: 'site-a.example',
    slug: 'solo-leveling',
    chapter: 100,
  });
  await seedCapture(server(), user, {
    title: 'Solo Leveling',
    host: 'site-b.example',
    slug: 'solo-leveling',
    chapter: 101,
    seriesId: seriesID,
  });
  await apiFetch(server(), user, 'PATCH', `/series/${String(seriesID)}`, {
    rating: 9,
    tags: ['action'],
  });

  await authedPage.goto('/');
  const card = authedPage.getByRole('link', { name: /Solo Leveling/ });
  await expect(card).toBeVisible();
  await expect(card).toContainText('Read till ch 101 · 2 sites');
  await expect(card).toContainText('★ 9');
  await expect(card).toContainText('action');
});

test('status and tag filters narrow the grid', async ({ authedPage, user }) => {
  const readingID = await seedCapture(server(), user, {
    title: 'Reading One',
    host: 'site-a.example',
    slug: 'reading-one',
    chapter: 3,
  });
  const doneID = await seedCapture(server(), user, {
    title: 'Done One',
    host: 'site-a.example',
    slug: 'done-one',
    chapter: 50,
  });
  await apiFetch(server(), user, 'PATCH', `/series/${String(doneID)}`, {
    status: 'completed',
  });
  await apiFetch(server(), user, 'PATCH', `/series/${String(readingID)}`, {
    tags: ['isekai'],
  });

  await authedPage.goto('/');
  await expect(
    authedPage.getByRole('link', { name: /Done One/ }),
  ).toBeVisible();

  await authedPage.getByLabel('Filter by status').selectOption('completed');
  await expect(
    authedPage.getByRole('link', { name: /Done One/ }),
  ).toBeVisible();
  await expect(
    authedPage.getByRole('link', { name: /Reading One/ }),
  ).toHaveCount(0);

  await authedPage.getByLabel('Filter by status').selectOption('');
  await authedPage.getByLabel('Filter by tag').fill('isekai');
  await authedPage.getByLabel('Filter by tag').press('Enter');
  await expect(
    authedPage.getByRole('link', { name: /Reading One/ }),
  ).toBeVisible();
  await expect(authedPage.getByRole('link', { name: /Done One/ })).toHaveCount(
    0,
  );

  // Filter state survives a reload (it lives in the query string).
  await authedPage.reload();
  await expect(
    authedPage.getByRole('link', { name: /Reading One/ }),
  ).toBeVisible();
  await expect(authedPage.getByRole('link', { name: /Done One/ })).toHaveCount(
    0,
  );
});

test('a filter that matches nothing shows the filtered-empty state', async ({
  authedPage,
  user,
}) => {
  await seedCapture(server(), user, {
    title: 'Only One',
    host: 'site-a.example',
    slug: 'only-one',
    chapter: 1,
  });

  await authedPage.goto('/');
  await authedPage.getByLabel('Filter by status').selectOption('dropped');
  await expect(
    authedPage.getByText('No series match these filters'),
  ).toBeVisible();
});
