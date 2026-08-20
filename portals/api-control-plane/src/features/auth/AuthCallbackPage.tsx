import { useEffect, useRef, useState } from 'react';
import { Navigate } from 'react-router-dom';

import { LoadingState } from '../../components/StateViews';
import { useAuth } from './AuthProvider';

export function AuthCallbackPage() {
  const auth = useAuth();
  const [redirectTo, setRedirectTo] = useState<string>();
  // completeLoginFromRedirect reads-and-clears the stored return URL (and, in
  // local-file mode, activates the session), so it must run exactly once even
  // though the auth context object changes as the session settles.
  const completedRef = useRef(false);

  useEffect(() => {
    if (completedRef.current) return;
    completedRef.current = true;
    setRedirectTo(auth.completeLoginFromRedirect());
  }, [auth]);

  if (!redirectTo) return <LoadingState fullScreen label="Completing sign in" />;

  return <Navigate to={redirectTo} replace />;
}
