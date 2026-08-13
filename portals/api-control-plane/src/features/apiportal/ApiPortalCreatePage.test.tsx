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

import { renderWithProviders, screen, waitFor } from '../../test/utils';

// Capture the create mutation so we can assert the submitted input and drive
// success/error callbacks the same way the real mutation would invoke them.
const { mutate } = vi.hoisted(() => ({ mutate: vi.fn() }));
vi.mock('../../api/hooks/useMvpQueries', () => ({
  useCreateApiPortal: () => ({ mutate, isPending: false }),
}));

import { ApiPortalCreatePage } from './ApiPortalCreatePage';

const ORG = 'acme';
const ROUTE = `/organizations/${ORG}/api-portal/new`;

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/api-portal/new"
        element={<ApiPortalCreatePage />}
      />
      <Route
        path="/organizations/:orgHandle/api-portal"
        element={<div>API Portal List</div>}
      />
    </Routes>,
    { route: ROUTE }
  );
}

function submitButton() {
  return screen.getByRole('button', { name: 'Provision API Portal' });
}

// FormLabel here isn't wired to its TextField via `htmlFor`/`aria-labelledby`,
// so getByLabelText can't find these fields — locate the input via the
// shared FormControl wrapper instead.
function getFieldInput(labelText: string): HTMLInputElement {
  const label = screen.getByText(labelText, { selector: 'label' });
  return label.parentElement!.querySelector('input') as HTMLInputElement;
}

describe('ApiPortalCreatePage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('disables submit until name, identifier, and URL are all valid', async () => {
    const { user } = renderPage();
    expect(submitButton()).toBeDisabled();

    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    // Identifier auto-derives from the name, but the URL is still missing.
    expect(submitButton()).toBeDisabled();

    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'https://api-portal.example.com'
    );
    expect(submitButton()).toBeEnabled();
  });

  it('auto-derives a slugified, locked identifier from the name', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Prod API Portal!!'
    );

    const identifierField = screen.getByPlaceholderText(
      'prod-api-portal'
    ) as HTMLInputElement;
    expect(identifierField.value).toBe('prod-api-portal');
    expect(identifierField).toHaveAttribute('readonly');
  });

  it('rejects an invalid identifier once unlocked', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'AB'
    );

    await user.click(screen.getByRole('button', { name: 'Edit identifier' }));
    const identifierField = screen.getByPlaceholderText('prod-api-portal');
    await user.clear(identifierField);
    await user.type(identifierField, 'AB');

    expect(
      screen.getByText('Lowercase letters, numbers, hyphens only; 3–64 chars.')
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it('rejects an invalid URL', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'not-a-url'
    );
    expect(screen.getByText('Enter a valid URL')).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it('requires IdP fields before enabling submit when IdP auth is selected', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'https://api-portal.example.com'
    );
    expect(submitButton()).toBeEnabled();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: 'IdP Client Credentials' }));

    // Local auth was valid, but switching to IdP auth requires its own fields.
    expect(submitButton()).toBeDisabled();

    await user.type(
      screen.getByPlaceholderText('https://idp.example.com/oauth2/token'),
      'https://idp.example.com/oauth2/token'
    );
    expect(submitButton()).toBeDisabled();

    await user.type(getFieldInput('Client ID'), 'client-123');
    expect(submitButton()).toBeDisabled();

    await user.type(getFieldInput('Client secret'), 'shh');
    expect(submitButton()).toBeEnabled();
  });

  it('submits a local-auth CreateApiPortalInput', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      '  https://api-portal.example.com  '
    );

    await user.click(submitButton());

    expect(mutate).toHaveBeenCalledWith(
      {
        name: 'Production API Portal',
        handle: 'production-api-portal',
        url: 'https://api-portal.example.com',
        description: undefined,
        authType: 'local',
      },
      expect.any(Object)
    );
  });

  it('submits an IdP-auth CreateApiPortalInput with trimmed fields', async () => {
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'https://api-portal.example.com'
    );

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: 'IdP Client Credentials' }));
    await user.type(
      screen.getByPlaceholderText('https://idp.example.com/oauth2/token'),
      '  https://idp.example.com/oauth2/token  '
    );
    await user.type(getFieldInput('Client ID'), '  client-123  ');
    await user.type(getFieldInput('Client secret'), 'shh');

    await user.click(submitButton());

    expect(mutate).toHaveBeenCalledWith(
      {
        name: 'Production API Portal',
        handle: 'production-api-portal',
        url: 'https://api-portal.example.com',
        description: undefined,
        authType: 'idp_client_credentials',
        stsTokenUrl: 'https://idp.example.com/oauth2/token',
        clientId: 'client-123',
        clientSecret: 'shh',
      },
      expect.any(Object)
    );
  });

  it('navigates to the API Portal list on success', async () => {
    mutate.mockImplementation((_input, { onSuccess }) =>
      onSuccess({ id: 'portal-1', name: 'Production API Portal' })
    );
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'https://api-portal.example.com'
    );

    await user.click(submitButton());

    await waitFor(() =>
      expect(screen.getByText('API Portal List')).toBeInTheDocument()
    );
  });

  it('notifies and stays on the page on failure', async () => {
    mutate.mockImplementation((_input, { onError }) =>
      onError(new Error('Handle already in use'))
    );
    const { user } = renderPage();
    await user.type(
      screen.getByPlaceholderText('Production API Portal'),
      'Production API Portal'
    );
    await user.type(
      screen.getByPlaceholderText('https://api-portal.example.com'),
      'https://api-portal.example.com'
    );

    await user.click(submitButton());

    await screen.findByText('Handle already in use');
    expect(
      screen.getByPlaceholderText('Production API Portal')
    ).toBeInTheDocument();
  });
});
