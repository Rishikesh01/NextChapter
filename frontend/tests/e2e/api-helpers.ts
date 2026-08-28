// Plain-HTTP helpers against the real backend: register a user and mint an
// API token by replaying the session cookie (node's fetch has no cookie jar) —
// the same cookie channel the options page uses.

export interface SeededUser {
  username: string;
  password: string;
  token: string;
}

async function json(response: Response): Promise<unknown> {
  if (!response.ok) {
    throw new Error(
      `${response.url}: HTTP ${String(response.status)} ${await response.text()}`,
    );
  }
  return response.json();
}

export async function createUserWithToken(
  serverUrl: string,
  prefix: string,
): Promise<SeededUser> {
  const username = `${prefix}-${String(Date.now())}-${Math.random().toString(36).slice(2, 6)}`;
  const password = 'e2e-password';

  const registerResponse = await fetch(`${serverUrl}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  await json(registerResponse);
  const cookie = registerResponse.headers
    .getSetCookie()
    .map((value) => value.split(';')[0] ?? '')
    .find((value) => value.startsWith('nc_session='));
  if (cookie === undefined) throw new Error('register did not set nc_session');

  const minted = (await json(
    await fetch(`${serverUrl}/auth/tokens`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Cookie: cookie },
      body: JSON.stringify({ label: prefix }),
    }),
  )) as { token?: string };
  if (minted.token === undefined) throw new Error('mint returned no token');

  return { username, password, token: minted.token };
}

export async function apiFetch(
  serverUrl: string,
  token: string,
  method: string,
  path: string,
  body?: unknown,
): Promise<Response> {
  return fetch(`${serverUrl}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}
