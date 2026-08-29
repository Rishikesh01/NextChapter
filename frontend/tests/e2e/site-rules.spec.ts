// Rule management end to end (ADR-0009): build a rule from the popup without
// touching a regex, capture in the same click, and get auto-detection on the
// next visit; view and delete rules from the options page.
import { expect, test } from './fixtures';
import { apiFetch, createUserWithToken } from './api-helpers';

test('build a rule from the popup, capture, and auto-detect next time', async ({
  seedSettings,
  openPopupOver,
}) => {
  // A fresh user: seeded defaults only, no rule for the fake site.
  const user = await createUserWithToken(
    process.env.NC_E2E_SERVER ?? '',
    'e2e-rules',
  );
  await seedSettings({ apiToken: user.token, username: user.username });

  const popup = await openPopupOver('/read/fresh-novel/7');
  await expect(popup.getByText('No rule for this site')).toBeVisible();
  await popup
    .getByRole('button', { name: 'Create a rule from this page' })
    .click();

  // The guess pre-selected slug + chapter; the preview shows what the rule detects.
  await expect(
    popup.getByRole('radio', { name: 'Series name part: fresh-novel' }),
  ).toBeChecked();
  await expect(
    popup.getByRole('radio', { name: 'Chapter part: 7' }),
  ).toBeChecked();
  await popup.getByRole('button', { name: 'Save rule & capture' }).click();

  // First capture for the key → picker → created.
  await expect(popup.getByText('Which series is this?')).toBeVisible();
  await popup.getByRole('button', { name: /Create new series/ }).click();
  await expect(popup.getByRole('status')).toHaveText(
    /Started tracking .* at ch 7/,
  );

  // The saved rule (cache refreshed, no TTL wait) now auto-detects chapter 8.
  const popup2 = await openPopupOver('/read/fresh-novel/8');
  await expect(
    popup2.getByRole('heading', { name: 'Fresh Novel' }),
  ).toBeVisible();
  await expect(popup2.getByLabel('Chapter')).toHaveValue('8');
  await popup2.getByRole('button', { name: 'Capture chapter' }).click();
  await expect(popup2.getByRole('status')).toHaveText(/Advanced .* to ch 8/);
});

test('options page lists the rules and deletes one behind the inline confirm', async ({
  context,
  extensionId,
  seedSettings,
}) => {
  const serverUrl = process.env.NC_E2E_SERVER ?? '';
  const user = await createUserWithToken(serverUrl, 'e2e-ruledel');
  // Deleting one rule and asserting the other survives needs two. Registration
  // seeds one, so this creates the second explicitly rather than relying on the
  // seed list's contents — the subject here is the delete flow, not the seeds.
  await apiFetch(serverUrl, user.token, 'POST', '/sites/rules', {
    host: 'comics.example.org',
    chapter_url_regex:
      '^/comic/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)$',
    slug_capture_group: 'slug',
    chapter_capture_group: 'chapter',
  });
  await seedSettings({ apiToken: user.token, username: user.username });

  const page = await context.newPage();
  await page.goto(`chrome-extension://${extensionId}/options.html`);
  await expect(page.getByRole('heading', { name: 'Site rules' })).toBeVisible();
  await expect(page.getByText('wuxiaworld.com')).toBeVisible();
  await expect(page.getByText('comics.example.org')).toBeVisible();

  await page
    .getByRole('button', { name: 'Delete rule for comics.example.org' })
    .click();
  await expect(page.getByText('Delete rule?')).toBeVisible();
  await page.getByRole('button', { name: 'Confirm' }).click();

  await expect(page.getByText('comics.example.org')).toHaveCount(0);
  await expect(page.getByText('wuxiaworld.com')).toBeVisible();
});

// The stale-cache race (design/flows/capture.md §5a): the popup's fresh cache
// says "no rule for this host" but the server already has one. Saving from the
// builder gets a duplicate-host 422; the popup must refetch and recover.
test('duplicate-host 422: recovers by capturing with the server rule', async ({
  seedSettings,
  seedRulesCache,
  openPopupOver,
}) => {
  const serverUrl = process.env.NC_E2E_SERVER ?? '';
  const user = await createUserWithToken(serverUrl, 'e2e-race');
  // The server already has a localhost rule that matches /read pages…
  await apiFetch(serverUrl, user.token, 'POST', '/sites/rules', {
    host: 'localhost',
    chapter_url_regex:
      '^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9]+(?:\\.[0-9]+)?)$',
    slug_capture_group: 'slug',
    chapter_capture_group: 'chapter',
  });
  await seedSettings({ apiToken: user.token, username: user.username });
  // …but the extension's fresh-stamped cache doesn't know it.
  await seedRulesCache([]);

  const popup = await openPopupOver('/read/race-novel/5');
  await expect(popup.getByText('No rule for this site')).toBeVisible();
  await popup
    .getByRole('button', { name: 'Create a rule from this page' })
    .click();
  await popup.getByRole('button', { name: 'Save rule & capture' }).click();

  // 422 → refetch → the server's rule detects this page → capture continues.
  await expect(popup.getByText('Which series is this?')).toBeVisible();
  await popup.getByRole('button', { name: /Create new series/ }).click();
  await expect(popup.getByRole('status')).toHaveText(
    /Started tracking .* at ch 5/,
  );
});

test("duplicate-host 422: falls back to manual with a banner when the server's rule doesn't match", async ({
  seedSettings,
  seedRulesCache,
  openPopupOver,
}) => {
  const serverUrl = process.env.NC_E2E_SERVER ?? '';
  const user = await createUserWithToken(serverUrl, 'e2e-race2');
  // A localhost rule exists, but for a different path shape than this page.
  await apiFetch(serverUrl, user.token, 'POST', '/sites/rules', {
    host: 'localhost',
    chapter_url_regex: '^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+)$',
    slug_capture_group: 'slug',
    chapter_capture_group: 'chapter',
  });
  await seedSettings({ apiToken: user.token, username: user.username });
  await seedRulesCache([]);

  const popup = await openPopupOver('/read/other-novel/3');
  await popup
    .getByRole('button', { name: 'Create a rule from this page' })
    .click();
  await popup.getByRole('button', { name: 'Save rule & capture' }).click();

  // The builder closes back to the manual form under the explanatory banner.
  await expect(popup.getByRole('alert')).toHaveText(
    /A rule for this site already exists — manage it in settings\./,
  );
  await expect(popup.getByText('No rule for this site')).toBeVisible();
});
