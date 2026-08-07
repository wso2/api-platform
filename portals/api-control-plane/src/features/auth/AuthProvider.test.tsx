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

import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { fireEvent, render, screen, waitFor } from '../../test/utils';
import { server } from '../../test/server';
import { AuthProvider, useAuth } from './AuthProvider';
import { CSRF_HEADER } from './authConstants';

// A minimal consumer that renders the auth state as text, so tests can assert
// on it via screen queries without reaching into React internals.
function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="status">{auth.status}</span>
      <span data-testid="user">{auth.user?.name ?? ''}</span>
      <span data-testid="error">{auth.error ?? ''}</span>
      <button onClick={() => void auth.loginWithCredentials('alice', 'secret')}>
        login
      </button>
      <button onClick={auth.logout}>logout</button>
    </div>
  );
}

function renderProvider() {
  return render(
    <AuthProvider>
      <Probe />
    </AuthProvider>
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('AuthProvider', () => {
  it('hydrates as authenticated from an existing session cookie', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: true, user: { name: 'Alice', email: 'alice@example.com' } })
      )
    );
    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );
    expect(screen.getByTestId('user')).toHaveTextContent('Alice');
  });

  it('hydrates as unauthenticated when there is no session', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: false }, { status: 401 })
      )
    );
    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
  });

  it('hydrates as expired when the BFF reports a stale session, not a plain unauthenticated', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: false, reason: 'expired' }, { status: 401 })
      )
    );
    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('expired')
    );
  });

  it('loginWithCredentials sends the CSRF header and updates state on success', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: false }, { status: 401 })
      )
    );
    let gotCsrfHeader: string | null = 'unset';
    server.use(
      http.post('/api/login', ({ request }) => {
        gotCsrfHeader = request.headers.get(CSRF_HEADER);
        return HttpResponse.json({ user: { name: 'Bob', email: 'bob@example.com' } });
      })
    );
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );

    fireEvent.click(screen.getByText('login'));

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );
    expect(screen.getByTestId('user')).toHaveTextContent('Bob');
    expect(gotCsrfHeader).not.toBeNull();
  });

  it('loginWithCredentials surfaces the BFF error message on invalid credentials', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: false }, { status: 401 })
      ),
      http.post('/api/login', () =>
        HttpResponse.json(
          { status: 'error', code: 'INVALID_CREDENTIALS', message: 'invalid credentials' },
          { status: 401 }
        )
      )
    );
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );

    fireEvent.click(screen.getByText('login'));

    await waitFor(() =>
      expect(screen.getByTestId('error')).toHaveTextContent('invalid credentials')
    );
    expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated');
  });

  it('logout clears local state and navigates to the returned logoutUrl', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: true, user: { name: 'Alice', email: 'alice@example.com' } })
      ),
      http.post('/api/logout', () =>
        HttpResponse.json({ logoutUrl: 'https://idp.example.com/logout' })
      )
    );
    const assign = vi.fn();
    vi.stubGlobal('location', { ...window.location, assign });
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );

    fireEvent.click(screen.getByText('logout'));

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
    expect(assign).toHaveBeenCalledWith('https://idp.example.com/logout');
  });

  it('logout falls back to a login path when the BFF returns no logoutUrl', async () => {
    server.use(
      http.get('/api/session', () =>
        HttpResponse.json({ authenticated: true, user: { name: 'Alice', email: 'alice@example.com' } })
      ),
      // file-based mode: 204 No Content, no logoutUrl body at all.
      http.post('/api/logout', () => new HttpResponse(null, { status: 204 }))
    );
    const assign = vi.fn();
    vi.stubGlobal('location', { ...window.location, assign });
    renderProvider();
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );

    fireEvent.click(screen.getByText('logout'));

    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
    // appBasePath is '' in this test environment (see config/runtime.test.ts),
    // so the base-path-aware fallback resolves to a bare "/login" here — the
    // non-empty-base-path case is covered at the unit level by
    // normalizeBasePath's own tests in config/runtime.test.ts.
    expect(assign).toHaveBeenCalledWith('/login');
  });
});
