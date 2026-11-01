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

import type { ReactNode } from 'react';
import { http, HttpResponse, type JsonBodyType } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { fireEvent, render, renderWithProviders, screen, waitFor } from '../../test/utils';
import { server } from '../../test/server';

// A console under /api-platform behind a gateway or auth proxy that acts as its
// BFF: no /api/* routes, and the gateway's own sign-in, session and sign-out URLs.
const EXTERNAL_CONFIG = {
  authMode: 'external',
  appBasePath: '/api-platform',
  authLoginUrl: '/api-platform/auth/login',
  authReturnParam: 'returnTo',
  authSessionUrl: '/api-platform/auth/session',
  authLogoutUrl: '/api-platform/auth/logout',
  authClaimMappings:
    '{"username":"name","organization":"ouId","org_name":"ouName","org_handle":"ouHandle"}',
};

// The gateway's oauth2-session policy: the ThunderID SDK's backend-mode session.
const SDK_CONFIG = {
  ...EXTERNAL_CONFIG,
  authSessionUrl: '/api-platform/auth/userinfo',
  authSessionFormat: 'sdk',
};

// runtimeConfig is read once, when the module loads, so every test loads the
// provider fresh after setting the deployment's config.
async function loadAuth(config: Record<string, string>) {
  window.__RUNTIME_CONFIG__ = config;
  vi.resetModules();
  return import('./AuthProvider');
}

type AuthModule = Awaited<ReturnType<typeof loadAuth>>;

/** Shows what the provider resolved, and offers its login and logout. */
function makeProbe(useAuth: AuthModule['useAuth']) {
  return function Probe() {
    const auth = useAuth();
    return (
      <div>
        <span data-testid="status">{auth.status}</span>
        <span data-testid="user">{auth.user?.name ?? ''}</span>
        <span data-testid="org">{auth.user?.org?.handle ?? ''}</span>
        <span data-testid="scopes">
          {auth.user?.scopes === undefined ? 'unknown' : `[${auth.user.scopes.join(' ')}]`}
        </span>
        <button onClick={() => auth.login('/orgs/acme')}>login to acme</button>
        <button onClick={() => auth.login()}>login here</button>
        <button onClick={auth.logout}>logout</button>
      </div>
    );
  };
}

async function renderProvider(config: Record<string, string>) {
  const { AuthProvider, useAuth } = await loadAuth(config);
  const Probe = makeProbe(useAuth);
  render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  );
}

const answer = (url: string, body: JsonBodyType | null, status = 200) =>
  server.use(
    http.get(url, () =>
      body === null ? new HttpResponse(null, { status }) : HttpResponse.json(body, { status }),
    ),
  );

const waitForStatus = (status: string) =>
  waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent(status));

afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
  vi.unstubAllGlobals();
});

// Each test loads the provider's module graph fresh, which can take more than
// the default 5s when the whole suite runs at once.
describe('AuthProvider in external mode', { timeout: 30_000 }, () => {
  describe('with the claims session format', () => {
    const URL = EXTERNAL_CONFIG.authSessionUrl;

    it('resolves the user, organization and scopes from the claims', async () => {
      answer(URL, {
        sub: '6c1f',
        name: 'Bob Builder',
        scope: 'openid ap:rest_api:manage',
        ouId: '436e4321',
        ouName: "Bob's Org",
        ouHandle: 'bobs-org',
      });
      await renderProvider(EXTERNAL_CONFIG);

      await waitForStatus('authenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('Bob Builder');
      expect(screen.getByTestId('org')).toHaveTextContent('bobs-org');
      expect(screen.getByTestId('scopes')).toHaveTextContent('[openid ap:rest_api:manage]');
    });

    // "unknown" lets PermissionProvider offer every action; "[]" disables them all.
    it.each([
      ['no scope claim', 'unknown', { sub: '6c1f' }],
      ['an empty scope claim', '[]', { sub: '6c1f', scope: '' }],
      ['an `scp` array', '[ap:rest_api:read]', { sub: '6c1f', scp: ['ap:rest_api:read'] }],
    ])('reads %s as scopes %s', async (_case, expected, claims) => {
      answer(URL, claims);
      await renderProvider(EXTERNAL_CONFIG);

      await waitForStatus('authenticated');
      expect(screen.getByTestId('scopes')).toHaveTextContent(expected);
    });

    it.each([
      ['a 401', { error: 'unauthorized' }, 401],
      ['a 2xx without a claims object', ['not', 'claims'], 200],
    ])('is signed out on %s', async (_case, body, status) => {
      answer(URL, body, status);
      await renderProvider(EXTERNAL_CONFIG);

      await waitForStatus('unauthenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('');
    });
  });

  describe('with the SDK session format', () => {
    const URL = SDK_CONFIG.authSessionUrl;

    it('resolves the user from `user` and the scopes from `scope`', async () => {
      answer(URL, {
        isSignedIn: true,
        expiresAt: 1800003600,
        user: {
          sub: '6c1f',
          name: 'Bob Builder',
          ouId: '436e',
          ouName: "Bob's Org",
          ouHandle: 'bobs-org',
        },
        scope: 'openid  ap:rest_api:manage',
      });
      await renderProvider(SDK_CONFIG);

      await waitForStatus('authenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('Bob Builder');
      expect(screen.getByTestId('org')).toHaveTextContent('bobs-org');
      expect(screen.getByTestId('scopes')).toHaveTextContent('[openid ap:rest_api:manage]');
    });

    it.each([
      ['no `scope`', 'unknown', { isSignedIn: true, user: { sub: '6c1f' } }],
      ['an empty `scope`', '[]', { isSignedIn: true, user: { sub: '6c1f' }, scope: '' }],
      // `user` holds ID token claims; a scope among them is not a grant.
      [
        'a scope only inside `user`',
        'unknown',
        { isSignedIn: true, user: { scope: 'ap:gateway:manage' } },
      ],
      ['no `user`', '[openid]', { isSignedIn: true, scope: 'openid' }],
    ])('reads a session with %s as scopes %s', async (_case, expected, body) => {
      answer(URL, body);
      await renderProvider(SDK_CONFIG);

      await waitForStatus('authenticated');
      expect(screen.getByTestId('scopes')).toHaveTextContent(expected);
    });

    it.each([
      ['isSignedIn: false', { isSignedIn: false }, 200],
      ['a missing isSignedIn', { user: { sub: '6c1f' } }, 200],
      ['a non-2xx', { error: 'service_unavailable' }, 503],
    ])('is signed out on %s', async (_case, body, status) => {
      answer(URL, body, status);
      await renderProvider(SDK_CONFIG);

      await waitForStatus('unauthenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('');
    });

    it('is signed out when the session request fails', async () => {
      server.use(http.get(URL, () => HttpResponse.error()));
      await renderProvider(SDK_CONFIG);

      await waitForStatus('unauthenticated');
    });
  });

  describe('login and logout', () => {
    it.each([
      [
        'a path inside the app, under the base path',
        'login to acme',
        '%2Fapi-platform%2Forgs%2Facme',
      ],
      ['the current page as it is', 'login here', '%2Fapi-platform%2Fapis'],
    ])('login comes back to %s', async (_case, button, returnTo) => {
      answer(SDK_CONFIG.authSessionUrl, { isSignedIn: false });
      const assign = vi.fn();
      vi.stubGlobal('location', { ...window.location, pathname: '/api-platform/apis', assign });
      await renderProvider(SDK_CONFIG);
      await waitForStatus('unauthenticated');

      fireEvent.click(screen.getByText(button));

      expect(assign).toHaveBeenCalledWith(`/api-platform/auth/login?returnTo=${returnTo}`);
    });

    // No request to the logout URL: MSW fails any request without a handler.
    // Local state stays as it is: the browser is leaving, and flipping to signed
    // out first would let a route guard redirect and cancel the navigation.
    it('logout navigates to the gateway without a request and keeps local state', async () => {
      answer(SDK_CONFIG.authSessionUrl, { isSignedIn: true, user: { name: 'Alice' } });
      const assign = vi.fn();
      vi.stubGlobal('location', { ...window.location, assign });
      await renderProvider(SDK_CONFIG);
      await waitForStatus('authenticated');

      fireEvent.click(screen.getByText('logout'));

      expect(assign).toHaveBeenCalledWith('/api-platform/auth/logout');
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated');
    });
  });

  // The session's scopes decide which actions the console offers. Creating a
  // gateway needs ap:gateway:create or ap:gateway:manage. While the session
  // loads every action is offered, so each case waits for the signed-in state.
  describe('with the permission layer', () => {
    async function renderGatedButton(body: JsonBodyType) {
      answer(SDK_CONFIG.authSessionUrl, body);
      const { AuthProvider, useAuth } = await loadAuth(SDK_CONFIG);
      // Loaded after the provider, into the same fresh module graph, so they
      // read the auth context it provides.
      const [{ PermissionProvider }, { Can }] = await Promise.all([
        import('../../permissions/PermissionProvider'),
        import('../../permissions/Can'),
      ]);
      const Probe = makeProbe(useAuth);
      const Gated = ({ children }: { children: ReactNode }) => (
        <AuthProvider>
          <Probe />
          <PermissionProvider>
            <Can do="CreateGateway" denied="disable">
              {children}
            </Can>
          </PermissionProvider>
        </AuthProvider>
      );
      renderWithProviders(
        <Gated>
          <button>Provision gateway</button>
        </Gated>,
      );
      await waitForStatus('authenticated');
      return screen.getByRole('button', { name: 'Provision gateway' });
    }

    it.each([
      ['enabled', 'with', 'openid ap:gateway:manage', true],
      ['disabled', 'without', 'openid ap:gateway:read ap:rest_api:manage', false],
    ])(
      'the action is %s %s ap:gateway:manage in the session scope',
      async (_state, _with, scope, enabled) => {
        const button = await renderGatedButton({ isSignedIn: true, user: { sub: 'u' }, scope });

        if (enabled) expect(button).toBeEnabled();
        else expect(button).toBeDisabled();
      },
    );

    it('offers every action when the session has no scope, leaving the decision to the server', async () => {
      const button = await renderGatedButton({ isSignedIn: true, user: { sub: 'u' } });

      expect(button).toBeEnabled();
    });
  });
});
