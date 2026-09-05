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
import { collection, failure } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { ApiCreationWizard } from './ApiCreationWizard';

/*
 * The first two steps are stubbed to a single button each. What they collect
 * is covered by their own suites; these tests are about where a *rejected*
 * create leaves the user, and reaching that point through the real pickers
 * would mean rendering Swagger UI and Monaco to get there.
 */
vi.mock('./components/ApiTypeSelector', () => ({
  ApiTypeSelector: ({ onChange }: { onChange: (apiType: unknown) => void }) => (
    <button
      onClick={() =>
        onChange({
          description: { defaultMessage: 'REST', id: 'test.apiType.description' },
          enabled: true,
          icon: null,
          key: 'rest',
          title: { defaultMessage: 'REST API', id: 'test.apiType.title' },
        })
      }
      type="button"
    >
      Choose REST
    </button>
  ),
}));

vi.mock('./components/DefineApiPanel', () => ({
  DefineApiPanel: ({ onDataFetched }: { onDataFetched: (draft: unknown) => void }) => (
    <button
      onClick={() => onDataFetched({ displayName: 'Orders API', version: '1.0' })}
      type="button"
    >
      Use this contract
    </button>
  ),
}));

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';

beforeEach(() => {
  resetHttpClient();
  server.use(collection('/rest-apis', []));
});

/** Runs the wizard as far as a submitted create request. */
const submitCreate = async () => {
  const rendered = renderWithProviders(<ApiCreationWizard />, { route, scope });
  const { user } = rendered;

  await user.click(screen.getByRole('button', { name: 'Choose REST' }));
  await user.click(screen.getByRole('button', { name: 'Use this contract' }));
  await user.type(screen.getByLabelText(/Target URL/), 'https://orders.example.com');
  await user.click(screen.getByRole('button', { name: 'Create' }));

  return rendered;
};

describe('ApiCreationWizard — a rejected create', () => {
  it('returns to the form with the reason on the field, when the user can fix it', async () => {
    server.use(
      failure('post', '/rest-apis', 409, 'CONFLICT', {
        errors: [{ field: 'id', message: 'An API with this identifier already exists.' }],
        message: 'The API could not be created.',
      }),
    );

    await submitCreate();

    expect(
      await screen.findByText('An API with this identifier already exists.'),
    ).toBeInTheDocument();
    // Back on the form, not stranded on a progress screen that cannot say why.
    expect(screen.getByLabelText(/Identifier/)).toHaveValue('orders-api');
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
  });

  it('keeps what the user typed, so nothing has to be entered twice', async () => {
    server.use(
      failure('post', '/rest-apis', 400, 'VALIDATION_FAILED', {
        errors: [{ field: 'upstream.main.url', message: 'Must be reachable over https.' }],
      }),
    );

    await submitCreate();

    expect(await screen.findByText('Must be reachable over https.')).toBeInTheDocument();
    expect(screen.getByLabelText(/Target URL/)).toHaveValue('https://orders.example.com');
  });

  it('stays on the progress screen for a failure no edit can fix', async () => {
    // A 500 is not the form's problem: sending the user back to retype fields
    // that were never wrong would be a lie about what went wrong.
    server.use(failure('post', '/rest-apis', 500, 'INTERNAL_ERROR'));

    await submitCreate();

    expect(await screen.findByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.queryByLabelText(/Target URL/)).not.toBeInTheDocument();
  });
});
