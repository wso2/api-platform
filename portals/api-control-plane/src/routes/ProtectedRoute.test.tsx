import { describe, expect, it } from 'vitest';
import { Route, Routes } from 'react-router-dom';

import { renderWithProviders, screen } from '../test/utils';
import { authStatePresets, makeAuthState } from '../test/mockAuthState';
import { ProtectedRoute } from './ProtectedRoute';

// Renders ProtectedRoute as a layout route with a sentinel child plus marker
// routes for each redirect target, so we can assert what the guard renders.
function renderGuard(authState = makeAuthState()) {
  return renderWithProviders(
    <Routes>
      <Route element={<ProtectedRoute />}>
        <Route index element={<div>Protected content</div>} />
      </Route>
      <Route path="/login" element={<div>Login Page</div>} />
      <Route path="/session-expired" element={<div>Session Expired</div>} />
      <Route path="/unauthorized" element={<div>Unauthorized Page</div>} />
    </Routes>,
    { route: '/', authState }
  );
}

describe('ProtectedRoute', () => {
  it('renders the protected outlet when authenticated', () => {
    renderGuard(authStatePresets.authenticated());
    expect(screen.getByText('Protected content')).toBeInTheDocument();
  });

  it('shows the session loader while auth is loading', () => {
    renderGuard(authStatePresets.loading());
    expect(screen.getByText('Checking session')).toBeInTheDocument();
    expect(screen.queryByText('Protected content')).not.toBeInTheDocument();
  });

  it('redirects to session-expired when the session expired', () => {
    renderGuard(authStatePresets.expired());
    expect(screen.getByText('Session Expired')).toBeInTheDocument();
  });

  it('redirects to unauthorized when forbidden', () => {
    renderGuard(authStatePresets.forbidden());
    expect(screen.getByText('Unauthorized Page')).toBeInTheDocument();
  });

  it('redirects to login when unauthenticated', () => {
    renderGuard(authStatePresets.unauthenticated());
    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });
});
