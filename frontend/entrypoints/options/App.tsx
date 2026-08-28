import { useEffect, useState } from 'react';
import { browser } from 'wxt/browser';
import { ApiError } from '@nextchapter/api-client';
import { apiClientFor } from '../../lib/api';
import { clearSettings, getSettings, setSettings } from '../../lib/storage';
import { ServerStep } from '../../components/options/ServerStep';
import { SignInStep } from '../../components/options/SignInStep';
import { PasteTokenStep } from '../../components/options/PasteTokenStep';
import { ConnectedCard } from '../../components/options/ConnectedCard';
import { StatusBanner } from '../../components/popup/StatusBanner';
import type { ConnectionState } from '../../components/options/ConnectionStatus';

/**
 * Chrome and Firefox match patterns cannot carry a port, and a portless
 * pattern matches every port — so request the host, not the origin.
 */
function permissionPatternFor(origin: string): string {
  const url = new URL(origin);
  return `${url.protocol}//${url.hostname}/*`;
}

export function App() {
  const [connected, setConnected] = useState<{
    username: string;
    serverUrl: string;
  } | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [serverUrl, setServerUrl] = useState('');
  const [origin, setOrigin] = useState<string | null>(null);
  const [connState, setConnState] = useState<ConnectionState>('unchecked');
  const [connDetail, setConnDetail] = useState<string>();
  const [tab, setTab] = useState<'signin' | 'token'>('signin');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void getSettings().then((settings) => {
      if (settings !== undefined) {
        setConnected({
          username: settings.username,
          serverUrl: settings.serverUrl,
        });
        setServerUrl(settings.serverUrl);
      }
      setLoaded(true);
    });
  }, []);

  // The Connect click is the user gesture that may show the browser's
  // host-permission prompt — everything here stays inside that handler.
  const connect = () => {
    setError(null);
    let parsed: URL;
    try {
      parsed = new URL(serverUrl.trim());
    } catch {
      setConnState('bad');
      setConnDetail('enter a full URL like https://nextchapter.example.com');
      return;
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      setConnState('bad');
      setConnDetail('the URL must start with http:// or https://');
      return;
    }
    const target = parsed.origin;
    setConnState('checking');
    setConnDetail(undefined);

    void (async () => {
      const pattern = { origins: [permissionPatternFor(target)] };
      const granted =
        (await browser.permissions.contains(pattern)) ||
        (await browser.permissions.request(pattern));
      if (!granted) {
        setConnState('bad');
        setConnDetail('the extension needs permission to reach this server');
        return;
      }
      try {
        await apiClientFor(target).health();
      } catch {
        setConnState('bad');
        setConnDetail('check the URL and that the server is running');
        return;
      }
      setServerUrl(target);
      setOrigin(target);
      setConnState('ok');
    })();
  };

  const finishOnboarding = async (
    target: string,
    token: string,
    fallbackUsername: string,
  ) => {
    const user = await apiClientFor(target, token).me();
    const username = user.username ?? fallbackUsername;
    await setSettings({ serverUrl: target, apiToken: token, username });
    setConnected({ username, serverUrl: target });
  };

  const signIn = (
    username: string,
    password: string,
    createAccount: boolean,
  ) => {
    if (origin === null) return;
    setBusy(true);
    setError(null);
    void (async () => {
      const client = apiClientFor(origin);
      try {
        if (createAccount) {
          await client.auth.register({ username, password });
        } else {
          await client.auth.login({ username, password });
        }
      } catch (err) {
        setError(
          err instanceof ApiError && err.fields !== undefined
            ? Object.entries(err.fields)
                .map(([field, message]) => `${field}: ${message}`)
                .join('; ')
            : err instanceof ApiError
              ? err.message
              : String(err),
        );
        setBusy(false);
        return;
      }

      try {
        const label = `extension ${new Date().toISOString().slice(0, 10)}`;
        const minted = await client.auth.mintToken({ label });
        if (minted.token === undefined || minted.token === '') {
          throw new ApiError(0, undefined, 'server returned no token');
        }
        await finishOnboarding(origin, minted.token, username);
      } catch {
        // The cookie channel can fail in some browser configurations
        // (ADR-0008 §6) — degrade to the paste-token path, never dead-end.
        setTab('token');
        setError(
          'Signed in, but minting a token from the extension failed. Create one from the link below and paste it here.',
        );
      } finally {
        await client.auth.logout().catch(() => undefined);
        setBusy(false);
      }
    })();
  };

  const pasteToken = (token: string) => {
    if (origin === null) return;
    setBusy(true);
    setError(null);
    void finishOnboarding(origin, token, 'you')
      .catch((err: unknown) => {
        setError(
          err instanceof ApiError && err.unauthorized
            ? 'The server rejected this token — check it was pasted completely.'
            : err instanceof ApiError
              ? err.message
              : String(err),
        );
      })
      .finally(() => {
        setBusy(false);
      });
  };

  const disconnect = () => {
    void clearSettings().then(() => {
      setConnected(null);
      setOrigin(null);
      setConnState('unchecked');
      setTab('signin');
    });
  };

  if (!loaded) return null;

  return (
    <main className="nc-options">
      <h1 className="nc-options-title">NextChapter settings</h1>
      <p className="nc-options-sub">
        Connect this extension to your self-hosted NextChapter server.
      </p>

      {error !== null && (
        <div className="nc-options-banner">
          <StatusBanner kind="error">{error}</StatusBanner>
        </div>
      )}

      {connected !== null ? (
        <ConnectedCard
          username={connected.username}
          serverUrl={connected.serverUrl}
          onDisconnect={disconnect}
        />
      ) : (
        <>
          <ServerStep
            url={serverUrl}
            state={connState}
            stateDetail={connDetail}
            onUrlChange={setServerUrl}
            onConnect={connect}
          />
          <section className="nc-section" aria-disabled={connState !== 'ok'}>
            <h2 className="nc-section-title">
              <span className="nc-step">2.</span> Account
            </h2>
            {connState !== 'ok' ? (
              <p className="nc-small">Connect to your server first.</p>
            ) : (
              <>
                <div
                  className="nc-tabs"
                  role="tablist"
                  aria-label="Authentication method"
                >
                  <button
                    className="nc-tab"
                    role="tab"
                    aria-selected={tab === 'signin'}
                    type="button"
                    onClick={() => {
                      setTab('signin');
                    }}
                  >
                    Sign in
                  </button>
                  <button
                    className="nc-tab"
                    role="tab"
                    aria-selected={tab === 'token'}
                    type="button"
                    onClick={() => {
                      setTab('token');
                    }}
                  >
                    Paste token
                  </button>
                </div>
                {tab === 'signin' ? (
                  <SignInStep busy={busy} onSubmit={signIn} />
                ) : (
                  <PasteTokenStep
                    serverUrl={origin ?? ''}
                    busy={busy}
                    onSubmit={pasteToken}
                  />
                )}
              </>
            )}
          </section>
        </>
      )}
    </main>
  );
}
