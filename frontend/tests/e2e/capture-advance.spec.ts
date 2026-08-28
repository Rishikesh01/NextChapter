// Advancing an existing entry: the upsert returns 200 and the popup reports
// "Advanced … to ch N" with no series picker involved.
import { expect, test } from './fixtures';

test('capture on an already-tracked slug advances the entry (200)', async ({
  seedSettings,
  openPopupOver,
}) => {
  // Pre-create the entry through the real API, as an earlier capture would.
  const response = await fetch(
    `${process.env.NC_E2E_SERVER ?? ''}/entries/capture`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${process.env.NC_E2E_TOKEN ?? ''}`,
      },
      body: JSON.stringify({
        site_host: 'localhost',
        series_slug: 'advance-series',
        site_title: 'Advance Series Chapter 100',
        chapter: 100,
        url: `${process.env.NC_E2E_SITE ?? ''}/series/advance-series/chapter-100`,
        new_series_title: 'Advance Series',
      }),
    },
  );
  if (response.status !== 201)
    throw new Error(`seed capture failed: ${String(response.status)}`);

  await seedSettings();
  const popup = await openPopupOver('/series/advance-series/chapter-101');

  await expect(popup.getByLabel('Chapter')).toHaveValue('101');
  await popup.getByRole('button', { name: 'Capture chapter' }).click();

  await expect(popup.getByRole('status')).toHaveText(/Advanced .* to ch 101/);
});
