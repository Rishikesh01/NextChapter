import { useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { ApiError } from '@nextchapter/api-client';
import { useRegister } from '../lib/queries';
import { ErrorBanner } from '../components/ErrorBanner';

export function RegisterPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const register = useRegister();
  const navigate = useNavigate();

  const fields =
    register.error instanceof ApiError ? register.error.fields : undefined;
  const bannerMessage =
    register.isError && fields === undefined
      ? register.error.message
      : undefined;

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (username.trim() === '' || password === '') return;
    register.mutate(
      { username: username.trim(), password },
      {
        onSuccess: () => {
          void navigate('/', { replace: true });
        },
      },
    );
  };

  return (
    <main className="nc-auth-wrap">
      <h1 className="nc-auth-brand">NextChapter</h1>
      <p className="nc-auth-tagline nc-small">Your reading, on your server.</p>

      <section className="nc-section">
        <h2 className="nc-section-title">Create an account</h2>
        {bannerMessage !== undefined && (
          <ErrorBanner>{bannerMessage}</ErrorBanner>
        )}
        <form onSubmit={submit}>
          <div className="nc-field">
            <label htmlFor="reg-username">Username</label>
            <input
              className={`nc-input${fields?.username !== undefined ? ' is-invalid' : ''}`}
              id="reg-username"
              type="text"
              autoComplete="username"
              spellCheck={false}
              autoFocus
              value={username}
              aria-invalid={fields?.username !== undefined}
              onChange={(event) => {
                setUsername(event.target.value);
              }}
            />
            {fields?.username !== undefined && (
              <p className="nc-field-error">{fields.username}</p>
            )}
          </div>
          <div className="nc-field">
            <label htmlFor="reg-password">Password</label>
            <input
              className={`nc-input${fields?.password !== undefined ? ' is-invalid' : ''}`}
              id="reg-password"
              type="password"
              autoComplete="new-password"
              value={password}
              aria-invalid={fields?.password !== undefined}
              onChange={(event) => {
                setPassword(event.target.value);
              }}
            />
            {fields?.password !== undefined && (
              <p className="nc-field-error">{fields.password}</p>
            )}
          </div>
          <button
            className="nc-btn-primary nc-btn-primary-block"
            type="submit"
            disabled={register.isPending}
          >
            {register.isPending ? 'Creating…' : 'Create account'}
          </button>
        </form>
      </section>

      <p className="nc-auth-alt nc-small">
        Have an account? <Link to="/login">Sign in</Link>
      </p>
    </main>
  );
}
