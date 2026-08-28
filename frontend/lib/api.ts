// The only place the server URL and token are read for API calls.
import { createApiClient, type ApiClient } from '@nextchapter/api-client';
import { getSettings } from './storage';

/** Client bound to the saved extension settings (popup, post-onboarding). */
export function extensionApiClient(): ApiClient {
  return createApiClient(async () => {
    const settings = await getSettings();
    return { baseUrl: settings?.serverUrl ?? '', token: settings?.apiToken };
  });
}

/**
 * Client for an explicit server/token pair — used by the options page during
 * onboarding, before anything is saved to storage.
 */
export function apiClientFor(baseUrl: string, token?: string): ApiClient {
  return createApiClient(() => Promise.resolve({ baseUrl, token }));
}
