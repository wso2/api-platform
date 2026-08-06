import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { LoadingState } from '../components/StateViews';
import { useAuth } from '../features/auth/AuthProvider';
import { routes } from './paths';

export function ProtectedRoute() {
  const auth = useAuth();
  const location = useLocation();

  // The BFF's session cookie already tells us, synchronously from
  // GET /api/session on mount, whether there's a session to restore — there
  // is no silent (iframe/prompt=none) restoration step to attempt.
  if (auth.isLoading) {
    return <LoadingState fullScreen label="Checking session" />;
  }

  if (auth.status === 'expired') {
    return <Navigate to={routes.sessionExpired} replace />;
  }

  if (auth.status === 'forbidden') {
    return <Navigate to={routes.unauthorized} replace />;
  }

  if (!auth.isAuthenticated) {
    return <Navigate to={routes.login} replace state={{ from: location }} />;
  }

  return <Outlet />;
}
