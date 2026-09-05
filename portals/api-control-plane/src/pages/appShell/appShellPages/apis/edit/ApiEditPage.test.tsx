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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import {
  accepts,
  aRestApi,
  recorder,
  resource,
  type Recorder,
  type RestApiFixture,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiEditPage } from './ApiEditPage';

const ORG = 'acme-org';
const PROJECT = 'retail';
const API = 'pizza-shack';

const anApi = (overrides: Partial<RestApiFixture> = {}) =>
  aRestApi({
    context: '/pizza',
    description: 'Pizza ordering',
    displayName: 'Pizza Shack',
    id: API,
    projectId: PROJECT,
    upstream: { main: { url: 'https://upstream.test' } },
    version: '1.0.0',
    ...overrides,
  });

let requests: Recorder;

beforeEach(() => {
  resetHttpClient();
  requests = recorder();
});

/**
 * The page's hooks read `ApiScopeContext` rather than the console scope, so the
 * provider is mounted here — without it the detail query stays
 * `enabled: false` and the page renders its loading state forever.
 *
 * The detail route is registered alongside so the redirect after a successful
 * save is observable, rather than asserted on a spy.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <Routes>
        <Route element={<ApiEditPage />} path={routes.apiEdit()} />
        <Route element={<div>API overview</div>} path={routes.api()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: routes.apiEdit(ORG, PROJECT, API),
      scope: makeConsoleScope({
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

describe('ApiEditPage', () => {
  it('opens the form on the API named in the URL', async () => {
    server.use(resource(`/rest-apis/${API}`, anApi()));

    renderPage();

    expect(await screen.findByDisplayValue('Pizza Shack')).toBeInTheDocument();
    expect(screen.getByLabelText(/Context/)).toHaveValue('/pizza');
  });

  it('PUTs the whole API back with the edits applied, then returns to the overview', async () => {
    server.use(resource(`/rest-apis/${API}`, anApi({ operations: [], transport: ['https'] })));
    server.use(
      accepts('put', `/rest-apis/${API}`, anApi({ displayName: 'Pizza Palace' }), {
        record: requests,
      }),
    );

    const { user } = renderPage();

    const name = await screen.findByDisplayValue('Pizza Shack');
    await user.clear(name);
    await user.type(name, 'Pizza Palace');
    await user.click(screen.getByRole('button', { name: /Save changes/ }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body) as RestApiFixture;

    expect(body.displayName).toBe('Pizza Palace');
    // Fields the form doesn't collect have to survive the round trip: the
    // spec's update body is the whole `RESTAPI`, not a patch.
    expect(body.id).toBe(API);
    expect(body.transport).toEqual(['https']);
    expect(body.upstream.main.url).toBe('https://upstream.test');

    expect(await screen.findByText('API overview')).toBeInTheDocument();
  });

  it('keeps a shared upstream `ref` intact instead of writing a url beside it', async () => {
    server.use(
      resource(`/rest-apis/${API}`, anApi({ upstream: { main: { ref: 'retail-backend' } } })),
    );
    server.use(accepts('put', `/rest-apis/${API}`, anApi(), { record: requests }));

    const { user } = renderPage();

    const name = await screen.findByDisplayValue('Pizza Shack');
    await user.clear(name);
    await user.type(name, 'Pizza Palace');
    await user.click(screen.getByRole('button', { name: /Save changes/ }));

    await waitFor(() => expect(requests.count()).toBe(1));
    const body = JSON.parse(requests.last()!.body) as RestApiFixture;

    expect(body.upstream).toEqual({ main: { ref: 'retail-backend' } });
  });

  it('refuses a gateway-managed API, even when reached by URL', async () => {
    server.use(resource(`/rest-apis/${API}`, anApi({ readOnly: true })));

    renderPage();

    expect(await screen.findByText('This API cannot be edited here')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Save changes/ })).not.toBeInTheDocument();
  });
});
