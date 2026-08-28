import { Navigate, Outlet, useLocation } from 'react-router';
import { useMe } from '../lib/queries';

/**
 * Gate for everything behind login: probes GET /auth/me once; while loading
 * shows nothing (avoids a login flash), on 401 redirects to /login carrying
 * the intended destination.
 */
export function RequireAuth() {
  const me = useMe();
  const location = useLocation();

  if (me.isPending) return null;
  if (me.isError) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }
  return <Outlet />;
}
