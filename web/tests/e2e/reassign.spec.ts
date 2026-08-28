// Entry reassignment — the README's first-class feature: move a per-site
// thread between series, or onto a brand-new one, with rollups shifting.
import { expect, test } from './fixtures';
import { seedCapture } from './api-helpers';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

test('move an entry to an existing series shifts both rollups', async ({
  authedPage,
  user,
}) => {
  const fromID = await seedCapture(server(), user, {
    title: 'Wrong Home',
    host: 'site-a.example',
    slug: 'shared-slug',
    chapter: 40,
  });
  await seedCapture(server(), user, {
    title: 'Wrong Home',
    host: 'site-b.example',
    slug: 'shared-slug',
    chapter: 44,
    seriesId: fromID,
  });
  await seedCapture(server(), user, {
    title: 'Right Home',
    host: 'site-c.example',
    slug: 'right-home',
    chapter: 10,
  });

  await authedPage.goto(`/library/${String(fromID)}`);
  const rowB = authedPage.getByRole('row', { name: /site-b\.example/ });
  await rowB.getByRole('button', { name: 'Move', exact: true }).click();

  const dialog = authedPage.getByRole('dialog', { name: 'Move this entry' });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: /Right Home/ }).click();

  // The moved thread leaves this series; the rollup drops to the remaining entry.
  await expect(
    authedPage.getByRole('row', { name: /site-b\.example/ }),
  ).toHaveCount(0);
  await expect(authedPage.getByText('Sites · read till ch 40')).toBeVisible();

  // And lands on the target with ITS rollup raised.
  await authedPage.goto('/');
  await expect(
    authedPage.getByRole('link', { name: /Right Home/ }),
  ).toContainText('Read till ch 44 · 2 sites');
});

test('move an entry onto a NEW series (create + move, non-atomic by design)', async ({
  authedPage,
  user,
}) => {
  const fromID = await seedCapture(server(), user, {
    title: 'Mixed Bag',
    host: 'site-a.example',
    slug: 'mixed-bag',
    chapter: 20,
  });
  await seedCapture(server(), user, {
    title: 'Mixed Bag',
    host: 'site-b.example',
    slug: 'actually-different',
    chapter: 3,
    seriesId: fromID,
  });

  await authedPage.goto(`/library/${String(fromID)}`);
  const rowB = authedPage.getByRole('row', { name: /site-b\.example/ });
  await rowB.getByRole('button', { name: 'Move', exact: true }).click();

  const dialog = authedPage.getByRole('dialog', { name: 'Move this entry' });
  await dialog.getByRole('searchbox').fill('The Different One');
  await dialog.getByRole('button', { name: /Create new series/ }).click();

  await expect(
    authedPage.getByRole('row', { name: /site-b\.example/ }),
  ).toHaveCount(0);
  await authedPage.goto('/');
  await expect(
    authedPage.getByRole('link', { name: /The Different One/ }),
  ).toContainText('Read till ch 3 · 1 site');
});
