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
import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aServiceAccount, aRole, apiUrl, collection, failure } from '@/test/msw';
import { resetTrackedFeatures, trackedFeatures } from '@/test/featureUsage';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { clearCredentials, hasHeldCredentials, holdCredentials } from './createdCredentials';
import { ServiceAccountCreatedPage } from './ServiceAccountCreatedPage';

vi.mock('@/featureUsage/moesif');

const ORG = 'api-platform-demo';
const SECRET = 'apsa_' + '7'.repeat(64);
const account = aServiceAccount();
const credentials = { clientId: account.clientId, clientSecret: SECRET, serviceAccount: account };

const LocationProbe = () => <output data-testid="location">{useLocation().pathname}</output>;

function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <ServiceAccountCreatedPage />
      <LocationProbe />
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/service-accounts/created`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  resetHttpClient();
  resetTrackedFeatures();
  server.use(collection('/roles', [aRole()]));
});
afterEach(() => clearCredentials());

describe('ServiceAccountCreatedPage', () => {
  it('goes to the list when there is no secret to show', async () => {
    renderPage();

    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(
        `/organizations/${ORG}/service-accounts`,
      ),
    );
    expect(screen.getByTestId('location')).not.toHaveTextContent('created');
  });

  it('shows the credentials, masking the secret until it is revealed', async () => {
    holdCredentials(credentials, ORG);
    const { user } = renderPage();

    expect(
      await screen.findByRole('heading', { name: /Service account created/ }),
    ).toBeInTheDocument();
    expect(screen.getByText('The secret will not be shown again')).toBeInTheDocument();
    expect(screen.getByLabelText('Client ID')).toHaveTextContent(account.clientId);
    expect(screen.getByLabelText('Client secret')).not.toHaveTextContent(SECRET);
    // The curl example never carries the secret itself.
    expect(document.body.textContent).not.toContain(SECRET);

    await user.click(screen.getByRole('button', { name: 'Show Client secret' }));
    expect(screen.getByLabelText('Client secret')).toHaveTextContent(SECRET);
    await user.click(screen.getByRole('button', { name: 'Hide Client secret' }));
    expect(screen.getByLabelText('Client secret')).not.toHaveTextContent(SECRET);
  });

  it('gets a test token with the held secret, then another', async () => {
    holdCredentials(credentials, ORG);
    let issued = 0;
    server.use(
      http.post(apiUrl('/service-accounts/token'), () => {
        issued += 1;
        return HttpResponse.json({
          access_token: `eyJ.token.${issued}`,
          expires_in: 900,
          scope: 'ap:organization:read ap:project:read',
          token_type: 'Bearer',
        });
      }),
    );
    const { user } = renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: 'Get token' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Get token' }));

    expect(await screen.findByText(/The credentials work/)).toBeInTheDocument();
    expect(trackedFeatures()).toEqual([
      ['apip.security.service-accounts', { operation: 'issue-test-token' }],
    ]);
    expect(screen.getByText(/Expires in 1[45]:/)).toBeInTheDocument();
    expect(screen.getByText('scopes in this token')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /scopes in this token/ }));
    expect(await screen.findByText('ap:project:read')).toBeInTheDocument();
    expect(screen.getByText(/This token cannot be revoked/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Get another' }));
    await waitFor(() => expect(issued).toBe(2));
    expect(trackedFeatures()).toEqual([
      ['apip.security.service-accounts', { operation: 'issue-test-token' }],
      ['apip.security.service-accounts', { operation: 'issue-test-token' }],
    ]);
  });

  it('says so when the server rejects the credentials', async () => {
    holdCredentials(credentials, ORG);
    server.use(failure('post', '/service-accounts/token', 401, 'UNAUTHORIZED'));
    const { user } = renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: 'Get token' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Get token' }));

    expect(await screen.findByText(/Check the secret/)).toBeInTheDocument();
  });

  it('forgets the secret and returns to the list on Done', async () => {
    holdCredentials(credentials, ORG);
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Done' }));

    expect(screen.getByTestId('location')).not.toHaveTextContent('created');
    expect(hasHeldCredentials()).toBe(false);
  });

  it('copies both values as .env and shows a tick', async () => {
    holdCredentials(credentials, ORG);
    const { user } = renderPage();
    const write = vi.spyOn(navigator.clipboard, 'writeText');

    await user.click(await screen.findByRole('button', { name: 'Copy both as .env' }));

    expect(write).toHaveBeenCalledWith(`CLIENT_ID=${account.clientId}\nCLIENT_SECRET=${SECRET}`);
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
  });

  it('copies the curl example', async () => {
    holdCredentials(credentials, ORG);
    const { user } = renderPage();
    const write = vi.spyOn(navigator.clipboard, 'writeText');

    await user.click(await screen.findByRole('button', { name: 'Copy Get an access token' }));

    expect(write).toHaveBeenCalledWith(expect.stringContaining('service-accounts/token'));
    expect(write.mock.calls[0]![0]).not.toContain(SECRET);
  });

  it('will not ask for a token while the roles failed to load', async () => {
    holdCredentials(credentials, ORG);
    server.use(failure('get', '/roles', 500, 'INTERNAL_ERROR'));
    renderPage();

    expect(await screen.findByText(/Could not load the roles/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Get token' })).toBeDisabled();
  });

  it('blames the scopes, not the secret, when the server refuses them', async () => {
    holdCredentials(credentials, ORG);
    server.use(failure('post', '/service-accounts/token', 400, 'SERVICE_ACCOUNT_INVALID_SCOPE'));
    const { user } = renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: 'Get token' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Get token' }));

    expect(await screen.findByText(/do not grant the requested scopes/)).toBeInTheDocument();
  });

  it('drops the held secret when the page is left without pressing Done', async () => {
    holdCredentials(credentials, ORG);
    const { unmount } = renderPage();
    await screen.findByRole('heading', { name: /Service account created/ });

    unmount();

    expect(hasHeldCredentials()).toBe(false);
  });

  it('retries loading the roles in place, keeping the secret on screen', async () => {
    holdCredentials(credentials, ORG);
    let calls = 0;
    server.use(
      http.get(apiUrl('/roles'), () =>
        ++calls === 1
          ? HttpResponse.json({ message: 'boom' }, { status: 500 })
          : HttpResponse.json({
              list: [aRole()],
              pagination: { limit: 20, offset: 0, total: 1 },
            }),
      ),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Retry' }));

    await waitFor(() => expect(screen.getByRole('button', { name: 'Get token' })).toBeEnabled());
    expect(screen.getByLabelText('Client ID')).toHaveTextContent(account.clientId);
  });

  it('refuses a secret issued for another organization and drops it', async () => {
    holdCredentials(credentials, 'some-other-org');
    renderPage();

    await waitFor(() => expect(screen.getByTestId('location')).not.toHaveTextContent('created'));
    expect(hasHeldCredentials()).toBe(false);
  });
});
