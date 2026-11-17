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

import { act, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '../api/core/errors';
import { OrganizationBootstrap, RECHECK_INTERVAL_MS } from './OrganizationBootstrap';

const state = vi.hoisted(() => ({
  existing: {} as {
    isPending: boolean;
    data: unknown;
    error: unknown;
    refetch: () => Promise<unknown>;
  },
  bootstrap: {} as { mutate: (vars: unknown, opts: { onError: (e: unknown) => void }) => void },
}));

vi.mock('../contexts/auth/AuthProvider', () => ({
  useAuth: () => ({ isAuthenticated: true, user: { org: { handle: 'acme', name: 'Acme' } } }),
}));
vi.mock('../config/runtime', () => ({ runtimeConfig: { defaultOrgRegion: 'us' } }));
vi.mock('../api/resources/organizations', () => ({
  useOrganization: () => state.existing,
  useBootstrapOrganization: () => state.bootstrap,
}));

/** The platform's own answer to a lookup: the organization does not exist. */
const orgNotFound = () => new ApiError('not found', { kind: 'http', status: 404, code: 'NOT_FOUND' });

/** A registration route that is not served: the gateway's 404, with no platform body. */
const routeNotServed = () =>
  new ApiError('failed', { kind: 'http', status: 404, code: 'CLIENT_MALFORMED_ERROR' });

const unavailable = () => new ApiError('unavailable', { kind: 'http', status: 503, code: 'SERVICE_UNAVAILABLE' });
const conflict = () => new ApiError('exists', { kind: 'http', status: 409, code: 'CONFLICT' });

let mutate: ReturnType<typeof vi.fn>;
let refetch: ReturnType<typeof vi.fn>;

/** Makes every registration fail with `error()`. */
function registrationFailsWith(error: () => ApiError) {
  mutate.mockImplementation((_vars: unknown, opts: { onError: (e: unknown) => void }) => opts.onError(error()));
}

/** A new failed lookup, as a refetch that still finds nothing returns. */
function lookupStillMissing(rerender: (ui: React.ReactElement) => void) {
  state.existing = { ...state.existing, error: orgNotFound() };
  rerender(<OrganizationBootstrap />);
}

beforeEach(() => {
  vi.useFakeTimers();
  mutate = vi.fn();
  refetch = vi.fn().mockResolvedValue(undefined);
  state.existing = { isPending: false, data: undefined, error: orgNotFound(), refetch };
  state.bootstrap = { mutate };
});

afterEach(() => {
  vi.useRealTimers();
});

describe('OrganizationBootstrap', () => {
  it('registers once when the organization is missing', () => {
    render(<OrganizationBootstrap />);
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0][0]).toMatchObject({ id: 'acme', displayName: 'Acme', region: 'us' });
  });

  it('stops registering when the deployment does not serve the route, and keeps looking instead', async () => {
    registrationFailsWith(routeNotServed);
    const { rerender } = render(<OrganizationBootstrap />);
    expect(mutate).toHaveBeenCalledTimes(1);

    // Renders with the lookup still missing must not register again.
    lookupStillMissing(rerender);
    lookupStillMissing(rerender);
    expect(mutate).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(RECHECK_INTERVAL_MS * 2);
    });
    expect(refetch).toHaveBeenCalledTimes(2);
    lookupStillMissing(rerender);
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it('stops looking once provisioning has created the organization', async () => {
    registrationFailsWith(routeNotServed);
    const { rerender } = render(<OrganizationBootstrap />);

    state.existing = { ...state.existing, data: { id: 'acme' }, error: null };
    rerender(<OrganizationBootstrap />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(RECHECK_INTERVAL_MS * 3);
    });
    expect(refetch).not.toHaveBeenCalled();
  });

  it('retries a failed registration only after waiting', async () => {
    registrationFailsWith(unavailable);
    const { rerender } = render(<OrganizationBootstrap />);
    expect(mutate).toHaveBeenCalledTimes(1);

    lookupStillMissing(rerender);
    expect(mutate).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(RECHECK_INTERVAL_MS);
    });
    expect(refetch).toHaveBeenCalledTimes(1);
    lookupStillMissing(rerender);
    expect(mutate).toHaveBeenCalledTimes(2);
  });

  it('treats a conflict as the other route having won', () => {
    registrationFailsWith(conflict);
    const { rerender } = render(<OrganizationBootstrap />);
    expect(refetch).toHaveBeenCalledTimes(1);

    lookupStillMissing(rerender);
    expect(mutate).toHaveBeenCalledTimes(1);
  });
});
