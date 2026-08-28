import { useState } from 'react';
import { useNavigate } from 'react-router';
import type { APIToken } from '@nextchapter/api-client';
import { useLogout, useMe, useMintToken } from '../lib/queries';
import { ErrorBanner } from '../components/ErrorBanner';

export function SettingsPage() {
  const me = useMe();
  const mint = useMintToken();
  const logout = useLogout();
  const navigate = useNavigate();

  const [label, setLabel] = useState('');
  const [minted, setMinted] = useState<APIToken | null>(null);
  const [copied, setCopied] = useState(false);

  const createToken = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    mint.mutate(
      { label: label.trim() === '' ? 'extension' : label.trim() },
      {
        onSuccess: (token) => {
          setMinted(token);
          setLabel('');
        },
      },
    );
  };

  const copy = () => {
    if (minted?.token === undefined) return;
    void navigator.clipboard.writeText(minted.token).then(() => {
      setCopied(true);
      setTimeout(() => {
        setCopied(false);
      }, 2000);
    });
  };

  return (
    <>
      <h1 className="nc-page-title">Settings</h1>

      <section className="nc-section">
        <h2 className="nc-section-title">Extension token</h2>
        {mint.isError && <ErrorBanner>{mint.error.message}</ErrorBanner>}
        {minted === null ? (
          <>
            <p className="nc-section-caption nc-small">
              The browser extension authenticates with an API token. Name it
              after where it lives — the token itself can never be shown again.
            </p>
            <form onSubmit={createToken}>
              <div className="nc-row">
                <input
                  className="nc-input"
                  type="text"
                  placeholder="e.g. laptop-firefox"
                  aria-label="Token label"
                  autoComplete="off"
                  spellCheck={false}
                  value={label}
                  onChange={(event) => {
                    setLabel(event.target.value);
                  }}
                />
                <button
                  className="nc-btn-primary"
                  type="submit"
                  disabled={mint.isPending}
                >
                  {mint.isPending ? 'Creating…' : 'Create token'}
                </button>
              </div>
            </form>
          </>
        ) : (
          <>
            <p className="nc-section-caption nc-small">
              Token <strong>{minted.label}</strong> created.
            </p>
            <div className="nc-field" style={{ marginBottom: 0 }}>
              <label htmlFor="minted-token">Your new token</label>
              <div className="nc-row">
                <input
                  className="nc-input nc-input-token"
                  id="minted-token"
                  type="text"
                  readOnly
                  value={minted.token ?? ''}
                  spellCheck={false}
                  onClick={(event) => {
                    event.currentTarget.select();
                  }}
                />
                <button
                  className="nc-btn-secondary"
                  type="button"
                  onClick={copy}
                >
                  {copied ? 'Copied' : 'Copy'}
                </button>
              </div>
              <p className="nc-token-warn" role="alert">
                Save it now — it won&rsquo;t be shown again.
              </p>
              <p className="nc-token-note nc-small">
                Paste it into the extension&rsquo;s settings (&ldquo;Paste
                token&rdquo; tab).
              </p>
            </div>
            <div
              className="nc-form-actions"
              style={{ marginTop: 'var(--nc-space-3)' }}
            >
              <button
                className="nc-btn-secondary"
                type="button"
                onClick={() => {
                  setMinted(null);
                }}
              >
                Done
              </button>
            </div>
          </>
        )}
      </section>

      <section className="nc-section">
        <h2 className="nc-section-title">Account</h2>
        <div className="nc-account-row">
          <p>
            Signed in as <strong>{me.data?.username}</strong>
          </p>
          <button
            className="nc-btn-secondary nc-btn-danger-quiet"
            type="button"
            onClick={() => {
              logout.mutate(undefined, {
                onSettled: () => {
                  void navigate('/login');
                },
              });
            }}
          >
            Sign out
          </button>
        </div>
      </section>
    </>
  );
}
