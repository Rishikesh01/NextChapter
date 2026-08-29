import { defineConfig } from 'wxt';

// DER-encoded RSA public key. Pins the unpacked Chromium extension ID to a
// deterministic value — Playwright addresses chrome-extension://<id>/... pages
// with it, and self-hosted unpacked installs keep a stable ID. Ships in every
// Chromium build on purpose; public key only, the matching private key was
// discarded (ADR-0008 §5).
const CHROMIUM_ID_KEY =
  'MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA6PV5XeHfl4sZvPFYdt0bMLZ8wANKnS2XTek0CysHq34rAyckPX0LQIVxMSi9kJ8BaUFcd+QWTmDv5+zRFt0Vy6f6reHf0QTGiTnP0PdpZe1qMQ+uYzLkc2wLSU38LTvYolRRVRTRS9didEBy7ABbz2B9L0j5Cs/A34Ahgyovxgs7d15eVgpIO/o98R8ryQpGb7lgqisVyctmhJ5KIDE/9TrbQczplhJ0BH7TPMeDMutXFLO3w5rm3aZMsqrhxIZgJNRws637oCm+PB7QdxauIWPUHo+ZqUHWhBMaH3hHAjYyTaJxYZ+Mh4pR26b/zs4KFqr/OIv5hnpnUoHjFDjncwIDAQAB';

// Release builds stamp the manifest from the git tag: the repository-root
// Makefile exports NEXTCHAPTER_VERSION into every `wxt build`/`wxt zip`.
// Both stores accept only 1-4 dot-separated integers for `version`, so a
// tag's `v` prefix and any prerelease/build metadata is stripped. The full
// tag survives in `version_name`, which is Chromium-only — Firefox's
// linter rejects the key. Anything that does not reduce to a valid version
// ("dev", a bare commit sha, unset) falls through to WXT's default: the
// version in package.json.
const releaseVersion = process.env.NEXTCHAPTER_VERSION?.trim() ?? '';

function toManifestVersion(raw: string): string | undefined {
  const core = raw.replace(/^v/, '').split(/[-+]/)[0] ?? '';
  return /^\d+(?:\.\d+){0,3}$/.test(core) ? core : undefined;
}

const stampedVersion = toManifestVersion(releaseVersion);

export default defineConfig({
  modules: ['@wxt-dev/module-react'],
  // Explicit imports keep eslint and depcheck honest (ADR-0008 §2).
  imports: false,
  // WXT defaults Firefox to MV2, which silently drops
  // optional_host_permissions and breaks onboarding — MV3 everywhere,
  // for every command (build, dev, zip).
  manifestVersion: 3,
  manifest: ({ browser, mode }) => ({
    name: 'NextChapter',
    description: 'Capture your reading position on the current chapter page.',
    // Present only on release builds; see toManifestVersion above.
    ...(stampedVersion ? { version: stampedVersion } : {}),
    ...(stampedVersion &&
    browser === 'chrome' &&
    releaseVersion !== stampedVersion
      ? { version_name: releaseVersion }
      : {}),
    permissions: [
      // activeTab: read the invoked tab's URL/title at the moment the user
      // clicks the toolbar button — the product's single opt-in capture
      // interaction. Avoids the broad `tabs` permission.
      'activeTab',
      // storage: server settings, API token, and the site-rule cache.
      'storage',
      // scripting: read the page's cover art on an explicit user gesture
      // (ADR-0011 §7). activeTab already grants the host access this needs;
      // `scripting` only unlocks the injection API itself, so this adds no
      // new install-time permission warning and nothing runs unprompted.
      'scripting',
    ],
    // The server URL is user-configured, so no concrete host can be listed at
    // install time. The options page requests exactly the user's server origin
    // at connect time; the grant exempts fetches to that one origin from CORS
    // and SameSite blocking (ADR-0008 §5).
    optional_host_permissions: ['http://*/*', 'https://*/*'],
    ...(browser === 'chrome' ? { key: CHROMIUM_ID_KEY } : {}),
    ...(browser === 'firefox'
      ? {
          browser_specific_settings: {
            // optional_host_permissions requires Firefox >= 128 (ADR-0008 §5).
            gecko: {
              id: 'nextchapter@self-hosted',
              strict_min_version: '128.0',
            },
          },
        }
      : {}),
    // Test builds only: install-time grants so e2e never hits a native
    // permission prompt Playwright cannot click. Never in production builds.
    ...(mode === 'test'
      ? { host_permissions: ['http://localhost/*', 'http://127.0.0.1/*'] }
      : {}),
  }),
  // Deterministic zip names: `make dist-extension` renames these into
  // dist/ with the release version, so the templates must not depend on a
  // version that is only present on stamped builds.
  zip: {
    artifactTemplate: '{{browser}}-mv3{{modeSuffix}}.zip',
    sourcesTemplate: 'firefox-mv3-sources{{modeSuffix}}.zip',
    // AMO reviewers must be able to rebuild from the sources zip, and the
    // extension imports @nextchapter/api-client (workspace:*) — so the
    // sources root is the pnpm workspace root, not frontend/.
    sourcesRoot: '..',
    includeSources: [
      'frontend/**',
      'packages/**',
      'design/tokens.css',
      'package.json',
      'pnpm-workspace.yaml',
      'pnpm-lock.yaml',
      'tsconfig.base.json',
      'eslint.config.js',
      'LICENSE',
      'README.md',
    ],
    excludeSources: [
      '**/node_modules/**',
      '**/.output/**',
      '**/.wxt/**',
      '**/dist/**',
      '**/test-results/**',
      '**/playwright-report/**',
      '**/blob-report/**',
      'frontend/tests/**',
      '**/*.db',
    ],
  },
});
