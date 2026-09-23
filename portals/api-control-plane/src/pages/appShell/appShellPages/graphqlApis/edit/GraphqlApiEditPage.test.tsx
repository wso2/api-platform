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
import { accepts, aGraphQLApiDetail, recorder, resource, type Recorder } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlApiEditPage } from './GraphqlApiEditPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'countries-graphql-api';

const SAMPLE_SDL = 'type Query { countries: [String] }';

const api = aGraphQLApiDetail({
  context: '/countries',
  description: 'Public countries data',
  displayName: 'Countries GraphQL API',
  id: API,
  projectId: PROJECT,
  schemaSource: 'introspection',
  version: '1.0.0',
});

let requests: Recorder;

beforeEach(() => {
  resetHttpClient();
  requests = recorder();
});

/**
 * Fork of `apis/edit/ApiEditPage.test.tsx` for a GraphQL API. The detail route
 * is registered alongside so the redirect after a successful save is
 * observable, rather than asserted on a spy.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route element={<GraphqlApiEditPage />} path={routes.graphqlApiEdit()} />
        <Route element={<div>API overview</div>} path={routes.graphqlApi()} />
      </Routes>
    </ApiScopeProvider>,
    {
      route: routes.graphqlApiEdit(ORG, PROJECT, API),
      scope: makeConsoleScope({ params: { orgHandle: ORG, projectHandler: PROJECT } }),
    },
  );
}

describe('GraphqlApiEditPage', () => {
  it('opens the form on the API named in the URL', async () => {
    server.use(
      resource('/graphql-apis/:graphqlApiId', api),
      resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }),
    );

    renderPage();

    expect(await screen.findByDisplayValue('Countries GraphQL API')).toBeInTheDocument();
    expect(screen.getByLabelText(/Context/)).toHaveValue('/countries');
  });

  it('PUTs the API back with the edits applied, then returns to the overview', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', api));
    server.use(resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }));
    server.use(
      accepts('put', `/graphql-apis/${API}`, { ...api, displayName: 'Countries API v2' }, {
        record: requests,
      }),
    );

    const { user } = renderPage();

    const name = await screen.findByDisplayValue('Countries GraphQL API');
    await user.clear(name);
    await user.type(name, 'Countries API v2');
    await user.click(screen.getByRole('button', { name: /Save changes/ }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(`/graphql-apis/${API}`);
    expect(requests.last()?.method).toBe('PUT');

    expect(await screen.findByText('API overview')).toBeInTheDocument();
  });

  // Pins the fix for schemaSource not being persisted/echoed back: a
  // non-introspection API has no sdl/sdlUrl in GraphQLAPIDetail to resupply
  // faithfully, so a metadata-only save must resupply the already-resolved
  // SDL as schemaSource "inline" rather than forcing "introspection" (which
  // would either 400 on a non-literal upstream or silently re-derive the
  // schema from it). The request body itself (multipart/form-data) isn't
  // inspectable through this environment's fetch/MSW stack — see
  // TestGraphQLUpdate_ResupplyInlineSchemaSource_MetadataOnlyEdit_SkipsIntrospection
  // in graphql_api_test.go for the assertion on what the server actually does
  // with a resupplied inline schemaSource; this only pins that the UI
  // completes the save without erroring for a non-introspection API.
  it('resupplies the current SDL as inline when the API is not introspection-sourced', async () => {
    const inlineApi = { ...api, schemaSource: 'inline' as const };
    server.use(resource('/graphql-apis/:graphqlApiId', inlineApi));
    server.use(resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }));
    server.use(accepts('put', `/graphql-apis/${API}`, inlineApi, { record: requests }));

    const { user } = renderPage();

    const name = await screen.findByDisplayValue('Countries GraphQL API');
    await user.clear(name);
    await user.type(name, 'Countries API v2');
    await user.click(screen.getByRole('button', { name: /Save changes/ }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.method).toBe('PUT');
    expect(await screen.findByText('API overview')).toBeInTheDocument();
  });

  it('requires a version, and blocks the save once it is cleared', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', api));
    server.use(resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }));
    server.use(accepts('put', `/graphql-apis/${API}`, api, { record: requests }));

    const { user } = renderPage();

    const version = await screen.findByDisplayValue('1.0.0');
    await user.clear(version);
    await user.click(screen.getByRole('button', { name: /Save changes/ }));

    expect(await screen.findByText('Enter a version.')).toBeInTheDocument();
    expect(requests.count()).toBe(0);
  });

  it('refuses a gateway-managed API, even when reached by URL', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { ...api, readOnly: true }));
    server.use(resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }));

    renderPage();

    expect(await screen.findByText('This API cannot be edited here')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Save changes/ })).not.toBeInTheDocument();
  });

  it('shows an error state when the API cannot be found', async () => {
    server.use(resource('/graphql-apis/:graphqlApiId', { status: 'error' }, { status: 404 }));
    server.use(resource('/graphql-apis/:graphqlApiId/sdl', { sdl: SAMPLE_SDL }));

    renderPage();

    expect(await screen.findByText('GraphQL API not found')).toBeInTheDocument();
  });
});
