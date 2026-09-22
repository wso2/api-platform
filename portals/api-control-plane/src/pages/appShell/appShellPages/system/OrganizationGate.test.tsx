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

import { HttpResponse, http as mswHttp } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetHttpClient } from '@/api/core/http';
import {
  anOrganization,
  apiUrl,
  collection,
  neverResponds,
  recorder,
  type Recorder,
} from '@/test/msw';
import { makeAuthState } from '@/test/mockAuthState';
import { server } from '@/test/server';
import { act, renderWithProviders, screen, waitFor } from '@/test/utils';

import { OrganizationGate } from './OrganizationGate';

const org = anOrganization({ id: 'acme', displayName: 'Acme' });

const authWithOrgClaim = () =>
  makeAuthState({
    user: {
      name: 'Test User',
      email: 'test.user@example.com',
      org: { id: 'org-uuid', name: 'Acme', handle: 'acme' },
    },
  });

const listBody = (list: (typeof org)[]) => ({
  count: list.length,
  list,
  pagination: { total: list.length, offset: 0, limit: 20 },
});

// `/organizations` driven by a per-call script, so a test can say what the Nth
// poll answers. `record` counts every call — that is what tells "stopped
// polling" apart from "still polling".
const scriptedOrganizations = (
  answer: (call: number) => 'empty' | 'found' | 'error',
  record: Recorder
) => {
  let call = 0;
  return mswHttp.get(apiUrl('/organizations'), async ({ request }) => {
    await record.capture(request);
    const outcome = answer(call++);
    if (outcome === 'error') return HttpResponse.json({}, { status: 500 });
    return HttpResponse.json(listBody(outcome === 'found' ? [org] : []));
  });
};

const renderGate = (authState = authWithOrgClaim()) =>
  renderWithProviders(
    <OrganizationGate>
      <div>console body</div>
    </OrganizationGate>,
    { authState }
  );

describe('OrganizationGate', () => {
  beforeEach(() => {
    resetHttpClient();
  });

  it('renders the console once an organization exists', async () => {
    server.use(collection('/organizations', [org]));

    renderGate();

    expect(await screen.findByText('console body')).toBeInTheDocument();
  });

  it('waits behind a loading screen while the organization is still being provisioned', async () => {
    server.use(collection('/organizations', []));

    renderGate();

    expect(await screen.findByText('Setting up your organization')).toBeInTheDocument();
    expect(screen.queryByText('console body')).not.toBeInTheDocument();
  });

  it('lets the console through when the session carries no organization claim', async () => {
    server.use(collection('/organizations', []));

    renderGate(makeAuthState());

    expect(await screen.findByText('console body')).toBeInTheDocument();
  });

  it('does not hold back a session with no organization claim while the list is in flight', async () => {
    server.use(neverResponds('get', '/organizations'));

    renderGate(makeAuthState());

    expect(await screen.findByText('console body')).toBeInTheDocument();
  });

  describe('with fake timers', () => {
    let requests: Recorder;

    beforeEach(() => {
      requests = recorder();
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });
    afterEach(() => {
      vi.useRealTimers();
    });

    it('polls until the organization appears, then stops polling', async () => {
      server.use(scriptedOrganizations((call) => (call < 2 ? 'empty' : 'found'), requests));

      renderGate();

      await waitFor(() =>
        expect(screen.getByText('Setting up your organization')).toBeInTheDocument()
      );

      await act(() => vi.advanceTimersByTimeAsync(2_000));
      await act(() => vi.advanceTimersByTimeAsync(2_000));

      await waitFor(() => expect(screen.getByText('console body')).toBeInTheDocument());

      const settled = requests.count();
      await act(() => vi.advanceTimersByTimeAsync(60_000));
      expect(requests.count()).toBe(settled);
    });

    it('keeps waiting through a failed poll instead of dropping into an error page', async () => {
      server.use(
        scriptedOrganizations(
          (call) => (call === 1 ? 'error' : call < 4 ? 'empty' : 'found'),
          requests
        )
      );

      renderGate();

      await waitFor(() =>
        expect(screen.getByText('Setting up your organization')).toBeInTheDocument()
      );

      await act(() => vi.advanceTimersByTimeAsync(2_000));
      expect(screen.getByText('Setting up your organization')).toBeInTheDocument();
      expect(screen.queryByText('console body')).not.toBeInTheDocument();

      await act(() => vi.advanceTimersByTimeAsync(30_000));
      await waitFor(() => expect(screen.getByText('console body')).toBeInTheDocument());
    });

    it('gives up on an absolute deadline, even when polls keep failing intermittently', async () => {
      server.use(scriptedOrganizations((call) => (call % 5 === 0 ? 'error' : 'empty'), requests));

      renderGate();

      await waitFor(() =>
        expect(screen.getByText('Setting up your organization')).toBeInTheDocument()
      );

      await act(() => vi.advanceTimersByTimeAsync(180_000));

      await waitFor(() =>
        expect(screen.getByText('This is taking longer than expected')).toBeInTheDocument()
      );
      expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Log out' })).toBeInTheDocument();

      const settled = requests.count();
      await act(() => vi.advanceTimersByTimeAsync(60_000));
      expect(requests.count()).toBe(settled);
    });

    it('re-arms the wait when the user retries after the deadline', async () => {
      let ready = false;
      server.use(scriptedOrganizations(() => (ready ? 'found' : 'empty'), requests));

      const { user } = renderGate();

      await waitFor(() =>
        expect(screen.getByText('Setting up your organization')).toBeInTheDocument()
      );
      await act(() => vi.advanceTimersByTimeAsync(180_000));
      await waitFor(() =>
        expect(screen.getByText('This is taking longer than expected')).toBeInTheDocument()
      );

      await user.click(screen.getByRole('button', { name: 'Try again' }));

      await waitFor(() =>
        expect(screen.getByText('Setting up your organization')).toBeInTheDocument()
      );

      ready = true;
      await act(() => vi.advanceTimersByTimeAsync(2_000));
      await waitFor(() => expect(screen.getByText('console body')).toBeInTheDocument());
    });
  });
});
