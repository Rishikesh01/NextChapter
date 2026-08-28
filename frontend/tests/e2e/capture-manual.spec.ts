// A page the site rule does not match: the manual form appears, the user
// supplies slug + chapter, and the capture completes through the picker.
import { expect, test } from './fixtures';

test('capture on an unmatched page falls back to the manual form', async ({
  seedSettings,
  openPopupOver,
}) => {
  await seedSettings();
  // The fake site serves this path too, but the rule only matches /series/…/chapter-…
  const popup = await openPopupOver('/read/some-novel/12');

  await expect(popup.getByText('No rule for this site')).toBeVisible();
  await popup.getByLabel('Series slug').fill('some-novel');
  await popup.getByLabel('Chapter').fill('12.5');
  await popup.getByRole('button', { name: 'Capture chapter' }).click();

  await expect(popup.getByText('Which series is this?')).toBeVisible();
  await popup.getByRole('button', { name: /Create new series/ }).click();
  await expect(popup.getByRole('status')).toHaveText(
    /Started tracking .* at ch 12\.5/,
  );
});
