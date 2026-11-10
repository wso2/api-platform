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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { accepts, aRestApi, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { EndpointsPanel } from './EndpointsPanel';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'pizza-shack';

const api = aRestApi({
  displayName: 'Pizza Shack',
  id: API,
  upstream: { main: { url: 'https://upstream.test' } },
});

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  resetTrackedFeatures();
});

function renderPanel() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <EndpointsPanel api={api} />
    </ApiScopeProvider>,
  );
}

describe('EndpointsPanel — feature usage', () => {
  it('reports an endpoint update when the endpoint drawer is saved', async () => {
    server.use(accepts('put', `/rest-apis/${API}`, api, { record: requests }));
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Edit endpoint' }));
    const endpointField = await screen.findByDisplayValue('https://upstream.test');
    await user.clear(endpointField);
    await user.type(endpointField, 'https://new-upstream.test');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(trackedFeatures()).toEqual([
      ['apip.manage-apis.api-endpoint-update-click', { api_type: 'rest-api' }],
    ]);
  });

  it('reports nothing when the drawer is closed without saving', async () => {
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Edit endpoint' }));
    await user.clear(await screen.findByDisplayValue('https://upstream.test'));
    await user.type(screen.getByRole('textbox'), 'https://new-upstream.test');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(trackedFeatures()).toEqual([]);
  });
});
