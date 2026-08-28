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
import { Link, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../test/utils';
import { ErrorBoundary } from './ErrorBoundary';
import { PageErrorFallback } from './ErrorFallback';

/**
 * React logs every caught error to `console.error` in addition to handing it to
 * the boundary, so a passing test would still print two stack traces per case.
 */
beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
});

function Boom({ message = 'render exploded' }: { message?: string }): never {
  throw new Error(message);
}

describe('ErrorBoundary', () => {
  it('renders children while nothing throws', () => {
    renderWithProviders(
      <ErrorBoundary>
        <p>page body</p>
      </ErrorBoundary>
    );

    expect(screen.getByText('page body')).toBeInTheDocument();
  });

  it('shows the app fallback instead of unmounting when a child throws', () => {
    renderWithProviders(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>
    );

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Go to console home' })
    ).toBeInTheDocument();
  });

  it('keeps the raw message off the screen in a production build', () => {
    vi.stubEnv('DEV', false);

    renderWithProviders(
      <ErrorBoundary>
        <Boom message="Cannot read properties of undefined" />
      </ErrorBoundary>
    );

    // Sterile copy only — the real message goes to the console, which the
    // `beforeEach` spy above is capturing.
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(
      screen.queryByText('Cannot read properties of undefined')
    ).not.toBeInTheDocument();
    expect(console.error).toHaveBeenCalled();
  });

  it('shows the raw message as a developer aid in a dev build', () => {
    vi.stubEnv('DEV', true);

    renderWithProviders(
      <ErrorBoundary>
        <Boom message="Cannot read properties of undefined" />
      </ErrorBoundary>
    );

    expect(
      screen.getByText('Cannot read properties of undefined')
    ).toBeInTheDocument();
  });

  it('recovers through the fallback reset without reloading', async () => {
    // Held outside the component: the boundary unmounts its children when it
    // catches, so component state would be back to "throwing" on reset.
    let throwing = true;

    function Flaky() {
      if (throwing) throw new Error('transient');
      return <p>recovered body</p>;
    }

    const { user } = renderWithProviders(
      <ErrorBoundary
        fallback={(error, reset) => (
          <PageErrorFallback error={error} reset={reset} />
        )}
      >
        <Flaky />
      </ErrorBoundary>
    );

    expect(
      screen.getByText('This page could not be displayed')
    ).toBeInTheDocument();

    throwing = false;
    await user.click(screen.getByRole('button', { name: 'Try again' }));

    expect(await screen.findByText('recovered body')).toBeInTheDocument();
  });

  it('clears a caught error when resetKeys change', () => {
    const { rerender } = renderWithProviders(
      <ErrorBoundary resetKeys={['/apis']}>
        <Boom />
      </ErrorBoundary>
    );

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();

    rerender(
      <ErrorBoundary resetKeys={['/gateways']}>
        <p>next page body</p>
      </ErrorBoundary>
    );

    expect(screen.getByText('next page body')).toBeInTheDocument();
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });

  it('offers a reload, not a retry, for a stale code-split chunk', () => {
    renderWithProviders(
      <ErrorBoundary
        fallback={(error, reset) => (
          <PageErrorFallback error={error} reset={reset} />
        )}
      >
        <Boom message="Failed to fetch dynamically imported module: /assets/ApiListPage-a1b2c3.js" />
      </ErrorBoundary>
    );

    expect(
      screen.getByText('A newer version of the console is available')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Try again' })
    ).not.toBeInTheDocument();
  });
});

describe('page-level containment', () => {
  /**
   * The behaviour the boundary exists for: a throw in the routed page must not
   * take the surrounding shell with it. Modelled on `AppLayout`'s structure
   * (persistent chrome as siblings of the guarded outlet) rather than mounting
   * `AppLayout` itself, which would drag in the whole scope/query stack and
   * test Oxygen's `AppShell` more than this boundary.
   */
  function Shell({ children }: { children: ReactNode }) {
    // `useLocation`, not `window.location` — the pathname the boundary resets
    // on has to be the router's, which is what `AppLayout` passes.
    const routerLocation = useLocation();

    return (
      <>
        <nav>
          <Link to="/gateways">Gateways</Link>
        </nav>
        <ErrorBoundary
          fallback={(error, reset) => (
            <PageErrorFallback error={error} reset={reset} />
          )}
          resetKeys={[routerLocation.pathname]}
        >
          {children}
        </ErrorBoundary>
        <footer>console footer</footer>
      </>
    );
  }

  it('leaves the surrounding chrome mounted and navigable', async () => {
    const { user } = renderWithProviders(
      <Shell>
        <Routes>
          <Route path="/apis" element={<Boom />} />
          <Route path="/gateways" element={<p>gateways page</p>} />
        </Routes>
      </Shell>,
      { route: '/apis' }
    );

    expect(
      screen.getByText('This page could not be displayed')
    ).toBeInTheDocument();
    // The whole point: chrome survives, so there is a way out of the failure.
    expect(screen.getByRole('link', { name: 'Gateways' })).toBeInTheDocument();
    expect(screen.getByText('console footer')).toBeInTheDocument();

    await user.click(screen.getByRole('link', { name: 'Gateways' }));

    expect(await screen.findByText('gateways page')).toBeInTheDocument();
    expect(
      screen.queryByText('This page could not be displayed')
    ).not.toBeInTheDocument();
  });
});
