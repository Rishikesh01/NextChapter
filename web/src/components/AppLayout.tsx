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
      <header className="nc-topnav">
        <div className="nc-topnav-inner">
          <NavLink to="/" className="nc-topnav-brand">
            NextChapter
          </NavLink>
          <nav className="nc-topnav-links" aria-label="Primary">
            <NavLink to="/" end className="nc-topnav-link">
              Library
            </NavLink>
            <NavLink to="/rules" className="nc-topnav-link">
              Site rules
            </NavLink>
            <NavLink to="/settings" className="nc-topnav-link">
              Settings
            </NavLink>
          </nav>
          <div className="nc-topnav-user">
            <span className="nc-small">{me.data?.username}</span>
            <button className="nc-btn-link" type="button" onClick={signOut}>
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main className="nc-page">
        <Outlet />
      </main>
    </>
  );
}
