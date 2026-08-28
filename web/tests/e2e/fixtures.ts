import { test as base, type Page } from '@playwright/test';
import { registerUser, type SeededUser } from './api-helpers';

export interface Fixtures {
  /** A fresh registered user for this test. */
  user: SeededUser;
  /**
   * A page already authenticated as `user`: the REAL nc_session cookie from
   * registration is planted into the browser context — nothing is mocked.
   */
  authedPage: Page;
}

export const test = base.extend<Fixtures>({
  user: async ({}, use) => {
    const serverUrl = process.env.NC_WEB_E2E_SERVER ?? '';
    await use(await registerUser(serverUrl, 'e2e-web'));
  },

  authedPage: async ({ context, page, user }, use) => {
    const serverUrl = process.env.NC_WEB_E2E_SERVER ?? '';
    const [name, value] = user.sessionCookie.split('=', 2);
    await context.addCookies([
      {
        name: name ?? 'nc_session',
        value: value ?? '',
        url: serverUrl,
        httpOnly: true,
        sameSite: 'Lax',
      },
    ]);
    await use(page);
  },
});

export { expect } from '@playwright/test';
