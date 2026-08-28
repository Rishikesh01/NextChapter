// Series detail: per-site rows, continue links, inline editors, delete.
import { expect, test } from './fixtures';
import { seedCapture } from './api-helpers';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

test('entries table lists the sites with working continue links', async ({
  authedPage,
  user,
}) => {
  const seriesID = await seedCapture(server(), user, {
    title: 'Detail Series',
    host: 'site-a.example',
    slug: 'detail-series',
    chapter: 10,
  });
  await seedCapture(server(), user, {
    title: 'Detail Series',
    host: 'site-b.example',
    slug: 'detail-series',
    chapter: 12.5,
    seriesId: seriesID,
  });

  await authedPage.goto(`/library/${String(seriesID)}`);
  await expect(
    authedPage.getByRole('heading', { name: 'Detail Series' }),
  ).toBeVisible();
  await expect(authedPage.getByText('Sites · read till ch 12.5')).toBeVisible();

  const rowA = authedPage.getByRole('row', { name: /site-a\.example/ });
  await expect(rowA).toContainText('at ch 10');
  await expect(
    rowA.getByRole('link', { name: /Continue reading/ }),
  ).toHaveAttribute(
    'href',
    'https://site-a.example/s/detail-series/chapter-10',
  );
});

test('status, rating, tags and notes edits persist across a reload', async ({
  authedPage,
  user,
}) => {
  const seriesID = await seedCapture(server(), user, {
    title: 'Editable Series',
    host: 'site-a.example',
    slug: 'editable-series',
    chapter: 4,
  });

  await authedPage.goto(`/library/${String(seriesID)}`);
  await authedPage.getByLabel('Status').selectOption('on_hold');
  await expect(authedPage.getByText('Saved').first()).toBeVisible();

  await authedPage.getByRole('button', { name: 'Rate' }).click();
  await authedPage.getByLabel('Rating').selectOption('8');
  await expect(authedPage.getByText('Saved').first()).toBeVisible();

  await authedPage.getByLabel('Add a tag').fill('reincarnation');
  await authedPage.getByLabel('Add a tag').press('Enter');
  await expect(authedPage.getByText('Saved').first()).toBeVisible();

  await authedPage.getByLabel('Notes').fill('Anime covers up to ch 3.');
  await authedPage.getByRole('button', { name: 'Save notes' }).click();
  await expect(authedPage.getByText('Saved').last()).toBeVisible();

  await authedPage.reload();
  await expect(authedPage.getByLabel('Status')).toHaveValue('on_hold');
  await expect(authedPage.getByLabel('Rating')).toHaveValue('8');
  await expect(authedPage.getByText('reincarnation')).toBeVisible();
  await expect(authedPage.getByLabel('Notes')).toHaveValue(
    'Anime covers up to ch 3.',
  );
});

test('entry chapter correction and removal', async ({ authedPage, user }) => {
  const seriesID = await seedCapture(server(), user, {
    title: 'Correctable',
    host: 'site-a.example',
    slug: 'correctable',
    chapter: 7,
  });
  await seedCapture(server(), user, {
    title: 'Correctable',
    host: 'site-b.example',
    slug: 'correctable',
    chapter: 5,
    seriesId: seriesID,
  });

  await authedPage.goto(`/library/${String(seriesID)}`);
  const rowA = authedPage.getByRole('row', { name: /site-a\.example/ });
  await rowA.getByRole('button', { name: 'Edit' }).click();
  await authedPage.getByLabel('Chapter').fill('7.5');
  await authedPage.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(rowA).toContainText('at ch 7.5');

  const rowB = authedPage.getByRole('row', { name: /site-b\.example/ });
  await rowB.getByRole('button', { name: 'Remove' }).click();
  await rowB.getByRole('button', { name: 'Confirm' }).click();
  await expect(
    authedPage.getByRole('row', { name: /site-b\.example/ }),
  ).toHaveCount(0);
});

test('delete series behind the confirm returns to the library', async ({
  authedPage,
  user,
}) => {
  const seriesID = await seedCapture(server(), user, {
    title: 'Doomed Series',
    host: 'site-a.example',
    slug: 'doomed-series',
    chapter: 1,
  });

  await authedPage.goto(`/library/${String(seriesID)}`);
  await authedPage.getByRole('button', { name: 'Delete series' }).click();
  await authedPage.getByRole('button', { name: 'Confirm' }).click();

  await expect(
    authedPage.getByRole('heading', { name: 'Library' }),
  ).toBeVisible();
  await expect(
    authedPage.getByRole('link', { name: /Doomed Series/ }),
  ).toHaveCount(0);
});
