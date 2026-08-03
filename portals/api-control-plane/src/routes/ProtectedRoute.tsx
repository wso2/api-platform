import { useEffect, useRef, useState } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { LoadingState } from '../components/StateViews';
import { useAuth } from '../features/auth/AuthProvider';
import { routes } from './paths';

type SilentState = 'idle' | 'running' | 'done';

export function ProtectedRoute() {
  const auth = useAuth();
  const location = useLocation();
  const [silentState, setSilentState] = useState<SilentState>('idle');
  // Guards the silent attempt to exactly once. A ref (not `silentState`) so the
  // effect isn't re-triggered by its own state change — `auth` from the SDK is
  // not referentially stable, so re-runs would otherwise cancel the in-flight
  // attempt before it settles and strand the user on the loader.
  const silentAttempted = useRef(false);

  useEffect(() => {
    if (auth.isLoading) return;
    // Already resolved one way or another — no silent attempt needed.
    if (
      auth.isAuthenticated ||
      auth.status === 'expired' ||
      auth.status === 'forbidden'
    ) {
      setSilentState('done');
      return;
    }
    if (silentAttempted.current) return;
    silentAttempted.current = true;

    // Unauthenticated with a (possibly) live IdP session: try to restore it
    // silently before falling back to the interactive login screen. Always
    // settle to 'done' so a failed restore proceeds to /login.
    setSilentState('running');
    void auth.signInSilently().finally(() => setSilentState('done'));
  }, [auth]);

  // Show the loader until the SDK has loaded AND any silent attempt settles, so
  // we never flash the login screen for a user whose session can be restored.
  if (
    auth.isLoading ||
    silentState === 'running' ||
    (silentState === 'idle' && !auth.isAuthenticated)
  ) {
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
