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
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';

// Capture the create mutation so we can assert the submitted input.
const { mutateAsync } = vi.hoisted(() => ({ mutateAsync: vi.fn() }));
vi.mock('../../api/hooks/useMvpQueries', () => ({
  useCreateApi: () => ({ mutateAsync, isPending: false, error: null }),
}));

import { ApiCreatePage } from './ApiCreatePage';

const ROUTE = '/organizations/acme/projects/retail/apis/new';

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis/new"
        element={<ApiCreatePage />}
      />
    </Routes>,
    { route: ROUTE }
  );
}

describe('ApiCreatePage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders the type + method selection with only HTTP/start methods enabled', () => {
    renderPage();
    expect(screen.getAllByText('HTTP').length).toBeGreaterThan(0);
    expect(screen.getByText('Import API Contract')).toBeInTheDocument();
    expect(screen.getByText('Start from Scratch')).toBeInTheDocument();
    // GraphQL/WebSocket/etc are "Soon"; GenAI is "Coming soon".
    expect(screen.getAllByText('Soon').length).toBeGreaterThan(0);
    expect(screen.getByText('Coming soon')).toBeInTheDocument();
  });

  it('start-from-scratch goes to details and disables Create until valid', async () => {
    const { user } = renderPage();
    await user.click(screen.getByText('Start from Scratch'));

    // Details phase rendered.
    expect(screen.getByText('Create an API Proxy')).toBeInTheDocument();
    const createButton = screen.getByRole('button', { name: 'Create' });
    expect(createButton).toBeDisabled();

    // Display name only → still disabled (scratch requires a backend URL).
    await user.type(screen.getByLabelText(/Display name/), 'Pizza Shack API');
    expect(createButton).toBeDisabled();
  });

  it('submits the built CreateApiInput once details are valid', async () => {
    mutateAsync.mockResolvedValue({ handler: 'pizza-shack-api' });
    const { user } = renderPage();
    await user.click(screen.getByText('Start from Scratch'));

    await user.type(screen.getByLabelText(/Display name/), 'Pizza Shack API');
    await user.type(
      screen.getByLabelText(/Target URL/),
      'https://backend.example.com'
    );

    const createButton = screen.getByRole('button', { name: 'Create' });
    await waitFor(() => expect(createButton).toBeEnabled());
    await user.click(createButton);

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1));
    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'API_PROXY',
        displayName: 'Pizza Shack API',
        name: 'pizza-shack-api',
        version: '1.0.0',
        prodUrl: 'https://backend.example.com',
        source: { mode: 'scratch' },
      })
    );
  });
});
