import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  globalSetup: './tests/e2e/global-setup.ts',
  // One worker: every spec shares the one backend process, and extension
  // windows fight over focus when parallel.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 30_000,
  reporter: [['list']],
});
