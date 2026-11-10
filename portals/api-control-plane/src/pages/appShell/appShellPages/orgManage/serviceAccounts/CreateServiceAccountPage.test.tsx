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

import { useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aServiceAccount,
  aRole,
  accepts,
  collection,
  failure,
  recorder,
  type Recorder,
} from '@/test/msw';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { CreateServiceAccountPage } from './CreateServiceAccountPage';
import { clearCredentials, peekCredentials } from './createdCredentials';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';
const FORM_TIMEOUT = 15_000;
const SECRET = 'apsa_' + '7'.repeat(64);

const account = aServiceAccount();
const roles = [
  aRole({ name: 'ap_admin', scopes: ['ap:organization:manage'] }),
  aRole(),
  aRole({ name: 'ap_deployer', scopes: ['ap:rest_api:deployment:manage', 'ap:rest_api:read'] }),
];
const credentials = {
  clientId: account.clientId,
  clientSecret: SECRET,
  serviceAccount: account,
};

let requests: Recorder;

const LocationProbe = () => <output data-testid="location">{useLocation().pathname}</output>;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <CreateServiceAccountPage />
      <LocationProbe />
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/service-accounts/create`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  resetTrackedFeatures();
  server.use(collection('/roles', roles));
});

describe('CreateServiceAccountPage', () => {
  it('shows the form with the default role preselected and a summary', async () => {
    renderPage();

    expect(await screen.findByRole('heading', { name: 'New service account' })).toBeInTheDocument();
    expect(screen.getByLabelText(/Display name/)).toBeInTheDocument();
    expect(await screen.findByRole('checkbox', { name: 'ap_service_account' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'ap_deployer' })).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Create service account' })).toBeDisabled();
  });

  it('lists the default role first', async () => {
    renderPage();

    const boxes = await screen.findAllByRole('checkbox');
    expect(boxes[0]).toHaveAccessibleName('ap_service_account');
    expect(boxes[0]).toBeChecked();
  });

  it('goes back to the list from the back button and from Cancel', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Service accounts' }));
    expect(screen.getByTestId('location')).toHaveTextContent(
      `/organizations/${ORG}/service-accounts`,
    );
    expect(screen.getByTestId('location')).not.toHaveTextContent('/create');
  });

  it('cancels back to the list', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Cancel' }));
    expect(screen.getByTestId('location')).not.toHaveTextContent('/create');
  });

  it('updates the summary and scope total as roles are picked', async () => {
    const { user } = renderPage();

    await screen.findByRole('checkbox', { name: 'ap_deployer' });
    await user.type(screen.getByLabelText(/Display name/), 'billing-sync');
    expect(screen.getAllByText('billing-sync').length).toBeGreaterThan(0);
    await user.click(screen.getByRole('checkbox', { name: 'ap_deployer' }));
    await waitFor(() => expect(screen.getByText('scopes granted')).toBeInTheDocument());

    await user.click(screen.getByRole('checkbox', { name: 'ap_service_account' }));
    await user.click(screen.getByRole('checkbox', { name: 'ap_deployer' }));
    expect(screen.getByText('None selected')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create service account' })).toBeDisabled();
  });

  it('lists a role’s scopes when it is expanded', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Show scopes of ap_deployer' }));
    expect(await screen.findByText('ap:rest_api:deployment:manage')).toBeInTheDocument();
  });

  it(
    'creates the account and moves to the created page, holding the secret in memory',
    async () => {
      server.use(accepts('post', '/service-accounts', credentials, { record: requests }));
      const { user } = renderPage();

      await screen.findByRole('checkbox', { name: 'ap_service_account' });
      await user.type(screen.getByLabelText(/Display name/), 'CI deployer');
      await user.type(screen.getByLabelText(/Description/), 'Deploys APIs');
      await user.click(screen.getByRole('button', { name: 'Create service account' }));

      await waitFor(() => expect(requests.calls.some((call) => call.method === 'POST')).toBe(true));
      expect(JSON.parse(requests.calls.find((call) => call.method === 'POST')!.body)).toEqual({
        description: 'Deploys APIs',
        displayName: 'CI deployer',
        roles: ['ap_service_account'],
      });
      expect(trackedFeatures()).toEqual([
        ['apip.security.service-accounts', { operation: 'create' }],
      ]);
      await waitFor(() =>
        expect(screen.getByTestId('location')).toHaveTextContent(
          `/organizations/${ORG}/service-accounts/created`,
        ),
      );
      expect(peekCredentials(ORG)?.clientSecret).toBe(SECRET);
      clearCredentials();
    },
    FORM_TIMEOUT,
  );

  it(
    'omits an empty description',
    async () => {
      server.use(accepts('post', '/service-accounts', credentials, { record: requests }));
      const { user } = renderPage();

      await screen.findByRole('checkbox', { name: 'ap_service_account' });
      await user.type(screen.getByLabelText(/Display name/), 'CI deployer');
      await user.click(screen.getByRole('button', { name: 'Create service account' }));

      await waitFor(() => expect(requests.calls.some((call) => call.method === 'POST')).toBe(true));
      expect(JSON.parse(requests.calls.find((call) => call.method === 'POST')!.body)).toEqual({
        displayName: 'CI deployer',
        roles: ['ap_service_account'],
      });
    },
    FORM_TIMEOUT,
  );

  it(
    'stays on the page and shows the error when the create is refused',
    async () => {
      server.use(
        failure('post', '/service-accounts', 409, 'SERVICE_ACCOUNT_EXISTS', {
          message: 'Name clash, try again.',
        }),
      );
      const { user } = renderPage();

      await screen.findByRole('checkbox', { name: 'ap_service_account' });
      await user.type(screen.getByLabelText(/Display name/), 'CI deployer');
      await user.click(screen.getByRole('button', { name: 'Create service account' }));

      expect(await screen.findByText('Name clash, try again.')).toBeInTheDocument();
      expect(screen.getByTestId('location')).toHaveTextContent('/create');
    },
    FORM_TIMEOUT,
  );

  it('measures the name in bytes, as the server does', async () => {
    const { user } = renderPage();

    await user.click(await screen.findByLabelText(/Display name/));
    // 100 characters, 300 bytes.
    await user.paste('界'.repeat(100));

    expect(await screen.findByText(/Too long: the limit is 255 bytes/)).toBeInTheDocument();
  });

  it('says so when the roles cannot be loaded', async () => {
    server.use(failure('get', '/roles', 500, 'INTERNAL_ERROR'));
    renderPage();

    expect(await screen.findByText(/Could not load the roles/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create service account' })).toBeDisabled();
  });
});
