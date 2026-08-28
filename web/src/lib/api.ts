// The web app's single API client: cookie auth mode against the page's own
// origin (same-origin in production via the embedded build, and in dev via
// the Vite proxy — ADR-0010 §2/§7). No URLs are hardcoded anywhere else.
import { createApiClient } from '@nextchapter/api-client';

export const api = createApiClient(() =>
  Promise.resolve({
    baseUrl: window.location.origin,
    authMode: 'cookie' as const,
  }),
);
