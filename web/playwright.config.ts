import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  globalSetup: './tests/e2e/global-setup.ts',
  // One worker: every spec shares the one backend process.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 30_000,
  reporter: [['list']],
  use: {
    // The embedded binary IS the app under test (ADR-0010): pages, assets,
    // and API all come from the one server.
    baseURL: 'http://127.0.0.1:18090',
  },
});
