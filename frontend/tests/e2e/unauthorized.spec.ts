// A revoked/garbage token: the capture attempt surfaces the auth error state
// with a route back to settings.
import { expect, test } from './fixtures';

test('a rejected token shows the reconfigure banner', async ({
  seedSettings,
  openPopupOver,
}) => {
  await seedSettings({ apiToken: 'nca_revoked', username: 'ghost' });

  const popup = await openPopupOver('/series/solo-leveling/chapter-102');

  // The rules fetch 401s too, so no rule cache exists → manual form.
  await expect(popup.getByText('No rule for this site')).toBeVisible();
  await popup.getByLabel('Series slug').fill('solo-leveling');
  await popup.getByLabel('Chapter').fill('102');
  await popup.getByRole('button', { name: 'Capture chapter' }).click();

  await expect(popup.getByRole('alert')).toHaveText(/Token rejected/);
  await expect(
    popup.getByRole('button', { name: 'open settings' }),
  ).toBeVisible();
});
