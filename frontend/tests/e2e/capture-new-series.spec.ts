// First capture for a (host, slug): optimistic capture comes back 422
// needs-series, the picker appears, creating a series completes with 201.
import { expect, test } from './fixtures';

test('capture on a rule-matched page: detect, pick-series, create (201)', async ({
  seedSettings,
  openPopupOver,
}) => {
  await seedSettings();
  const popup = await openPopupOver('/series/solo-leveling/chapter-101');

  // The Go-syntax rule matched and prefilled everything.
  await expect(
    popup.getByRole('heading', { name: 'Solo Leveling' }),
  ).toBeVisible();
  await expect(popup.getByLabel('Chapter')).toHaveValue('101');

  await popup.getByRole('button', { name: 'Capture chapter' }).click();

  // No entry exists yet → series picker.
  await expect(popup.getByText('Which series is this?')).toBeVisible();
  await popup.getByRole('button', { name: /Create new series/ }).click();

  await expect(popup.getByRole('status')).toHaveText(
    /Started tracking .* at ch 101/,
  );
});
