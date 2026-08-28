// Plain-HTTP helpers against the real backend. A small deliberate duplicate
// of frontend/tests/e2e/api-helpers.ts so the two test rigs stay decoupled;
// extract to a shared package if a third rig ever appears.

export interface SeededUser {
  username: string;
  password: string;
  /** The raw nc_session cookie value from registration ("nc_session=ncs_…"). */
  sessionCookie: string;
}

async function json(response: Response): Promise<unknown> {
  if (!response.ok) {
    throw new Error(
      `${response.url}: HTTP ${String(response.status)} ${await response.text()}`,
    );
  }
  return response.json();
}

export async function registerUser(
  serverUrl: string,
  prefix: string,
): Promise<SeededUser> {
  const username = `${prefix}-${String(Date.now())}-${Math.random().toString(36).slice(2, 6)}`;
  const password = 'e2e-password';

  const response = await fetch(`${serverUrl}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  await json(response);
  const sessionCookie = response.headers
    .getSetCookie()
    .map((value) => value.split(';')[0] ?? '')
    .find((value) => value.startsWith('nc_session='));
  if (sessionCookie === undefined)
    throw new Error('register did not set nc_session');

  return { username, password, sessionCookie };
}

/** Cookie-authenticated request, replaying the registration session. */
export async function apiFetch(
  serverUrl: string,
  user: SeededUser,
  method: string,
  path: string,
  body?: unknown,
): Promise<unknown> {
  const response = await fetch(`${serverUrl}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Cookie: user.sessionCookie,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (response.status === 204) return undefined;
  return json(response);
}

/**
 * Seed one captured entry. Pass `seriesId` to add a second site to an
 * existing series (a bare title would create a duplicate); returns the
 * entry's series id for exactly that chaining.
 */
export async function seedCapture(
  serverUrl: string,
  user: SeededUser,
  input: {
    title: string;
    host: string;
    slug: string;
    chapter: number;
    seriesId?: number;
  },
): Promise<number> {
  const entry = (await apiFetch(serverUrl, user, 'POST', '/entries/capture', {
    site_host: input.host,
    series_slug: input.slug,
    site_title: `${input.title} Chapter ${String(input.chapter)}`,
    chapter: input.chapter,
    url: `https://${input.host}/s/${input.slug}/chapter-${String(input.chapter)}`,
    ...(input.seriesId !== undefined
      ? { series_id: input.seriesId }
      : { new_series_title: input.title }),
  })) as { series_id?: number };
  if (entry.series_id === undefined)
    throw new Error('capture returned no series_id');
  return entry.series_id;
}
