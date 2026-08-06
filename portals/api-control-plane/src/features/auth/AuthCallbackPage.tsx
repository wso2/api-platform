import { Navigate } from 'react-router-dom';

// The BFF's own /api/auth/callback performs the whole OIDC code exchange
// server-side and redirects straight to the sanitized return URL with the
// session cookie already set — the SPA is never actually navigated here in
// the normal flow. This route exists only as a safe fallback for a stray
// deep link to /signin.
export function AuthCallbackPage() {
  return <Navigate to="/" replace />;
}
