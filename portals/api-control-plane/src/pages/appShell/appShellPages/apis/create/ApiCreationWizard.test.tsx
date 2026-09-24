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
import { useEffect } from 'react';

import { resetHttpClient } from '@/api/core/http';
import { accepts, collection, failure, recorder } from '@/test/msw';
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

const STUB_SPEC_FILE = new File(
  ['{"openapi":"3.0.3","info":{"title":"Orders API","version":"1.0.0"},"paths":{}}'],
  'orders-api.json',
  { type: 'application/json' },
);

vi.mock('./components/DefineApiPanel', () => ({
  DefineApiPanel: ({ onDraftChange }: { onDraftChange: (draft: unknown) => void }) => {
    useEffect(() => () => onDraftChange(null), [onDraftChange]);

    return (
      <>
        <button
          onClick={() =>
            onDraftChange({
              displayName: 'Orders API',
              version: '1.0',
              contractImport: { specFile: STUB_SPEC_FILE },
            })
          }
          type="button"
        >
          Use this contract
        </button>
        {/* What designing from scratch hands over: a placeholder backend. */}
        <button
          onClick={() =>
            onDraftChange({
              contractImport: { specFile: STUB_SPEC_FILE },
              displayName: 'Untitled API',
              upstream: { main: { url: 'https://example.com' } },
              version: '1.0',
            })
          }
          type="button"
        >
          Start from scratch
        </button>
      </>
    );
  },
}));

const scope = makeConsoleScope();
const route = '/organizations/api-platform-demo/projects/retail-apis/apis/create';

beforeEach(() => {
  resetHttpClient();
  server.use(collection('/rest-apis', []));
  server.use(accepts('post', '/rest-apis/validate-openapi', { isValid: true, errors: [] }));
});

/** Runs the wizard as far as a submitted create request. */
const submitCreate = async () => {
  const rendered = renderWithProviders(<ApiCreationWizard />, { route, scope });
  const { user } = rendered;

  await user.click(screen.getByRole('button', { name: 'Choose REST' }));
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  await user.click(screen.getByRole('button', { name: 'Use this contract' }));
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  await user.type(screen.getByLabelText(/Target URL/), 'https://orders.example.com');
  await user.click(screen.getByRole('button', { name: 'Create' }));

  return rendered;
};

describe('ApiCreationWizard — explicit creation boundary', () => {
  it('preserves the selected source when returning from configuration', async () => {
    const { user } = renderWithProviders(<ApiCreationWizard />, { route, scope });

    await user.click(screen.getByRole('button', { name: 'Choose REST' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: 'Use this contract' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: 'Back' }));

    expect(screen.getByRole('button', { name: 'Continue' })).toBeEnabled();
  });

  it('shows Step 3 without posting when Continue is clicked, then posts on Create', async () => {
    const createRequests = recorder();
    server.use(accepts('post', '/rest-apis/import-openapi', { id: 'orders-api' }, { record: createRequests }));
    const { user } = renderWithProviders(<ApiCreationWizard />, { route, scope });

    await user.click(screen.getByRole('button', { name: 'Choose REST' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: 'Use this contract' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(screen.getByText('Step 3 of 3')).toBeInTheDocument();
    expect(screen.getByLabelText(/Target URL/)).toBeInTheDocument();
    expect(createRequests.count()).toBe(0);

    await user.type(screen.getByLabelText(/Target URL/), 'https://orders.example.com');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    expect(createRequests.count()).toBe(1);
  });
});

describe('ApiCreationWizard — a rejected create', () => {
  it('returns to the form with the reason on the field, when the user can fix it', async () => {
    server.use(
      failure('post', '/rest-apis/import-openapi', 409, 'CONFLICT', {
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
      failure('post', '/rest-apis/import-openapi', 400, 'VALIDATION_FAILED', {
        errors: [{ field: 'upstream.main.url', message: 'Must be reachable over https.' }],
      }),
    );

    await submitCreate();

    expect(await screen.findByText('Must be reachable over https.')).toBeInTheDocument();
    expect(screen.getByLabelText(/Target URL/)).toHaveValue('https://orders.example.com');
  });

  it('does not call the backend a placeholder again once the user has chosen it', async () => {
    // The form unmounts while the progress screen stands in for it, so the
    // "user has been into this field" provenance has to outlive it — otherwise
    // a rejected create returns a form that calls the user's own URL a
    // placeholder, purely because it happens to match the skeleton's.
    server.use(
      failure('post', '/rest-apis/import-openapi', 400, 'VALIDATION_FAILED', {
        errors: [{ field: 'context', message: 'Context is already in use.' }],
      }),
    );
    const { user } = renderWithProviders(<ApiCreationWizard />, { route, scope });

    await user.click(screen.getByRole('button', { name: 'Choose REST' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));
    await user.click(screen.getByRole('button', { name: 'Start from scratch' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    const placeholderNotice = /using https:\/\/example\.com as a placeholder backend/i;
    expect(screen.getByText(placeholderNotice)).toBeInTheDocument();

    // Deliberately settling on the same URL retires the notice.
    const targetUrl = screen.getByLabelText(/Target URL/);
    await user.clear(targetUrl);
    await user.type(targetUrl, 'https://example.com');
    expect(screen.queryByText(placeholderNotice)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Create' }));

    expect(await screen.findByText('Context is already in use.')).toBeInTheDocument();
    expect(screen.getByLabelText(/Target URL/)).toHaveValue('https://example.com');
    expect(screen.queryByText(placeholderNotice)).not.toBeInTheDocument();
  });

  it('stays on the progress screen for a failure no edit can fix', async () => {
    // A 500 is not the form's problem: sending the user back to retype fields
    // that were never wrong would be a lie about what went wrong.
    server.use(failure('post', '/rest-apis/import-openapi', 500, 'INTERNAL_ERROR'));

    await submitCreate();

    expect(await screen.findByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.queryByLabelText(/Target URL/)).not.toBeInTheDocument();
  });
});
