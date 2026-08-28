// Spawns the REAL backend binary — with the REAL web dist embedded (the
// Makefile's web-test-e2e runs web-embed + backend build first) — on a port
// that doesn't collide with the extension's e2e rig (18080/18081).
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const BACKEND_PORT = 18090;
export const SERVER_URL = `http://127.0.0.1:${String(BACKEND_PORT)}`;

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

export default async function globalSetup(): Promise<() => void> {
  const backendBin =
    process.env.NC_BACKEND_BIN ??
    path.resolve(import.meta.dirname, '../../../backend/bin/nextchapter');
  const dbDir = mkdtempSync(path.join(tmpdir(), 'nextchapter-web-e2e-'));

  const backend: ChildProcess = spawn(backendBin, [], {
    env: {
      ...process.env,
      NEXTCHAPTER_LISTEN_ADDR: `127.0.0.1:${String(BACKEND_PORT)}`,
      NEXTCHAPTER_DATABASE_URL: `sqlite://${path.join(dbDir, 'e2e.db')}`,
      NEXTCHAPTER_LOG_LEVEL: 'warn',
    },
    stdio: 'inherit',
  });
  try {
    await waitForHealthz();
  } catch (cause) {
    backend.kill('SIGTERM');
    throw cause;
  }

  process.env.NC_WEB_E2E_SERVER = SERVER_URL;

  return () => {
    backend.kill('SIGTERM');
  };
}
