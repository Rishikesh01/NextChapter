import { useId, useState } from 'react';

export interface PasteTokenStepProps {
  /** Server origin, used to link to its swagger UI. */
  serverUrl: string;
  busy: boolean;
  onSubmit: (token: string) => void;
}

/** Fallback path: the user pastes a token minted elsewhere (e.g. via swagger). */
export function PasteTokenStep({
  serverUrl,
  busy,
  onSubmit,
}: PasteTokenStepProps) {
  const id = useId();
  const [token, setToken] = useState('');
  const swaggerUrl = `${serverUrl}/swagger/index.html`;
  const swaggerLabel = `${serverUrl.replace(/^https?:\/\//, '')}/swagger`;

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (token.trim() === '') return;
    onSubmit(token.trim());
  };

  return (
    <form onSubmit={submit}>
      <div className="nc-field">
        <label htmlFor={id}>API token</label>
        <input
          className="nc-input nc-input-token"
          id={id}
          type="password"
          placeholder="paste your token"
          autoComplete="off"
          spellCheck={false}
          value={token}
          onChange={(event) => {
            setToken(event.target.value);
          }}
        />
        <p className="nc-help nc-small">
          Create one from your server’s API docs at{' '}
          <a href={swaggerUrl}>{swaggerLabel}</a> (
          <code>POST /auth/tokens</code>), then paste it here.
        </p>
      </div>
      <button
        className="nc-btn-primary nc-btn-primary-block"
        type="submit"
        disabled={busy}
      >
        {busy ? 'Verifying…' : 'Save token'}
      </button>
    </form>
  );
}
