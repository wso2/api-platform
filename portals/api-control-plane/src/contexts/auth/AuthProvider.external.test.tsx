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

import { useState, type ReactNode } from 'react';
import { http, HttpResponse, type JsonBodyType } from 'msw';
import { afterEach, beforeEach, describe, expect, it, onTestFinished, vi } from 'vitest';

import { act, fireEvent, render, renderWithProviders, screen, waitFor } from '../../test/utils';
import { server } from '../../test/server';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from './authConstants';

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

// Exact, so waiting for "authenticated" doesn't stop at "unauthenticated".
const waitForStatus = (status: string) =>
  waitFor(() => expect(screen.getByTestId('status').textContent).toBe(status));

afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
  vi.unstubAllGlobals();
  vi.useRealTimers();
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

    // A misrouted session URL serving the console's page: signed out would
    // send the browser round the sign-in loop.
    it('is unavailable, not signed out, on a 2xx that is not JSON', async () => {
      server.use(http.get(URL, () => HttpResponse.html('<!doctype html><title>Console</title>')));
      await renderProvider(EXTERNAL_CONFIG);

      await waitForStatus('unavailable');
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

    // Never `expired`: that page's button would end the identity provider session.
    it.each([
      ['isSignedIn: false', { isSignedIn: false }, 200],
      ['a missing isSignedIn', { user: { sub: '6c1f' } }, 200],
      ['a 4xx', { error: 'not_found' }, 404],
    ])('is signed out on %s', async (_case, body, status) => {
      answer(URL, body, status);
      await renderProvider(SDK_CONFIG);

      await waitForStatus('unauthenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('');
    });

    // The gateway keeps sessions through a Redis outage.
    it.each([
      ['a 503', () => HttpResponse.json({ error: 'service_unavailable' }, { status: 503 })],
      ['no answer', () => HttpResponse.error()],
    ])('is unavailable, not signed out, on %s', async (_case, reply) => {
      server.use(http.get(URL, reply));
      await renderProvider(SDK_CONFIG);

      await waitForStatus('unavailable');
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

    it('logout posts to the gateway and follows the redirectUrl it answers', async () => {
      answer(SDK_CONFIG.authSessionUrl, { isSignedIn: true, user: { name: 'Alice' } });
      let csrfHeader: string | null = null;
      server.use(
        http.post('/api-platform/auth/logout', ({ request }) => {
          csrfHeader = request.headers.get(CSRF_HEADER);
          return HttpResponse.json({ redirectUrl: 'https://idp.example.com/oauth2/logout?id_token_hint=x' });
        }),
      );
      const assign = vi.fn();
      vi.stubGlobal('location', { ...window.location, assign });
      await renderProvider({ ...SDK_CONFIG, authLogoutStyle: 'post-json' });
      await waitForStatus('authenticated');

      fireEvent.click(screen.getByText('logout'));

      await waitFor(() =>
        expect(assign).toHaveBeenCalledWith('https://idp.example.com/oauth2/logout?id_token_hint=x'),
      );
      expect(csrfHeader).toBe(CSRF_HEADER_VALUE);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
    });

    it('signs out once, however often logout is pressed', async () => {
      answer(SDK_CONFIG.authSessionUrl, { isSignedIn: true, user: { name: 'Alice' } });
      let posts = 0;
      server.use(
        http.post('/api-platform/auth/logout', () => {
          posts++;
          return HttpResponse.json({ redirectUrl: 'https://idp.example.com/oauth2/logout' });
        }),
      );
      const assign = vi.fn();
      vi.stubGlobal('location', { ...window.location, assign });
      await renderProvider({ ...SDK_CONFIG, authLogoutStyle: 'post-json' });
      await waitForStatus('authenticated');

      fireEvent.click(screen.getByText('logout'));
      fireEvent.click(screen.getByText('logout'));

      await waitFor(() => expect(assign).toHaveBeenCalledWith('https://idp.example.com/oauth2/logout'));
      expect(posts).toBe(1);
      expect(assign).toHaveBeenCalledTimes(1);
    });

    // A check answering mid-logout must not redirect to sign-in and cancel the logout.
    it('ignores session checks once logout has started', async () => {
      let signedIn = true;
      server.use(
        http.get(SDK_CONFIG.authSessionUrl, () =>
          HttpResponse.json(signedIn ? { isSignedIn: true, user: { name: 'Alice' } } : { isSignedIn: false }),
        ),
      );
      let finishLogout: () => void = () => undefined;
      server.use(
        http.post('/api-platform/auth/logout', async () => {
          signedIn = false;
          await new Promise<void>((resolve) => {
            finishLogout = resolve;
          });
          return HttpResponse.json({ redirectUrl: 'https://idp.example.com/oauth2/logout' });
        }),
      );
      const assign = vi.fn();
      vi.stubGlobal('location', { ...window.location, assign });
      await renderProvider({ ...SDK_CONFIG, authLogoutStyle: 'post-json' });
      await waitForStatus('authenticated');
      const { notifyUnauthorized } = await import('../../api/core/sessionEvents');

      fireEvent.click(screen.getByText('logout'));
      await vi.waitFor(() => expect(signedIn).toBe(false));
      const alive = await act(() => notifyUnauthorized());
      finishLogout();

      expect(alive).toBe(false);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
      await waitFor(() => expect(assign).toHaveBeenCalledWith('https://idp.example.com/oauth2/logout'));
    });
  });

  describe('after a 401', () => {
    async function renderAndCheck(sessionAnswers: Array<() => Response>) {
      let calls = 0;
      server.use(
        http.get(SDK_CONFIG.authSessionUrl, () => sessionAnswers[Math.min(calls++, sessionAnswers.length - 1)]()),
      );
      await renderProvider(SDK_CONFIG);
      await waitForStatus('authenticated');
      // The provider's fresh module graph; in act, so the state it sets is on screen.
      const { notifyUnauthorized } = await import('../../api/core/sessionEvents');
      return act(() => notifyUnauthorized());
    }
    const signedIn = () => HttpResponse.json({ isSignedIn: true, user: { name: 'Alice' } });

    it('answers that the session is alive when the session endpoint says so', async () => {
      await expect(renderAndCheck([signedIn])).resolves.toBe(true);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
    });

    it('moves to signed out, with no retry, when the session has ended', async () => {
      const alive = await renderAndCheck([signedIn, () => HttpResponse.json({ isSignedIn: false })]);

      expect(alive).toBe(false);
      await waitForStatus('unauthenticated');
    });

    it('stays signed in, with no retry, when the session endpoint is down', async () => {
      const alive = await renderAndCheck([
        signedIn,
        () => HttpResponse.json({ error: 'service_unavailable' }, { status: 503 }),
      ]);

      expect(alive).toBe(false);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('Alice');
    });

    it('stays signed in, with no retry, when the answer breaks off', async () => {
      const alive = await renderAndCheck([
        signedIn,
        () => new HttpResponse('{"isSignedIn": tr', { headers: { 'Content-Type': 'application/json' } }),
      ]);

      expect(alive).toBe(false);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
      expect(screen.getByTestId('user')).toHaveTextContent('Alice');
    });
  });

  // The first load's check answers after a 401's check has signed the browser
  // back in: the older answer must not sign it out again.
  it('lets only the newest session check set the state', async () => {
    let calls = 0;
    let answerFirst: () => void = () => undefined;
    server.use(
      http.get(SDK_CONFIG.authSessionUrl, async () => {
        if (++calls === 1) {
          await new Promise<void>((resolve) => {
            answerFirst = resolve;
          });
          return HttpResponse.json({ isSignedIn: false });
        }
        return HttpResponse.json({ isSignedIn: true, user: { name: 'Alice' } });
      }),
    );
    // Nothing on screen marks the older answer landing, so mark when its body
    // has been read. From there to the state update is promise steps only.
    let firstRead: Promise<unknown> | undefined;
    let fetches = 0;
    const realFetch = globalThis.fetch;
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (...args) => {
      const first = ++fetches === 1;
      const response = await realFetch(...args);
      if (first) {
        const json = response.json.bind(response);
        response.json = () => (firstRead = json());
      }
      return response;
    });
    onTestFinished(() => fetchSpy.mockRestore());
    await renderProvider(SDK_CONFIG);
    await vi.waitFor(() => expect(calls).toBe(1));
    const { notifyUnauthorized } = await import('../../api/core/sessionEvents');

    await expect(act(() => notifyUnauthorized())).resolves.toBe(true);
    await waitForStatus('authenticated');
    await act(async () => {
      answerFirst();
      await vi.waitFor(() => expect(firstRead).toBeDefined());
      await firstRead;
      // One timer tick runs every promise step queued behind the read.
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(screen.getByTestId('status').textContent).toBe('authenticated');
    expect(calls).toBe(2);
  });

  describe('when the session endpoint does not answer in time', () => {
    /** Holds back the session check's timeout, to fire it when the test says. */
    function holdCheckTimeout() {
      const held: Array<() => void> = [];
      const realSetTimeout = globalThis.setTimeout;
      vi.spyOn(globalThis, 'setTimeout').mockImplementation(((
        callback: () => void,
        ms?: number,
        ...args: unknown[]
      ) => (ms === 15_000 ? held.push(callback) : realSetTimeout(callback, ms, ...args))) as typeof setTimeout);
      return {
        held: () => held.length,
        fire: () => held.splice(0).forEach((callback) => callback()),
      };
    }
    afterEach(() => vi.restoreAllMocks());

    it('is unavailable, not stuck loading, on the first load', async () => {
      const timeout = holdCheckTimeout();
      server.use(http.get(SDK_CONFIG.authSessionUrl, () => new Promise<Response>(() => undefined)));
      await renderProvider(SDK_CONFIG);
      await vi.waitFor(() => expect(timeout.held()).toBe(1));

      act(() => timeout.fire());

      await waitForStatus('unavailable');
    });

    it('answers a 401 and stays signed in when the body stops arriving', async () => {
      const timeout = holdCheckTimeout();
      let calls = 0;
      server.use(
        http.get(SDK_CONFIG.authSessionUrl, () => {
          if (++calls === 1) return HttpResponse.json({ isSignedIn: true, user: { name: 'Alice' } });
          const stalled = new ReadableStream({
            start: (controller) => controller.enqueue(new TextEncoder().encode('{"isSignedIn":')),
          });
          return new HttpResponse(stalled, { headers: { 'Content-Type': 'application/json' } });
        }),
      );
      await renderProvider(SDK_CONFIG);
      await waitForStatus('authenticated');
      const { notifyUnauthorized } = await import('../../api/core/sessionEvents');

      const alive = act(() => notifyUnauthorized());
      await vi.waitFor(() => expect(timeout.held()).toBe(2));
      timeout.fire();

      await expect(alive).resolves.toBe(false);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
    });
  });

  describe('polling the check-session endpoint', () => {
    const CHECK_URL = '/api-platform/auth/checksession';
    const POLLING_CONFIG = { ...SDK_CONFIG, authCheckSessionUrl: CHECK_URL, authCheckSessionInterval: '30' };
    const alice = { isSignedIn: true, user: { sub: '6c1f', name: 'Alice' }, scope: 'openid' };
    // A poll is a fetch and a render after the timer: allow for a busy suite.
    const eventually = (assertion: () => void) => vi.waitFor(assertion, { timeout: 5000 });
    const eventuallyStatus = (status: string) =>
      eventually(() => expect(screen.getByTestId('status').textContent).toBe(status));

    let visibility: DocumentVisibilityState = 'visible';
    const setVisibility = (state: DocumentVisibilityState) => {
      visibility = state;
      document.dispatchEvent(new Event('visibilitychange'));
    };
    beforeEach(() => {
      visibility = 'visible';
      vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility);
      // Only the polling interval. Testing Library's waitFor runs on setInterval
      // too, so waits on anything but the DOM use vi.waitFor.
      vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    });
    afterEach(() => vi.restoreAllMocks());

    /** Signs in, then answers the check endpoint with each body in turn (the last repeats). */
    async function signInAndPoll(config: Record<string, string>, checkAnswers: Array<() => Response>) {
      let checks = 0;
      answer(SDK_CONFIG.authSessionUrl, alice);
      server.use(
        http.get(CHECK_URL, () => checkAnswers[Math.min(checks++, checkAnswers.length - 1)]()),
      );
      await renderProvider(config);
      await waitForStatus('authenticated');
      await act(async () => {}); // the polling effect is set up after that render
      return { checks: () => checks };
    }

    it('asks every interval while the tab is visible, and signs out when the session has ended', async () => {
      const poll = await signInAndPoll(POLLING_CONFIG, [
        () => HttpResponse.json(alice),
        () => HttpResponse.json({ isSignedIn: false }),
      ]);

      vi.advanceTimersByTime(30_000);
      await eventually(() => expect(poll.checks()).toBe(1));
      expect(screen.getByTestId('status').textContent).toBe('authenticated');

      // A tick while the first poll is still finishing is skipped, so keep ticking.
      await eventually(() => {
        vi.advanceTimersByTime(30_000);
        expect(screen.getByTestId('status').textContent).toBe('unauthenticated');
      });

      // Signed out: no more polling.
      const checksWhenSignedOut = poll.checks();
      vi.advanceTimersByTime(90_000);
      expect(poll.checks()).toBe(checksWhenSignedOut);
    });

    it('waits while the tab is hidden, and asks the moment it is shown again', async () => {
      const poll = await signInAndPoll(POLLING_CONFIG, [() => HttpResponse.json({ isSignedIn: false })]);

      setVisibility('hidden');
      vi.advanceTimersByTime(120_000);
      expect(poll.checks()).toBe(0);

      setVisibility('visible');
      await eventuallyStatus('unauthenticated');
      expect(poll.checks()).toBe(1);
    });

    it('keeps the same user object when nothing changed, so the app does not re-render', async () => {
      const seen = new Set<unknown>();
      const { AuthProvider, useAuth } = await loadAuth(POLLING_CONFIG);
      function Watcher() {
        const auth = useAuth();
        if (auth.user) seen.add(auth.user);
        return <span data-testid="status">{auth.status}</span>;
      }
      let checks = 0;
      answer(SDK_CONFIG.authSessionUrl, alice);
      server.use(http.get(CHECK_URL, () => (checks++, HttpResponse.json(alice))));
      render(
        <AuthProvider>
          <Watcher />
        </AuthProvider>,
      );
      await waitForStatus('authenticated');
      await act(async () => {});

      await eventually(() => {
        vi.advanceTimersByTime(30_000);
        expect(checks).toBeGreaterThanOrEqual(2);
      });

      expect(seen.size).toBe(1);
    });

    it('does not poll without a check-session URL', async () => {
      const poll = await signInAndPoll(SDK_CONFIG, [() => HttpResponse.json({ isSignedIn: false })]);

      vi.advanceTimersByTime(120_000);

      expect(poll.checks()).toBe(0);
      expect(screen.getByTestId('status').textContent).toBe('authenticated');
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

  describe('switching organizations', () => {
    const SWITCH_CONFIG = {
      ...SDK_CONFIG,
      authOrganizationsUrl: '/api-platform/idp/users/me/ous',
      authSwitchOrganizationUrls: '/auth/switch-organization, /api-platform/auth/switch-organization',
    };
    const ORG_A = { id: 'ou-a', handle: 'org-a', name: 'Org A' };
    const ORG_B = { id: 'ou-b', handle: 'org-b', name: 'Org B' };
    const sessionIn = (org: typeof ORG_A) => ({
      isSignedIn: true,
      user: { sub: 'u', ouId: org.id, ouName: org.name, ouHandle: org.handle },
      scope: 'openid',
    });

    /** Shows the session's organization and list, and switches on request. */
    async function renderSwitching(config: Record<string, string> = SWITCH_CONFIG) {
      const { AuthProvider, useAuth } = await loadAuth(config);
      function Probe() {
        const auth = useAuth();
        const [outcome, setOutcome] = useState('');
        const switchTo = (handle: string) =>
          auth.exchangeOrgToken(handle).then(
            () => setOutcome('switched'),
            (error: { reason?: string }) => setOutcome(`failed:${error.reason}`),
          );
        return (
          <div>
            <span data-testid="status">{auth.status}</span>
            <span data-testid="org">{auth.user?.org?.handle ?? ''}</span>
            <span data-testid="can-switch">{String(auth.canSwitchOrganization)}</span>
            <span data-testid="orgs">{auth.organizations?.map((org) => org.name).join(',') ?? 'none'}</span>
            <span data-testid="outcome">{outcome}</span>
            {['org-a', 'org-b', 'org-c'].map((handle) => (
              <button key={handle} onClick={() => void switchTo(handle)}>{`to ${handle}`}</button>
            ))}
          </div>
        );
      }
      render(
        <AuthProvider>
          <Probe />
        </AuthProvider>,
      );
      await waitForStatus('authenticated');
    }

    const PARENT_SWITCH = '/auth/switch-organization';
    const OWN_SWITCH = '/api-platform/auth/switch-organization';

    /**
     * Each switch is recorded as `<path> <organization id>`, and must name the
     * unit as the ThunderID SDK's `ouSwitch` does: in the query and the body.
     * The console's own session moves to org B on a switch that names ou-b.
     */
    function serveSwitchableSession(switchStatus = 200) {
      let current = ORG_A;
      const switches: string[] = [];
      const switchAt = (path: string) =>
        http.post(path, async ({ request }) => {
          const fromQuery = new URL(request.url).searchParams.get('ouId');
          const { ouId } = (await request.json()) as { ouId?: string };
          switches.push(fromQuery === ouId ? `${path} ${ouId}` : `${path} mismatch`);
          if (switchStatus === 200 && path === OWN_SWITCH && ouId === ORG_B.id) current = ORG_B;
          return switchStatus === 200
            ? new HttpResponse(null, { status: 200 })
            : HttpResponse.json({ error: 'access_denied' }, { status: switchStatus });
        });
      server.use(
        http.get(SWITCH_CONFIG.authSessionUrl, () => HttpResponse.json(sessionIn(current))),
        http.get(SWITCH_CONFIG.authOrganizationsUrl, () =>
          HttpResponse.json({ totalResults: 2, organizationUnits: [ORG_A, ORG_B] }),
        ),
        switchAt(PARENT_SWITCH),
        switchAt(OWN_SWITCH),
      );
      return switches;
    }

    it('lists the organizations the session can move into', async () => {
      serveSwitchableSession();
      await renderSwitching();

      expect(screen.getByTestId('can-switch')).toHaveTextContent('true');
      await waitFor(() => expect(screen.getByTestId('orgs')).toHaveTextContent('Org A,Org B'));
    });

    it("moves each session in order by the organization's id, then reads the session again", async () => {
      const switches = serveSwitchableSession();
      await renderSwitching();

      fireEvent.click(screen.getByRole('button', { name: 'to org-b' }));

      await waitFor(() => expect(screen.getByTestId('outcome')).toHaveTextContent('switched'));
      expect(switches).toEqual([`${PARENT_SWITCH} ou-b`, `${OWN_SWITCH} ou-b`]);
      expect(screen.getByTestId('org')).toHaveTextContent('org-b');
    });

    it('does not switch when the session is already in the organization', async () => {
      const switches = serveSwitchableSession();
      await renderSwitching();

      fireEvent.click(screen.getByRole('button', { name: 'to org-a' }));

      await waitFor(() => expect(screen.getByTestId('outcome')).toHaveTextContent('switched'));
      expect(switches).toEqual([]);
    });

    it('refuses an organization the user is not a member of, without asking the gateway', async () => {
      const switches = serveSwitchableSession();
      await renderSwitching();

      fireEvent.click(screen.getByRole('button', { name: 'to org-c' }));

      await waitFor(() => expect(screen.getByTestId('outcome')).toHaveTextContent('failed:forbidden'));
      expect(switches).toEqual([]);
    });

    it.each([
      [403, 'forbidden'],
      [503, 'unavailable'],
    ])('reports a %i from a switch as %s, stops there and keeps the session', async (status, reason) => {
      const switches = serveSwitchableSession(status);
      await renderSwitching();

      fireEvent.click(screen.getByRole('button', { name: 'to org-b' }));

      await waitFor(() => expect(screen.getByTestId('outcome')).toHaveTextContent(`failed:${reason}`));
      expect(switches).toEqual([`${PARENT_SWITCH} ou-b`]);
      expect(screen.getByTestId('status')).toHaveTextContent('authenticated');
      expect(screen.getByTestId('org')).toHaveTextContent('org-a');
    });

    it('stays off, and never calls the switch endpoints, unless both URLs are set', async () => {
      const switches = serveSwitchableSession();
      const lists = vi.fn();
      server.events.on('request:start', ({ request }) => {
        if (request.url.includes(SWITCH_CONFIG.authOrganizationsUrl)) lists();
      });
      onTestFinished(() => {
        server.events.removeAllListeners();
      });
      await renderSwitching({ ...SDK_CONFIG, authOrganizationsUrl: SWITCH_CONFIG.authOrganizationsUrl });

      fireEvent.click(screen.getByRole('button', { name: 'to org-b' }));

      await waitFor(() => expect(screen.getByTestId('outcome')).toHaveTextContent('switched'));
      expect(screen.getByTestId('can-switch')).toHaveTextContent('false');
      expect(screen.getByTestId('orgs')).toHaveTextContent('none');
      expect(switches).toEqual([]);
      expect(lists).not.toHaveBeenCalled();
    });
  });
});
