import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import {
  test as base,
  chromium,
  type BrowserContext,
  type Page,
} from '@playwright/test';

const EXT_PATH = path.resolve(
  import.meta.dirname,
  '../../.output/chrome-mv3-test',
);

// Present on extension pages, where seedSettings' evaluate callback runs.
declare const chrome: {
  storage: {
    local: { set: (items: Record<string, unknown>) => Promise<void> };
  };
};

/** Chrome derives the unpacked-extension ID from the manifest `key`: the first
 * 16 bytes of SHA-256(DER public key), each nibble mapped onto a–p. */
export function extensionIdFromKey(keyB64: string): string {
  const hash = createHash('sha256')
    .update(Buffer.from(keyB64, 'base64'))
    .digest();
  let id = '';
  for (const byte of hash.subarray(0, 16)) {
    id +=
      String.fromCharCode(97 + (byte >> 4)) +
      String.fromCharCode(97 + (byte & 15));
  }
  return id;
}

export interface SeededSettings {
  serverUrl: string;
  apiToken: string;
  username: string;
}

export interface SeededRule {
  host: string;
  chapter_url_regex: string;
  slug_capture_group: string;
  chapter_capture_group: string;
}

export interface Fixtures {
  context: BrowserContext;
  extensionId: string;
  /**
   * Seed the extension's storage with a working server+token config; pass
   * overrides to seed a broken one (e.g. a revoked token).
   */
  seedSettings: (overrides?: Partial<SeededSettings>) => Promise<void>;
  /**
   * Plant a FRESH-stamped rules cache with exactly these rules — the tool for
   * staging a stale-cache race (a fresh cache is never refetched on popup
   * open, so the popup believes it over the server).
   */
  seedRulesCache: (rules: SeededRule[]) => Promise<void>;
  /** Open a fake-site chapter page and the popup so the popup "sees" it. */
  openPopupOver: (sitePath: string) => Promise<Page>;
}

export const test = base.extend<Fixtures>({
  context: async ({}, use) => {
    const context = await chromium.launchPersistentContext('', {
      channel: 'chromium',
      args: [
        `--disable-extensions-except=${EXT_PATH}`,
        `--load-extension=${EXT_PATH}`,
      ],
    });
    await use(context);
    await context.close();
  },

  extensionId: async ({}, use) => {
    const manifest = JSON.parse(
      readFileSync(path.join(EXT_PATH, 'manifest.json'), 'utf8'),
    ) as {
      key?: string;
    };
    if (manifest.key === undefined)
      throw new Error('test build manifest has no key');
    await use(extensionIdFromKey(manifest.key));
  },

  seedSettings: async ({ context, extensionId }, use) => {
    await use(async (overrides?: Partial<SeededSettings>) => {
      const settings: SeededSettings = {
        serverUrl: process.env.NC_E2E_SERVER ?? '',
        apiToken: process.env.NC_E2E_TOKEN ?? '',
        username: process.env.NC_E2E_USERNAME ?? '',
        ...overrides,
      };
      const page = await context.newPage();
      await page.goto(`chrome-extension://${extensionId}/options.html`);
      await page.evaluate(async (value) => {
        // Runs inside the extension page — chrome.* is the real API here.
        // The key mirrors SETTINGS_KEY in lib/storage.ts.
        await chrome.storage.local.set({ 'settings:v1': value });
      }, settings);
      await page.close();
    });
  },

  seedRulesCache: async ({ context, extensionId }, use) => {
    await use(async (rules: SeededRule[]) => {
      const page = await context.newPage();
      await page.goto(`chrome-extension://${extensionId}/options.html`);
      await page.evaluate(
        async (cache) => {
          // The key mirrors RULES_KEY in lib/storage.ts.
          await chrome.storage.local.set({ 'siteRules:v1': cache });
        },
        { rules, trackedHosts: [], fetchedAt: Date.now() },
      );
      await page.close();
    });
  },

  openPopupOver: async ({ context, extensionId }, use) => {
    await use(async (sitePath: string) => {
      const sitePage = await context.newPage();
      await sitePage.goto(`${process.env.NC_E2E_SITE ?? ''}${sitePath}`);
      const popup = await context.newPage();
      await popup.goto(`chrome-extension://${extensionId}/popup.html`);
      // The real popup is never a tab; here it is, which steals "active" from
      // the chapter tab. Hand focus back, then remount the popup so its
      // active-tab query resolves to the chapter page (lib/tabs.ts).
      await sitePage.bringToFront();
      await popup.reload();
      return popup;
    });
  },
});

export { expect } from '@playwright/test';
