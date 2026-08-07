/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

  it('redirects to login when unauthenticated', () => {
    renderGuard(authStatePresets.unauthenticated());
    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });
});
