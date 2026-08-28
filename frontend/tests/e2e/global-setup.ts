// Starts the REAL Go backend (SQLite in a temp dir) and a local fake chapter
// site, registers a throwaway user, mints an API token over the cookie
// channel, and installs a site rule for the fake site through the real API —
// in Go regex syntax, so the specs exercise the full translate-and-match
// pipeline. Nothing about the backend is mocked.
import { spawn, type ChildProcess } from 'node:child_process';
import { createServer, type Server } from 'node:http';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { createUserWithToken } from './api-helpers';

const BACKEND_PORT = 18080;
const SITE_PORT = 18081;

export const SERVER_URL = `http://127.0.0.1:${String(BACKEND_PORT)}`;
// The backend's host validator rejects IP literals, so the fake chapter site
// lives on "localhost" (the test-mode manifest grants both).
export const SITE_URL = `http://localhost:${String(SITE_PORT)}`;

async function waitForHealthz(): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt++) {
    try {
      const response = await fetch(`${SERVER_URL}/healthz`);
      if (response.ok) return;
    } catch {
      // not up yet
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`backend did not become healthy at ${SERVER_URL}`);
}

function startFakeSite(): Promise<Server> {
  const server = createServer((req, res) => {
    const match = /^\/series\/(?<slug>[^/]+)\/chapter-(?<chapter>[\d.]+)$/.exec(
      req.url ?? '',
    );
    const slug = match?.groups?.slug ?? 'unknown';
    const chapter = match?.groups?.chapter ?? '?';
    const pretty = slug
      .split('-')
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(
      `<!doctype html><html><head><title>${pretty} Chapter ${chapter} - Fake Reader</title></head>` +
        `<body><h1>${pretty} — chapter ${chapter}</h1></body></html>`,
    );
  });
  return new Promise((resolve) => {
    // No host: bind both stacks, whichever family "localhost" resolves to.
    server.listen(SITE_PORT, () => {
      resolve(server);
    });
  });
}

async function json(response: Response): Promise<unknown> {
  if (!response.ok) {
    throw new Error(
      `${response.url}: HTTP ${String(response.status)} ${await response.text()}`,
    );
  }
  return response.json();
}

export default async function globalSetup(): Promise<() => Promise<void>> {
  const backendBin =
    process.env.NC_BACKEND_BIN ??
    path.resolve(import.meta.dirname, '../../../backend/bin/nextchapter');
  const dbDir = mkdtempSync(path.join(tmpdir(), 'nextchapter-e2e-'));

  const backend: ChildProcess = spawn(backendBin, [], {
    env: {
      ...process.env,
      NEXTCHAPTER_LISTEN_ADDR: `127.0.0.1:${String(BACKEND_PORT)}`,
      NEXTCHAPTER_DATABASE_URL: `sqlite://${path.join(dbDir, 'e2e.db')}`,
      NEXTCHAPTER_LOG_LEVEL: 'warn',
    },
    stdio: 'inherit',
  });
  // From here on, any setup failure must not orphan the backend process.
  try {
    return await seed(backend);
  } catch (cause) {
    backend.kill('SIGTERM');
    throw cause;
  }
}

async function seed(backend: ChildProcess): Promise<() => Promise<void>> {
  await waitForHealthz();
  const site = await startFakeSite();

  const { username, token } = await createUserWithToken(SERVER_URL, 'e2e-seed');

  // Site rule for the fake site, in Go syntax — the extension must translate it.
  await json(
    await fetch(`${SERVER_URL}/sites/rules`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        host: 'localhost',
        chapter_url_regex:
          '^/series/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)$',
        slug_capture_group: 'slug',
        chapter_capture_group: 'chapter',
      }),
    }),
  );

  process.env.NC_E2E_SERVER = SERVER_URL;
  process.env.NC_E2E_SITE = SITE_URL;
  process.env.NC_E2E_TOKEN = token;
  process.env.NC_E2E_USERNAME = username;

  return async () => {
    await new Promise((resolve) => site.close(resolve));
    backend.kill('SIGTERM');
  };
}
