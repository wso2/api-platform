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
import { describe, expect, it } from 'vitest';

import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import type { ApiDetail } from '@/types/domain';
import { RoutingPanel } from './RoutingPanel';

const detail: ApiDetail = {
  id: 'orders-api',
  projectId: 'proj',
  name: 'orders',
  displayName: 'Orders',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  operations: [
    { method: 'GET', path: '/orders' },
    { method: 'GET', path: '/new' },
  ],
  policies: [],
  endpoints: { prodUrl: 'http://api.test' },
};

describe('RoutingTab backend discovery', () => {
  it('auto-maps proxy resources found in the backend contract and leaves the rest dry', async () => {
    // The backend contract only declares /orders — not /new.
    server.use(
      http.get('http://api.test/openapi.json', () =>
        HttpResponse.json({
          openapi: '3.0.0',
          paths: { '/orders': { get: {} } },
        }),
      ),
    );

    const { user } = renderWithProviders(<RoutingPanel detail={detail} />);

    // Seeded from the operations, both resources start mapped.
    expect(screen.queryByTitle(/Not connected/)).not.toBeInTheDocument();

    await user.click(screen.getByLabelText('Discover backend resources'));

    // After discovery, /orders stays mapped while /new hangs dry.
    const dry = await screen.findByTitle(/Not connected/);
    expect(dry).toHaveTextContent('/new');
    expect(screen.getAllByTitle(/Not connected/)).toHaveLength(1);
  });

  it('lets the backend URL be edited inline on the canvas, which triggers discovery', async () => {
    server.use(
      http.get('http://api.test/openapi.json', () =>
        HttpResponse.json({
          openapi: '3.0.0',
          paths: { '/orders': { get: {} } },
        }),
      ),
    );

    const noUrl: ApiDetail = { ...detail, endpoints: {} };
    const { user } = renderWithProviders(<RoutingPanel detail={noUrl} />);

    // The canvas holder is a real, editable input (not a static label).
    const field = screen.getByLabelText('Backend URL') as HTMLInputElement;
    expect(field).toBeEnabled();
    expect(field.readOnly).toBe(false);

    await user.type(field, 'http://api.test');
    expect(field).toHaveValue('http://api.test');

    // Typing a new URL auto-runs discovery (debounced): /new has no backend
    // counterpart and is left dry.
    const dry = await screen.findByTitle(/Not connected/, undefined, {
      timeout: 3000,
    });
    expect(dry).toHaveTextContent('/new');
  });
});
