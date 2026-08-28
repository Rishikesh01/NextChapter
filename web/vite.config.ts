import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Dev-only proxy: the SPA and the API share an origin in dev exactly as they
// do in production (the SPA is embedded and served by the Go binary), so the
// nc_session cookie works with zero configuration (ADR-0010 §7). This is
// build tooling, not application code — the app itself only ever talks to
// window.location.origin.
const API_PROXY_TARGET = 'http://127.0.0.1:8080';
const API_PREFIXES = [
  '/auth',
  '/series',
  '/entries',
  '/sites',
  '/healthz',
  '/swagger',
];

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: Object.fromEntries(
      API_PREFIXES.map((prefix) => [prefix, { target: API_PROXY_TARGET }]),
    ),
  },
});
