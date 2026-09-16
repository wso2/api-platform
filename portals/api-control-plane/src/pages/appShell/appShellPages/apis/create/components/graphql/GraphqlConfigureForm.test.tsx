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

import { resetHttpClient } from '@/api/core/http';
import { collection } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlConfigureForm } from './GraphqlConfigureForm';

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';

type FormProps = Parameters<typeof GraphqlConfigureForm>[0];

const renderForm = (
  initialValues?: FormProps['initialValues'],
  serverErrors?: FormProps['serverErrors'],
) =>
  renderWithProviders(
    <GraphqlConfigureForm
      initialValues={initialValues}
      onBack={() => {}}
      onSubmit={() => {}}
      serverErrors={serverErrors}
    />,
    { route, scope },
  );

/** What the wizard hands back after a rejected create. */
const submitted = {
  context: '/countries-api/v1.0.0',
  displayName: 'Countries API',
  endpointUrl: 'https://backend.example.com/graphql',
  id: 'countries-api',
  schemaSource: 'introspection' as const,
  version: '1.0.0',
};

/** The identifier field probes availability as it settles. */
beforeEach(() => {
  resetHttpClient();
  server.use(collection('/graphql-apis', []));
});

describe('GraphqlConfigureForm — initial values', () => {
  it('derives an identifier and single-path context from the display name and version', () => {
    renderForm({ displayName: 'Countries API', version: '1.0.0', schemaSource: 'introspection' });

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('countries-api');
    // GraphQL has one route for every operation — no project-prefixed base
    // path the way REST's context is built — and always ends in `/graphql`,
    // the platform's documented convention for a GraphQL endpoint's context.
    expect(screen.getByLabelText(/Context/)).toHaveValue('/countries-api/v1.0.0/graphql');
  });

  it('keeps a restored identifier and context the draft already carries', () => {
    renderForm(submitted);

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('countries-api');
    expect(screen.getByLabelText(/Context/)).toHaveValue('/countries-api/v1.0.0');
    expect(screen.getByLabelText(/Query and Mutation URL/)).toHaveValue(
      'https://backend.example.com/graphql',
    );
  });

  it('leaves a restored context alone when the name is edited afterwards', async () => {
    const { user } = renderForm(submitted);

    await user.type(screen.getByLabelText(/^Name/), ' v2');

    expect(screen.getByLabelText(/Identifier/)).toHaveValue('countries-api');
    expect(screen.getByLabelText(/Context/)).toHaveValue('/countries-api/v1.0.0');
  });
});

describe('GraphqlConfigureForm — client-side validation', () => {
  it('requires the endpoint URL to be a full http(s) address', async () => {
    const onSubmit = vi.fn();
    const { user } = renderWithProviders(
      <GraphqlConfigureForm
        initialValues={{ displayName: 'Countries API', schemaSource: 'introspection' }}
        onBack={() => {}}
        onSubmit={onSubmit}
      />,
      { route, scope },
    );

    const endpoint = screen.getByLabelText(/Query and Mutation URL/);
    await user.type(endpoint, 'not-a-url');
    await user.tab();

    expect(
      await screen.findByText('Enter a full URL, for example https://api.example.com/graphql.'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Create' }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('submits once every required field is filled in', async () => {
    const onSubmit = vi.fn();
    const { user } = renderWithProviders(
      <GraphqlConfigureForm
        initialValues={{ schemaSource: 'introspection' }}
        onBack={() => {}}
        onSubmit={onSubmit}
      />,
      { route, scope },
    );

    await user.type(screen.getByLabelText(/^Name/), 'Countries API');
    await user.type(screen.getByLabelText(/Query and Mutation URL/), 'https://backend.example.com/graphql');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        displayName: 'Countries API',
        endpointUrl: 'https://backend.example.com/graphql',
        id: 'countries-api',
      }),
    );
  });
});

describe('GraphqlConfigureForm — a rejected submission', () => {
  it('shows the server’s reason on the field it names', async () => {
    renderForm(submitted, {
      fields: { id: 'A GraphQL API with this identifier already exists.' },
      unmapped: [],
    });

    expect(
      await screen.findByText('A GraphQL API with this identifier already exists.'),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/Identifier/)).toHaveAccessibleDescription(
      'A GraphQL API with this identifier already exists.',
    );
  });

  it('moves focus to the first field the server named', async () => {
    renderForm(submitted, {
      fields: { targetUrl: 'This endpoint could not be reached.' },
      unmapped: [],
    });

    await waitFor(() =>
      expect(screen.getByLabelText(/Query and Mutation URL/)).toHaveFocus(),
    );
  });

  it('retracts the message once that value is edited', async () => {
    const { user } = renderForm(submitted, {
      fields: { id: 'A GraphQL API with this identifier already exists.' },
      unmapped: [],
    });

    await user.type(screen.getByLabelText(/Identifier/), '-v2');

    expect(
      screen.queryByText('A GraphQL API with this identifier already exists.'),
    ).not.toBeInTheDocument();
  });

  it('says nothing when the last submission was not rejected', () => {
    renderForm(submitted);

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
