import { expect, test } from './fixtures';
import { seedCapture } from './api-helpers';

const server = () => process.env.NC_WEB_E2E_SERVER ?? '';

/**
 * Covers against the embedded binary (ADR-0011). The image bytes are
 * uploaded through the real API exactly as the extension would, then the
 * grid and detail page are checked for what the user actually sees.
 */

/** A real 2x3 PNG, base64 — small enough to inline, real enough to decode. */
const PNG_2x3_BASE64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAIAAAADCAYAAAC56t6BAAAAKElEQVR4nAAbAOT/AsgoWv8AAAAAAAAAAAAAAAAAAgAAAAAoWsj/AwA8MQSXAHNISgAAAABJRU5ErkJggg==';

function pngBytes(): Buffer {
  return Buffer.from(PNG_2x3_BASE64, 'base64');
}

test('a cover uploaded through the API shows on the grid and the detail page', async ({
  authedPage,
  user,
}) => {
  const seriesId = await seedCapture(server(), user, {
    title: 'Cover Test Series',
    host: 'novels.example.com',
    slug: 'cover-test',
    chapter: 12,
  });

  // Before any upload the card falls back to the lettered placeholder,
  // and no cover request is issued for a coverless series.
  await authedPage.goto('/');
  const card = authedPage.locator('.nc-card', {
    hasText: 'Cover Test Series',
  });
  await expect(card.locator('.nc-cover-empty')).toBeVisible();
  await expect(card.locator('img.nc-cover')).toHaveCount(0);

  // Upload as the extension does: raw bytes, deliberately mislabelled
  // Content-Type, with the provenance header.
  const put = await authedPage.request.put(
    `${server()}/series/${String(seriesId)}/cover`,
    {
      headers: {
        Cookie: user.sessionCookie,
        'Content-Type': 'application/octet-stream',
        'X-Cover-Source-Url': 'https://novels.example.com/series/cover-test',
      },
      data: pngBytes(),
    },
  );
  expect(put.status(), await put.text()).toBe(200);
  const meta = (await put.json()) as { mime: string; width: number };
  expect(meta.mime).toBe('image/png');
  expect(meta.width).toBe(2);

  // The grid now renders a real <img>, cache-busted by cover_updated_at.
  await authedPage.reload();
  const cover = card.locator('img.nc-cover');
  await expect(cover).toBeVisible();
  const src = await cover.getAttribute('src');
  expect(src).toMatch(/^\/series\/\d+\/cover\?v=/);

  // And it actually loads — a broken src would leave naturalWidth at 0.
  await expect
    .poll(async () =>
      cover.evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBe(2);

  // The detail page shows the large variant and offers to remove it.
  await card.click();
  await expect(authedPage.locator('img.nc-cover-lg')).toBeVisible();

  await authedPage.getByRole('button', { name: 'Remove cover' }).click();
  await authedPage.getByRole('button', { name: 'Confirm' }).click();

  // Removing falls straight back to the placeholder, on the detail page
  // and on the grid behind it.
  await expect(authedPage.locator('.nc-cover-lg.nc-cover-empty')).toBeVisible();
  await authedPage.goto('/');
  await expect(card.locator('.nc-cover-empty')).toBeVisible();
});

test('a non-image upload is refused and leaves no cover behind', async ({
  authedPage,
  user,
}) => {
  const seriesId = await seedCapture(server(), user, {
    title: 'Rejects Junk',
    host: 'novels.example.com',
    slug: 'rejects-junk',
    chapter: 3,
  });

  const put = await authedPage.request.put(
    `${server()}/series/${String(seriesId)}/cover`,
    {
      headers: {
        Cookie: user.sessionCookie,
        // Claiming to be a PNG must not help: the server sniffs the bytes.
        'Content-Type': 'image/png',
      },
      data: '<!doctype html><html><body>not an image</body></html>',
    },
  );
  expect(put.status()).toBe(422);

  const get = await authedPage.request.get(
    `${server()}/series/${String(seriesId)}/cover`,
    { headers: { Cookie: user.sessionCookie } },
  );
  expect(get.status()).toBe(404);
});
