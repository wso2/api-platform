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
import { accepts, recorder, type Recorder } from '@/test/msw';
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

describe('GraphqlIntrospectionForm — validation', () => {
  it('requires an endpoint before checking', async () => {
    const { onResolved, user } = renderForm();

    await user.click(screen.getByRole('button', { name: 'Fetch' }));

    expect(
      await screen.findByText('Enter the GraphQL endpoint to introspect.'),
    ).toBeInTheDocument();
    expect(onResolved).not.toHaveBeenCalled();
  });

  it('rejects a non-URL value once the field is touched', async () => {
    const { user } = renderForm();

    await user.type(screen.getByLabelText(/Backend endpoint/), 'not-a-url');
    await user.tab();

    expect(await screen.findByText('Enter a valid HTTP or HTTPS URL.')).toBeInTheDocument();
  });
});

describe('GraphqlIntrospectionForm — a successful check', () => {
  it('reports the resolved schema and shows the type count', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/validate-schema',
        { resolved: true, sdl: 'type Query { hello: String greet(name: String): String }' },
        { record: requests },
      ),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Backend endpoint/), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch' }));

    await waitFor(() =>
      expect(onResolved).toHaveBeenCalledWith({
        endpointUrl: 'https://backend.example.com/graphql',
        schemaSource: 'introspection',
        sdl: 'type Query { hello: String greet(name: String): String }',
      }),
    );
    expect(requests.count()).toBe(1);
    // Query + String, plus Boolean pulled in by the always-present
    // @skip/@include directives (see graphqlSchema.test.ts's countNamedTypes
    // coverage for why) = 3 named types for this tiny schema.
    expect(await screen.findByText(/3 types/)).toBeInTheDocument();
  });
});

describe('GraphqlIntrospectionForm — a failed check', () => {
  it('shows the unresolved message and reports null when introspection could not derive a schema', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: false }));
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Backend endpoint/), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch' }));

    expect(
      await screen.findByText(
        'Could not derive a schema from that endpoint. Check the URL, and that introspection is enabled.',
      ),
    ).toBeInTheDocument();
    expect(onResolved).toHaveBeenLastCalledWith(null);
  });

  it('reports null and shows nothing extra when the request itself fails', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', { status: 'error' }, { status: 500 }),
    );
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Backend endpoint/), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch' }));

    await waitFor(() => expect(onResolved).toHaveBeenLastCalledWith(null));
    expect(
      screen.queryByText(
        'Could not derive a schema from that endpoint. Check the URL, and that introspection is enabled.',
      ),
    ).not.toBeInTheDocument();
  });

  it('clears a prior result and re-checks when the endpoint is edited, re-enabling Check', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: true, sdl: 'type Query { a: String }' }));
    const { onResolved, user } = renderForm();

    await user.type(screen.getByLabelText(/Backend endpoint/), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Fetch' }));
    await waitFor(() =>
      expect(onResolved).toHaveBeenLastCalledWith({
        endpointUrl: 'https://backend.example.com/graphql',
        schemaSource: 'introspection',
        sdl: 'type Query { a: String }',
      }),
    );
    expect(screen.getByRole('button', { name: 'Fetch' })).toBeDisabled();

    await user.type(screen.getByLabelText(/Backend endpoint/), '2');

    expect(onResolved).toHaveBeenLastCalledWith(null);
    expect(screen.queryByText(/types in this schema/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Fetch' })).toBeEnabled();
  });
});
