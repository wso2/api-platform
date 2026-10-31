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

/**
 * The read-only banner, end to end through the real `AppLayout`.
 *
 * Every piece of this path is unit-tested on its own — `useResourceLimits`
 * holds the verdict, `ReadOnlyBanner` renders what it is handed, and the cloud
 * plugin decides. What none of those cover is the **join**: that a supplier
 * mounted on `app.gate` reaches the Port at all, and that what it supplies
 * comes back out as rendered markup. That seam is what breaks silently.
 *
 * The supplier here is a stub, deliberately. The point is the host's half of
 * the contract — a real plugin is a different repo's concern, and the host must
 * work for any supplier that honours the Port.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useEffect } from 'react';

import { makeConsoleScope } from '@/test/mockScope';
import { collection } from '@/test/msw';
import { server } from '@/test/server';

import {
  APP_GATE_SLOT,
  ExtensionsProvider,
  HEADER_ACTIONS_SLOT,
  type ApiControlPlaneAppGate,
  type ApiControlPlaneCloudEntry,
} from '../../extensions';
import { ResourceLimitsProvider } from '../../hooks/useResourceLimits';
import type { CloudHostPort } from '../../hostPort';
import { renderWithProviders, screen, waitFor } from '../../test/utils';

import AppLayout from './AppLayout';

/**
 * A stand-in for the cloud entitlements plugin: headless, and its only job is
 * to push a verdict through the Port exactly as the real gate does.
 */
function makeGate(
  supply: (port: CloudHostPort) => void,
  id = 'entitlements-gate'
): ApiControlPlaneAppGate {
  return {
    id,
    slot: APP_GATE_SLOT,
    order: 10,
    render: (port) => <GateProbe port={port as CloudHostPort} supply={supply} />,
  };
}

function GateProbe({
  port,
  supply,
}: {
  port: CloudHostPort;
  supply: (port: CloudHostPort) => void;
}) {
  // In an effect, like the real gate: setting state during render would warn.
  useEffect(() => {
    supply(port);
    // The real gate re-supplies when the organization or refresh signal
    // changes; here one push is enough to prove the wiring.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return null;
}

const renderLayout = (extensions: ApiControlPlaneCloudEntry[] = []) =>
  renderWithProviders(
    <ResourceLimitsProvider>
      <ExtensionsProvider extensions={extensions}>
        <AppLayout />
      </ExtensionsProvider>
    </ResourceLimitsProvider>,
    {
      route: '/organizations/acme',
      scope: makeConsoleScope(),
    }
  );

beforeEach(() => {
  // The shell mounts the sidebar and header, which are not what this asserts.
  vi.spyOn(console, 'error').mockImplementation(() => {});
  // The header's quick-selectors list these on mount. Declared so the suite
  // does not depend on an unhandled-request fallback, which the test server
  // treats as an error.
  server.use(collection('/rest-apis', []), collection('/graphql-apis', []));
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('read-only banner, host wiring', () => {
  it('renders the supplier’s own markup when it reports read-only', async () => {
    renderLayout([
      makeGate((port) =>
        port.resourceLimits.setReadOnly(
          true,
          'Your organization is read-only because it has no active plan.',
          <span>
            This organization is in <strong>read-only mode</strong>.
          </span>
        )
      ),
    ]);

    const banner = await screen.findByTestId('read-only-banner');
    expect(banner).toHaveTextContent('This organization is in read-only mode.');
  });

  // The banner falls back to the plain sentence, so a supplier that gives a
  // reason but no markup still explains itself rather than rendering empty.
  it('falls back to the plain reason when no banner markup is supplied', async () => {
    renderLayout([
      makeGate((port) => port.resourceLimits.setReadOnly(true, 'No active plan.')),
    ]);

    const banner = await screen.findByTestId('read-only-banner');
    expect(banner).toHaveTextContent('No active plan.');
  });

  it('shows a default explanation when the reason is blank', async () => {
    renderLayout([makeGate((port) => port.resourceLimits.setReadOnly(true, '   '))]);

    const banner = await screen.findByTestId('read-only-banner');
    expect(banner).toHaveTextContent(/read-only/i);
  });

  it('renders no banner when the supplier reports not read-only', async () => {
    renderLayout([makeGate((port) => port.resourceLimits.setReadOnly(false))]);

    // Wait for the gate to have run, so this is not just asserting on an
    // unmounted first frame.
    await waitFor(() => expect(screen.queryByTestId('read-only-banner')).toBeNull());
  });

  /**
   * The §1.1 regression check: a standalone, on-prem or OSS build registers no
   * cloud plugin at all. Nothing must render, and nothing must be fetched.
   */
  describe('with no supplier — the standalone path', () => {
    it('renders no banner', async () => {
      renderLayout([]);
      await waitFor(() => expect(screen.queryByTestId('read-only-banner')).toBeNull());
    });

    it('issues no entitlement request of its own', async () => {
      const fetchSpy = vi.spyOn(globalThis, 'fetch');
      renderLayout([]);

      await waitFor(() => expect(screen.queryByTestId('read-only-banner')).toBeNull());

      const billingCalls = fetchSpy.mock.calls.filter(([input]) =>
        String(input).includes('/proxy/billing')
      );
      expect(billingCalls).toEqual([]);
    });
  });

  /**
   * Worth stating explicitly, because it is easy to assume otherwise: the slot
   * decides only WHERE an entry renders, not what it may do. Every extension
   * receives the same Port, so an entry registered against the header slot can
   * supply a read-only verdict just as a gate can — and the banner appears.
   *
   * `app.gate` is therefore a placement convention for headless policy, not a
   * capability boundary. Anything that could abuse the Port could do so from
   * any slot.
   */
  it('lets any slot’s entry supply a verdict, because the Port is shared', async () => {
    renderLayout([
      {
        ...makeGate((port) =>
          port.resourceLimits.setReadOnly(true, 'Supplied from the header slot.')
        ),
        slot: HEADER_ACTIONS_SLOT,
      } as ApiControlPlaneCloudEntry,
    ]);

    const banner = await screen.findByTestId('read-only-banner');
    expect(banner).toHaveTextContent('Supplied from the header slot.');
  });
});
