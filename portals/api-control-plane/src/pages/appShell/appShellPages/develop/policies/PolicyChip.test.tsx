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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetHttpClient } from '@/api/core/http';
import { runtimeConfig } from '@/config/runtime';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { PolicyChip } from './PolicyChip';

// The hub is a different origin from the page, so jsdom enforces CORS on the reply.
const cors = { 'Access-Control-Allow-Origin': '*' };

const serveVersions = () =>
  server.use(
    http.get(`${runtimeConfig.policyHubBaseUrl}/policies/api-key-auth/versions`, () =>
      HttpResponse.json(
        [{ displayName: 'API Key Auth', isLatest: true, name: 'api-key-auth', version: '1.2' }],
        { headers: cors },
      ),
    ),
  );

const renderChip = (props: Partial<React.ComponentProps<typeof PolicyChip>> = {}) => {
  const onEdit = vi.fn();
  const onRemove = vi.fn();
  const utils = renderWithProviders(
    <PolicyChip
      canEdit
      editable
      index={0}
      onEdit={onEdit}
      onRemove={onRemove}
      policy={{ name: 'api-key-auth', version: 'v1' }}
      {...props}
    />,
  );
  return { ...utils, onEdit, onRemove };
};

beforeEach(() => {
  resetHttpClient();
  // jsdom's AbortSignal is not the one undici's fetch accepts, so the signal React
  // Query hands the hub request is dropped here; MSW still answers the request.
  const mswFetch = globalThis.fetch;
  vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) =>
    mswFetch(input, { ...init, signal: undefined }),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('PolicyChip', () => {
  it('shows the catalog display name and falls back to the handle until it arrives', async () => {
    serveVersions();
    renderChip();

    expect(screen.getByText('api-key-auth')).toBeInTheDocument();
    expect(await screen.findByText('API Key Auth')).toBeInTheDocument();
    expect(screen.queryByText('api-key-auth')).not.toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('keeps the handle when the catalog has no such policy', async () => {
    server.use(
      http.get(
        `${runtimeConfig.policyHubBaseUrl}/policies/api-key-auth/versions`,
        () => new HttpResponse(null, { headers: cors, status: 404 }),
      ),
    );
    renderChip();

    await screen.findByLabelText('Edit policy');
    expect(screen.getByText('api-key-auth')).toBeInTheDocument();
  });

  it('edits and removes through its own buttons', async () => {
    serveVersions();
    const { user, onEdit, onRemove } = renderChip();

    await user.click(screen.getByLabelText('Edit policy'));
    await user.click(screen.getByLabelText('Remove policy'));
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it('is display-only when not editable', () => {
    serveVersions();
    renderChip({ editable: false });

    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Remove policy')).not.toBeInTheDocument();
  });

  it('disables edit while the Policy Hub cannot serve a definition', () => {
    serveVersions();
    renderChip({ canEdit: false });

    expect(screen.getByLabelText('Edit policy')).toBeDisabled();
  });
});
