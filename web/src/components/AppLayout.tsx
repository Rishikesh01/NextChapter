import { NavLink, Outlet, useNavigate } from 'react-router';
import { useLogout, useMe } from '../lib/queries';

export function AppLayout() {
  const me = useMe();
  const logout = useLogout();
  const navigate = useNavigate();

  const signOut = () => {
    logout.mutate(undefined, {
      onSettled: () => {
        void navigate('/login');
      },
    });
  };

  return (
    <>
      <header className="nc-nav">
        <div className="nc-nav-inner">
          <NavLink to="/" className="nc-nav-brand">
            NextChapter
          </NavLink>
          <nav className="nc-nav-links" aria-label="Primary">
            <NavLink to="/" end className="nc-nav-link">
              Library
            </NavLink>
            <NavLink to="/rules" className="nc-nav-link">
              Site rules
            </NavLink>
            <NavLink to="/settings" className="nc-nav-link">
              Settings
            </NavLink>
          </nav>
          <div className="nc-nav-user">
            <span className="nc-nav-username">{me.data?.username}</span>
            <button className="nc-btn-link" type="button" onClick={signOut}>
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main className="nc-content">
        <Outlet />
      </main>
    </>
  );
}
