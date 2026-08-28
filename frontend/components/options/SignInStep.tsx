import { useId, useState } from 'react';

export interface SignInStepProps {
  busy: boolean;
  onSubmit: (
    username: string,
    password: string,
    createAccount: boolean,
  ) => void;
}

/**
 * Username + password form; the toggle switches the same form to registration.
 * Signing in mints an extension token automatically — the user never sees it
 * on this path (design/flows/onboarding.md).
 */
export function SignInStep({ busy, onSubmit }: SignInStepProps) {
  const usernameId = useId();
  const passwordId = useId();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [createAccount, setCreateAccount] = useState(false);

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (username.trim() === '' || password === '') return;
    onSubmit(username.trim(), password, createAccount);
  };

  return (
    <form onSubmit={submit}>
      <div className="nc-field">
        <label htmlFor={usernameId}>Username</label>
        <input
          className="nc-input"
          id={usernameId}
          type="text"
          autoComplete="username"
          spellCheck={false}
          value={username}
          onChange={(event) => {
            setUsername(event.target.value);
          }}
        />
      </div>
      <div className="nc-field">
        <label htmlFor={passwordId}>Password</label>
        <input
          className="nc-input"
          id={passwordId}
          type="password"
          autoComplete={createAccount ? 'new-password' : 'current-password'}
          value={password}
          onChange={(event) => {
            setPassword(event.target.value);
          }}
        />
      </div>
      <button
        className="nc-btn-primary nc-btn-primary-block"
        type="submit"
        disabled={busy}
      >
        {busy ? 'Signing in…' : createAccount ? 'Create account' : 'Sign in'}
      </button>
      <p className="nc-auth-toggle">
        <button
          className="nc-btn-link"
          type="button"
          onClick={() => {
            setCreateAccount((current) => !current);
          }}
        >
          {createAccount
            ? 'Already have an account? Sign in instead'
            : 'New server? Create an account instead'}
        </button>
      </p>
    </form>
  );
}
