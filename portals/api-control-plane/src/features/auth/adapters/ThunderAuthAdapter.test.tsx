import { type Context, useContext } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { render, screen, userEvent, waitFor } from '../../../test/utils';
import type { AuthState } from '../authTypes';

// Controllable UserManager mock shared with the hoisted vi.mock factory.
const mgr = vi.hoisted(() => ({
  getUser: vi.fn(),
  signinRedirect: vi.fn().mockResolvedValue(undefined),
  signinRedirectCallback: vi.fn(),
  signinSilent: vi.fn(),
  removeUser: vi.fn().mockResolvedValue(undefined),
  signoutRedirect: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('oidc-client-ts', () => ({
  UserManager: vi.fn(() => mgr),
  WebStorageStateStore: vi.fn(),
  User: class {},
}));

const oidcUser = (overrides: Record<string, unknown> = {}) => ({
  access_token: 'thunder-access-token',
  expired: false,
  profile: { name: 'Ada Lovelace', email: 'ada@example.com' },
  ...overrides,
});

// The adapter and AuthStateContext must come from the SAME (freshly reset)
// module graph, so the probe reads the adapter's provider value rather than a
// different context instance.
const makeProbe = (ctx: Context<AuthState | null>) =>
  function Probe() {
    const s = useContext(ctx);
    return (
      <div>
        <span data-testid="status">{s?.status}</span>
        <span data-testid="token">{s?.token ?? ''}</span>
        <span data-testid="email">{s?.user?.email ?? ''}</span>
        <button type="button" onClick={() => s?.login('/back')}>
          login
        </button>
        <button type="button" onClick={() => s?.logout()}>
          logout
        </button>
      </div>
    );
  };

async function loadAdapter() {
  vi.stubEnv('VITE_AUTH_MODE', 'thunder');
  vi.stubEnv('VITE_AUTH_BASE_URL', 'http://idp.test/oauth2');
  vi.stubEnv('VITE_AUTH_CLIENT_ID', 'APIP_CONSOLE');
  vi.resetModules();
  const { ThunderAuthAdapter } = await import('./ThunderAuthAdapter');
  const { AuthStateContext } = await import('../AuthStateContext');
  return { ThunderAuthAdapter, Probe: makeProbe(AuthStateContext) };
}

describe('ThunderAuthAdapter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
  });
  afterEach(() => vi.unstubAllEnvs());

  it('settles to unauthenticated when there is no stored session', async () => {
    mgr.getUser.mockResolvedValue(null);
    const { ThunderAuthAdapter, Probe } = await loadAdapter();
    render(
      <ThunderAuthAdapter>
        <Probe />
      </ThunderAuthAdapter>
    );
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
  });

  it('activates an existing, unexpired session', async () => {
    mgr.getUser.mockResolvedValue(oidcUser());
    const { ThunderAuthAdapter, Probe } = await loadAdapter();
    render(
      <ThunderAuthAdapter>
        <Probe />
      </ThunderAuthAdapter>
    );
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );
    expect(screen.getByTestId('token')).toHaveTextContent('thunder-access-token');
    expect(screen.getByTestId('email')).toHaveTextContent('ada@example.com');
  });

  it('starts a redirect sign-in with the return path as state', async () => {
    mgr.getUser.mockResolvedValue(null);
    const user = userEvent.setup();
    const { ThunderAuthAdapter, Probe } = await loadAdapter();
    render(
      <ThunderAuthAdapter>
        <Probe />
      </ThunderAuthAdapter>
    );
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
    await user.click(screen.getByText('login'));
    expect(mgr.signinRedirect).toHaveBeenCalledWith({ state: '/back' });
  });

  it('clears the session on logout', async () => {
    mgr.getUser.mockResolvedValue(oidcUser());
    const user = userEvent.setup();
    const { ThunderAuthAdapter, Probe } = await loadAdapter();
    render(
      <ThunderAuthAdapter>
        <Probe />
      </ThunderAuthAdapter>
    );
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated')
    );
    await user.click(screen.getByText('logout'));
    expect(mgr.removeUser).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(screen.getByTestId('status')).toHaveTextContent('unauthenticated')
    );
  });
});
