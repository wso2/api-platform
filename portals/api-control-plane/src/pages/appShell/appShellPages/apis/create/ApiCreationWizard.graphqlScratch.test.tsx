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
import { accepts, collection } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiCreationWizard } from './ApiCreationWizard';

/**
 * The wizard's GraphQL "Start from scratch" step, end to end through the
 * wizard's own footer. Unlike `ApiCreationWizard.test.tsx`, the real
 * `GraphqlDefinePanel` is mounted here — what's under test is whether the real
 * source step lets the footer's Continue through. Only the API-type cards are
 * stubbed, since choosing a type isn't what this file is about.
 */
vi.mock('./components/ApiTypeSelector', () => ({
  ApiTypeSelector: ({ onChange }: { onChange: (apiType: unknown) => void }) => (
    <button
      onClick={() =>
        onChange({
          description: { defaultMessage: 'GraphQL', id: 'test.apiType.description' },
          enabled: true,
          icon: null,
          key: 'graphql',
          title: { defaultMessage: 'GraphQL API', id: 'test.apiType.title' },
        })
      }
      type="button"
    >
      Choose GraphQL
    </button>
  ),
}));

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';
const ENDPOINT = 'https://backend.example.com/graphql';

beforeEach(() => {
  resetHttpClient();
  server.use(collection('/rest-apis', []), collection('/graphql-apis', []));
});

/** Opens the wizard on GraphQL's "Start from scratch" source step. */
const openStartFromScratch = async () => {
  const rendered = renderWithProviders(<ApiCreationWizard />, { route, scope });
  const { user } = rendered;

  await user.click(screen.getByRole('button', { name: 'Choose GraphQL' }));
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  await user.click(screen.getByRole('button', { name: /Start from scratch/ }));

  return rendered;
};

describe('ApiCreationWizard — GraphQL "Start from scratch"', () => {
  it('keeps Continue disabled until an endpoint is entered', async () => {
    await openStartFromScratch();

    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled();
  });

  it('enables Continue for an endpoint with introspection disabled', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', {
        message: 'introspection is disabled on this endpoint',
        resolved: false,
      }),
    );
    const { user } = await openStartFromScratch();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    // The check comes back with no schema …
    expect(await screen.findByText(/introspection may be disabled/)).toBeInTheDocument();
    // … and Continue is still available.
    expect(screen.getByRole('button', { name: 'Continue' })).toBeEnabled();
  });

  it('enables Continue even when the background check itself fails', async () => {
    server.use(
      accepts('post', '/graphql-apis/validate-schema', { status: 'error' }, { status: 500 }),
    );
    const { user } = await openStartFromScratch();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);

    await waitFor(() =>
      expect(screen.queryByText('Checking the endpoint for a schema…')).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: 'Continue' })).toBeEnabled();
  });

  it('carries the endpoint into the configure step when introspection is disabled', async () => {
    server.use(accepts('post', '/graphql-apis/validate-schema', { resolved: false }));
    const { user } = await openStartFromScratch();

    await user.type(screen.getByLabelText(/Endpoint URL/), ENDPOINT);
    await screen.findByText(/introspection may be disabled/);
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByLabelText(/Query and Mutation URL/)).toHaveValue(ENDPOINT);
  });
});
