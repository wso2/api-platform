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
import { http } from 'msw';

import { accepts, apiUrl, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlIntrospectionForm } from './GraphqlIntrospectionForm';

const ORG = 'api-platform-demo';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderForm = (onResolved = vi.fn()) => {
  const rendered = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GraphqlIntrospectionForm onResolved={onResolved} />
    </ApiScopeProvider>,
  );
  return { onResolved, ...rendered };
};

const ENDPOINT = 'https://backend.example.com/graphql';
const UNRESOLVED =
  'Could not derive a schema from that endpoint — introspection may be disabled. You can still continue; the API starts with an empty schema.';

describe('GraphqlIntrospectionForm — the endpoint alone', () => {
  it('has no separate fetch step, like REST’s Start from scratch', () => {
    renderForm();

    expect(screen.queryByRole('button', { name: 'Fetch' })).not.toBeInTheDocument();
  });

  it('reports a valid endpoint as the source straight away, before any check finishes', async () => {
    // Never answers, so only the endpoint itself can have been reported.
    server.use(http.post(apiUrl('/graphql-apis/validate-schema'), () => new Promise(() => {})));
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    expect(onResolved).toHaveBeenLastCalledWith({
      endpointUrl: ENDPOINT,
      schemaSource: 'introspection',
    });
  });

  it('reports nothing usable for an empty or invalid endpoint, and says why once touched', async () => {
    const { onResolved, user } = renderForm();

    await user.click(screen.getByLabelText(/Endpoint URL/));
    await user.tab();
    expect(
      await screen.findByText('Enter the GraphQL endpoint to introspect.'),
    ).toBeInTheDocument();

    await user.type(screen.getByLabelText(/Endpoint URL/), 'not-a-url');
    expect(await screen.findByText('Enter a valid HTTP or HTTPS URL.')).toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith(null);
  });
});

describe('GraphqlIntrospectionForm — the background check', () => {
  it('checks once typing settles, and adds the schema it finds', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String greet(name: String): String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        endpointUrl: ENDPOINT,
        schemaSource: 'introspection',
        sdl: 'type Query { hello: String greet(name: String): String }',
      }),
    );
    // One request for the whole typed URL, not one per keystroke.
    expect(requests.count()).toBe(1);
    // Query + String, plus Boolean pulled in by the always-present
    // @skip/@include directives (see graphqlSchema.test.ts's countNamedTypes
    // coverage for why) = 3 named types for this tiny schema.
    expect(await screen.findByText(/3 types/)).toBeInTheDocument();
  });

  it('fills the endpoint field from the sample-URL link and checks it', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { a: String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.click(screen.getByRole('button', { name: 'Try with Sample URL' }));

    expect(screen.getByLabelText(/Endpoint URL/)).toHaveValue(
      'https://countries.trevorblades.com/graphql',
    );
    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        endpointUrl: 'https://countries.trevorblades.com/graphql',
        schemaSource: 'introspection',
        sdl: 'type Query { a: String }',
      }),
    );
    expect(requests.count()).toBe(1);
  });

  it('drops a schema found for an earlier endpoint once the field changes', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        resolved: true,
        sdl: 'type Query { a: String }',
      }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);
    await screen.findByText(/types/);

    await user.type(screen.getByLabelText(/Endpoint URL/), '2');

    expect(onResolved).toHaveBeenLastCalledWith({
      endpointUrl: `${ENDPOINT}2`,
      schemaSource: 'introspection',
    });
  });
});

describe('GraphqlIntrospectionForm — a check that finds no schema', () => {
  it('keeps the endpoint as the source, without a schema, when introspection is disabled', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        message: 'introspection is disabled',
        resolved: false,
      }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    // Explained here, under the field — the only place it is.
    expect(await screen.findByText(UNRESOLVED)).toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith({
      endpointUrl: ENDPOINT,
      schemaSource: 'introspection',
    });
  });

  it('keeps the endpoint as the source and shows nothing extra when the check itself fails', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { status: 'error' },
        { record: requests, status: 500 },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    await waitFor(() => expect(requests.count()).toBe(1));
    await waitFor(() =>
      expect(screen.queryByText('Checking the endpoint for a schema…')).not.toBeInTheDocument(),
    );
    expect(screen.queryByText(UNRESOLVED)).not.toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith({
      endpointUrl: ENDPOINT,
      schemaSource: 'introspection',
    });
  });
});
