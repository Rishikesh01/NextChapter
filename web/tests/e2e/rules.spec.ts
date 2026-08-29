// Site-rule management — including regex-level EDITING, the half ADR-0009
// deferred to the web track.
import { expect, test } from './fixtures';
import { seedCapture } from './api-helpers';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

test('lists the seeded defaults and surfaces tracked hosts without rules', async ({
  authedPage,
  user,
}) => {
  await seedCapture(server(), user, {
    title: 'Manual Site Series',
    host: 'no-rule-site.example',
    slug: 'manual-one',
    chapter: 2,
  });

  await authedPage.goto('/rules');
  await expect(authedPage.getByText('wuxiaworld.com')).toBeVisible();

  const hintRow = authedPage.getByRole('row', {
    name: /no-rule-site\.example/,
  });
  await expect(hintRow).toContainText(
    'No rule yet — captures here are manual.',
  );
});

test('create, edit the regex, and delete a rule', async ({ authedPage }) => {
  await authedPage.goto('/rules');

  // Create.
  await authedPage.getByRole('button', { name: 'Add rule' }).click();
  await authedPage.getByLabel('Host').fill('scans.example');
  await authedPage
    .getByLabel('Chapter URL pattern')
    .fill('^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9]+(?:\\.[0-9]+)?)$');
  await authedPage.getByRole('button', { name: 'Save rule' }).click();

  const row = authedPage.getByRole('row', { name: /scans\.example/ });
  await expect(row).toContainText(
    '^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9]+(?:\\.[0-9]+)?)$',
  );

  // Edit the regex in place.
  await row.getByRole('button', { name: 'Edit' }).click();
  const regexInput = authedPage.getByLabel('Chapter URL pattern');
  await regexInput.fill('^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+)$');
  await authedPage.getByRole('button', { name: 'Save rule' }).click();
  await expect(
    authedPage.getByRole('row', { name: /scans\.example/ }),
  ).toContainText('^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+)$');

  // Delete behind the confirm.
  const updated = authedPage.getByRole('row', { name: /scans\.example/ });
  await updated.getByRole('button', { name: 'Delete' }).click();
  await updated.getByRole('button', { name: 'Confirm' }).click();
  await expect(
    authedPage.getByRole('row', { name: /scans\.example/ }),
  ).toHaveCount(0);
});

test('server validation errors render field-level', async ({ authedPage }) => {
  await authedPage.goto('/rules');
  await authedPage.getByRole('button', { name: 'Add rule' }).click();

  // A regex that doesn't compile → field-level error on the pattern.
  await authedPage.getByLabel('Host').fill('brand-new.example');
  await authedPage.getByLabel('Chapter URL pattern').fill('([unclosed');
  await authedPage.getByRole('button', { name: 'Save rule' }).click();
  await expect(authedPage.getByLabel('Chapter URL pattern')).toHaveAttribute(
    'aria-invalid',
    'true',
  );

  // Fix the regex but collide with a seeded host → field-level error on host.
  await authedPage.getByLabel('Host').fill('wuxiaworld.com');
  await authedPage
    .getByLabel('Chapter URL pattern')
    .fill('^/x/(?P<slug>[^/]+)/(?P<chapter>[0-9]+)$');
  await authedPage.getByRole('button', { name: 'Save rule' }).click();
  await expect(authedPage.getByLabel('Host')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
});
