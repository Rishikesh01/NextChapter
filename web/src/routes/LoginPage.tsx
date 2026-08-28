import { useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router';
import { ApiError } from '@nextchapter/api-client';
import { useLogin } from '../lib/queries';
import { ErrorBanner } from '../components/ErrorBanner';

export function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const login = useLogin();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const passwordRef = useRef<HTMLInputElement>(null);

  // Router state (the RequireAuth redirect) wins; ?next= covers the
  // full-reload path a mid-session 401 takes (ADR-0010 §2). Only
  // same-origin relative paths are honored.
  const nextParam = searchParams.get('next');
  const from =
    (location.state as { from?: string } | null)?.from ??
    (nextParam !== null &&
    nextParam.startsWith('/') &&
    !nextParam.startsWith('//')
      ? nextParam
      : '/');

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (username.trim() === '' || password === '') return;
    login.mutate(
      { username: username.trim(), password },
      {
        onSuccess: () => {
          void navigate(from, { replace: true });
        },
      },
    );
  };

  const rejected = login.error instanceof ApiError && login.error.unauthorized;

  // A rejection is usually a typo: put the caret back on the password with
  // its value selected (design/flows/web-library.md).
  useEffect(() => {
    if (rejected) passwordRef.current?.select();
  }, [rejected]);

  return (
    <main className="nc-auth-wrap">
      <h1 className="nc-auth-brand">NextChapter</h1>
      <p className="nc-auth-tagline nc-small">Your reading, on your server.</p>

      <section className="nc-section">
        <h2 className="nc-section-title">Sign in</h2>
        {rejected && <ErrorBanner>Wrong username or password.</ErrorBanner>}
        {login.isError && !rejected && (
          <ErrorBanner>{login.error.message}</ErrorBanner>
        )}
        <form onSubmit={submit}>
          <div className="nc-field">
            <label htmlFor="login-username">Username</label>
            <input
              className="nc-input"
              id="login-username"
              type="text"
              autoComplete="username"
              spellCheck={false}
              autoFocus
              value={username}
              onChange={(event) => {
                setUsername(event.target.value);
              }}
            />
          </div>
          <div className="nc-field">
            <label htmlFor="login-password">Password</label>
            <input
              className="nc-input"
              id="login-password"
              type="password"
              autoComplete="current-password"
              ref={passwordRef}
              value={password}
              onChange={(event) => {
                setPassword(event.target.value);
              }}
            />
          </div>
          <button
            className="nc-btn-primary nc-btn-primary-block"
            type="submit"
            disabled={login.isPending}
          >
            {login.isPending ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </section>

      <p className="nc-auth-alt nc-small">
        New here? <Link to="/register">Create an account</Link>
      </p>
    </main>
  );
}
