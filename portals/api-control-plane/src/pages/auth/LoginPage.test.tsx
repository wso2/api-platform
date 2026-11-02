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

import { afterEach, describe, expect, it, vi } from 'vitest';

// Loading the testing library fresh skips its automatic cleanup between
// tests, so each render is unmounted here instead.
let unmountPage: (() => void) | undefined;

// The page reads the sign-in style from runtimeConfig, which is fixed at module
// load: set the deployment's runtime config, then load the page and the test
// providers fresh so they share one module graph (and one auth context).
async function renderLoginPage(
  runtimeConfig: Record<string, string>,
  preset: 'unauthenticated' | 'unavailable' | 'loading' = 'unauthenticated',
) {
  window.__RUNTIME_CONFIG__ = runtimeConfig;
  vi.resetModules();
  const { renderWithProviders, screen } = await import('../../test/utils');
  const { authStatePresets } = await import('../../test/mockAuthState');
  const { LoginPage } = await import('./LoginPage');

  const authState = authStatePresets[preset]();
  unmountPage = renderWithProviders(<LoginPage />, { route: '/login', authState }).unmount;
  return { authState, screen };
}

afterEach(() => {
  unmountPage?.();
  unmountPage = undefined;
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
});

// Each test loads the page's whole module graph fresh, which can take more
// than the default 5s on a cold transform cache.
describe('LoginPage', { timeout: 30_000 }, () => {
  it('shows the credential form in basic mode and does not redirect', async () => {
    const { authState, screen } = await renderLoginPage({ authMode: 'basic' });

    expect(await screen.findByLabelText('Username')).toBeInTheDocument();
    expect(authState.login).not.toHaveBeenCalled();
  });

  it('hands over to the sign-in endpoint straight away in oidc mode', async () => {
    const { authState, screen } = await renderLoginPage({ authMode: 'oidc' });

    expect(await screen.findByText('Signing you in')).toBeInTheDocument();
    expect(authState.login).toHaveBeenCalledWith('/');
  });

  it('hands over to the gateway straight away in external mode', async () => {
    const { authState, screen } = await renderLoginPage({
      authMode: 'external',
      authLoginUrl: '/api-platform/auth/login',
    });

    expect(await screen.findByText('Signing you in')).toBeInTheDocument();
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
    expect(authState.login).toHaveBeenCalledWith('/');
  });

  it('does not hand over to the gateway while the session endpoint is unavailable', async () => {
    const { authState, screen } = await renderLoginPage(
      { authMode: 'external', authLoginUrl: '/api-platform/auth/login' },
      'unavailable',
    );

    expect(await screen.findByText('Server error')).toBeInTheDocument();
    expect(authState.login).not.toHaveBeenCalled();
  });

  // Opened directly (a bookmark, a refresh): the session may be alive, or its
  // endpoint down.
  it('waits for the session check before handing over to the gateway', async () => {
    const { authState, screen } = await renderLoginPage(
      { authMode: 'external', authLoginUrl: '/api-platform/auth/login' },
      'loading',
    );

    expect(await screen.findByText('Signing you in')).toBeInTheDocument();
    expect(authState.login).not.toHaveBeenCalled();
  });

  it('shows the credential form when a deployment forces form sign-in', async () => {
    const { authState, screen } = await renderLoginPage({
      authMode: 'external',
      authLoginStyle: 'form',
    });

    expect(await screen.findByLabelText('Username')).toBeInTheDocument();
    expect(authState.login).not.toHaveBeenCalled();
  });
});
