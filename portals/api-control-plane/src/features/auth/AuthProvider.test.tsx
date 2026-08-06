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
});
