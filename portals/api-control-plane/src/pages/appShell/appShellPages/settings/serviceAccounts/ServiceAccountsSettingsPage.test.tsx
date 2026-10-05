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

import type { QueryClient } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aServiceAccount,
  aServiceAccountRole,
  accepts,
  apiUrl,
  collection,
  failure,
  listEnvelope,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '@/test/msw';
import { makeAuthState } from '@/test/mockAuthState';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { ServiceAccountsSettingsPage } from './ServiceAccountsSettingsPage';

const ORG = 'api-platform-demo';
/** The create tests type through a whole form, which outruns 5s under a full-suite load. */
const FORM_TIMEOUT = 15_000;
const SECRET = 'apsa_' + '7'.repeat(64);

const ciDeployer = aServiceAccount();
const nightly = aServiceAccount({
  clientId: 'sa_acme_nightly_aa11bb',
  displayName: 'Nightly report',
  id: 'nightly',
  lastUsedAt: '2026-10-01T10:00:00Z',
  lastUsedIp: '203.0.113.7',
  roles: ['ap_sa_reader', 'ap_sa_deployer'],
  status: 'disabled',
});
const roles = [
  aServiceAccountRole(),
  aServiceAccountRole({ name: 'ap_sa_deployer', scopes: ['ap:rest_api:deployment:manage'] }),
];
const credentialsFor = (account = ciDeployer) => ({
  clientId: account.clientId,
  clientSecret: SECRET,
  serviceAccount: account,
});

let requests: Recorder;

/** True once neither React Query cache holds the secret any more. */
const secretGone = (queryClient: QueryClient) =>
  !JSON.stringify([
    queryClient.getQueryCache().getAll().map((query) => query.state.data),
    queryClient.getMutationCache().getAll().map((mutation) => [mutation.state.data, mutation.state.variables]),
  ]).includes(SECRET);

function renderPage(options: Parameters<typeof renderWithProviders>[1] = {}) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <ServiceAccountsSettingsPage />
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/settings/service-accounts`, scope: makeConsoleScope(), ...options },
  );
}

const useAccounts = (accounts = [ciDeployer, nightly]) =>
  server.use(
    collection('/service-accounts', accounts, { record: requests }),
    collection('/service-account-roles', roles),
  );

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('ServiceAccountsSettingsPage', () => {
  it('lists accounts with owner, roles, status and last token issued', async () => {
    useAccounts();
    renderPage();

    expect(await screen.findByText('CI deployer')).toBeInTheDocument();
    const row = screen.getByText('Nightly report').closest('tr')!;
    expect(within(row).getByText('ap_sa_deployer')).toBeInTheDocument();
    expect(within(row).getByText('Disabled')).toBeInTheDocument();
    expect(screen.getByText('Never')).toBeInTheDocument();
    expect(screen.getAllByText('platform-team@example.com').length).toBeGreaterThan(0);
  });

  it('shows two roles, then a "+N" chip naming the rest', async () => {
    useAccounts([
      aServiceAccount({ roles: ['ap_sa_a', 'ap_sa_b', 'ap_sa_c', 'ap_sa_d'] }),
      aServiceAccount({ displayName: 'Two roles', id: 'two', roles: ['ap_sa_a', 'ap_sa_b'] }),
    ]);
    const { user } = renderPage();

    const row = (await screen.findByText('CI deployer')).closest('tr')!;
    expect(within(row).getByText('ap_sa_b')).toBeInTheDocument();
    expect(within(row).queryByText('ap_sa_c')).not.toBeInTheDocument();
    const more = within(row).getByLabelText('2 more roles: ap_sa_c, ap_sa_d');
    expect(more).toHaveTextContent('+2');
    await user.hover(more);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('ap_sa_c, ap_sa_d');

    const two = screen.getByText('Two roles').closest('tr')!;
    expect(within(two).queryByText(/^\+/)).not.toBeInTheDocument();
  });

  it('shows only the name and a copy button, with the client ID in its tooltip', async () => {
    useAccounts();
    const { user } = renderPage();

    const row = (await screen.findByText('Nightly report')).closest('tr')!;
    expect(within(row).queryByText('nightly')).not.toBeInTheDocument();
    expect(within(row).queryByText('sa_acme_nightly_aa11bb')).not.toBeInTheDocument();

    await user.hover(within(row).getByRole('button', { name: 'Copy client ID of Nightly report' }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Copy client ID: sa_acme_nightly_aa11bb');
  });

  it('opens the edit view from the name, showing the ID, client ID and secret', async () => {
    useAccounts();
    server.use(resource('/service-accounts/ci-deployer', aServiceAccount()));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'CI deployer' }));
    const form = await screen.findByRole('dialog');
    expect(within(form).getByText('Edit service account')).toBeInTheDocument();
    const facts = within(form).getAllByRole('definition')[0].closest('dl')!;
    expect(within(facts).getByText('ID').nextElementSibling).toHaveTextContent('ci-deployer');
    expect(within(facts).getByText('Client ID').nextElementSibling).toHaveTextContent('sa_acme_ci-deployer_3f9a1c');
    expect(within(facts).getByText('Secret').nextElementSibling).toHaveTextContent('***9f2c1');
    expect(within(form).getByRole('button', { name: 'Copy client ID' })).toBeInTheDocument();
    expect(within(form).getByLabelText(/Display name/)).toHaveValue('CI deployer');
  });

  it('shows "not allowed" to a user without the manage scope, and fetches nothing', async () => {
    server.use(
      collection('/service-accounts', [ciDeployer], { record: requests }),
      collection('/service-account-roles', roles, { record: requests }),
    );
    renderPage({
      authState: makeAuthState({
        user: { email: 'op@example.com', name: 'Operator', scopes: ['ap:service_account:read'] },
      }),
      permissionMode: 'enforce',
    });

    expect(await screen.findByText('Service accounts are managed by administrators')).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(requests.count()).toBe(0);
  });

  it('says the feature is off when the server has no service-account routes', async () => {
    server.use(
      failure('get', '/service-accounts', 404, 'NOT_FOUND'),
      failure('get', '/service-account-roles', 404, 'NOT_FOUND'),
    );
    renderPage();

    expect(await screen.findByText('Service accounts are turned off')).toBeInTheDocument();
  });

  it('creates an account and shows the secret once, in a dialog Escape cannot close', async () => {
    useAccounts([]);
    server.use(accepts('post', '/service-accounts', credentialsFor(), { record: requests }));
    const { user, queryClient } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'New service account' }));
    const form = await screen.findByRole('dialog');
    await user.type(within(form).getByLabelText(/Display name/), 'CI deployer');
    expect(within(form).getByLabelText(/^ID/)).toHaveValue('ci-deployer');
    expect(within(form).getByLabelText(/^ID/)).toBeDisabled();
    await user.type(within(form).getByLabelText(/Owner/), 'platform-team@example.com');
    await user.type(within(form).getByLabelText(/Description/), 'Deploys APIs');
    await user.click(within(form).getByRole('combobox', { name: /Roles/ }));
    await user.click(await screen.findByRole('option', { name: /ap_sa_reader/ }));
    await user.keyboard('{Escape}');
    await user.click(within(form).getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(requests.calls.some((call) => call.method === 'POST')).toBe(true));
    const post = requests.calls.find((call) => call.method === 'POST')!;
    expect(JSON.parse(post.body)).toEqual({
      description: 'Deploys APIs',
      displayName: 'CI deployer',
      id: 'ci-deployer',
      owner: 'platform-team@example.com',
      roles: ['ap_sa_reader'],
    });

    expect(await screen.findByText('Copy the secret now')).toBeInTheDocument();
    expect(screen.getByLabelText('Client secret')).toHaveValue(SECRET);
    // The curl example never carries the secret itself.
    expect(document.body.textContent).not.toContain(SECRET);

    await user.keyboard('{Escape}');
    expect(screen.getByText('Copy the secret now')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Done' }));
    await waitFor(() => expect(screen.queryByText('Copy the secret now')).not.toBeInTheDocument());
    await waitFor(() => expect(secretGone(queryClient)).toBe(true));
  }, FORM_TIMEOUT);

  it('needs only a name and a role, and takes a custom ID after Edit ID', async () => {
    useAccounts([]);
    server.use(accepts('post', '/service-accounts', credentialsFor(), { record: requests }));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'New service account' }));
    const form = await screen.findByRole('dialog');
    const create = within(form).getByRole('button', { name: 'Create' });
    expect(create).toBeDisabled();
    await user.type(within(form).getByLabelText(/Display name/), `Release ${'x'.repeat(50)}`);
    // Still no role.
    expect(create).toBeDisabled();
    // Cut to the server's 40-character limit.
    expect(within(form).getByLabelText(/^ID/)).toHaveValue(`release-${'x'.repeat(32)}`);

    await user.click(within(form).getByRole('button', { name: 'Edit ID' }));
    const id = within(form).getByLabelText(/^ID/);
    expect(id).toBeEnabled();
    expect(id).toHaveFocus();
    await user.clear(id);
    await user.type(id, 'x');
    expect(within(form).getByText('Use 3 to 40 characters.')).toBeInTheDocument();
    await user.clear(id);
    await user.type(id, 'release-bot');
    // Once edited, the ID stops following the name.
    await user.type(within(form).getByLabelText(/Display name/), 'y');
    expect(id).toHaveValue('release-bot');

    await user.click(within(form).getByRole('combobox', { name: /Roles/ }));
    await user.click(await screen.findByRole('option', { name: /ap_sa_reader/ }));
    await user.keyboard('{Escape}');
    expect(create).toBeEnabled();
    await user.click(create);

    await waitFor(() => expect(requests.calls.some((call) => call.method === 'POST')).toBe(true));
    // No owner or description: the server makes the creator the owner.
    expect(JSON.parse(requests.calls.find((call) => call.method === 'POST')!.body)).toEqual({
      displayName: `Release ${'x'.repeat(50)}y`,
      id: 'release-bot',
      roles: ['ap_sa_reader'],
    });
  }, FORM_TIMEOUT);

  it('sends a cleared owner as empty, so the server resets it to the creator', async () => {
    const account = aServiceAccount({ createdBy: 'alice' });
    useAccounts([account]);
    server.use(
      accepts('put', '/service-accounts/ci-deployer', account, { record: requests }),
      resource('/service-accounts/ci-deployer', account),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
    const form = await screen.findByRole('dialog');
    expect(within(form).getByText(/Leave empty for the creator, alice/)).toBeInTheDocument();
    await user.clear(within(form).getByLabelText(/Owner/));
    await user.clear(within(form).getByLabelText(/Description/));
    await user.click(within(form).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(requests.calls.some((call) => call.method === 'PUT')).toBe(true));
    expect(JSON.parse(requests.calls.find((call) => call.method === 'PUT')!.body)).toEqual({
      description: '',
      owner: '',
    });
  }, FORM_TIMEOUT);

  it('flags an ID that is already taken on the ID field', async () => {
    useAccounts([]);
    server.use(failure('post', '/service-accounts', 409, 'SERVICE_ACCOUNT_EXISTS'));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'New service account' }));
    const form = await screen.findByRole('dialog');
    await user.type(within(form).getByLabelText(/Display name/), 'CI deployer');
    await user.type(within(form).getByLabelText(/Owner/), 'team');
    await user.type(within(form).getByLabelText(/Description/), 'deploys');
    await user.click(within(form).getByRole('combobox', { name: /Roles/ }));
    await user.click(await screen.findByRole('option', { name: /ap_sa_reader/ }));
    await user.keyboard('{Escape}');
    await user.click(within(form).getByRole('button', { name: 'Create' }));

    expect(await within(form).findByText('An account with this ID already exists.')).toBeInTheDocument();
  }, FORM_TIMEOUT);

  it('regenerates the secret after a warning, then shows the new one', async () => {
    useAccounts();
    server.use(
      accepts('post', '/service-accounts/ci-deployer/regenerate-secret', credentialsFor(), {
        record: requests,
        status: 200,
      }),
    );
    const { user, queryClient } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Regenerate secret' }));
    const confirm = screen.getByRole('dialog');
    expect(within(confirm).getByText(/stops working immediately, with no overlap/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Regenerate secret' }));

    expect(await screen.findByText('Copy the secret now')).toBeInTheDocument();
    expect(screen.getByLabelText('Client secret')).toHaveValue(SECRET);

    await user.click(screen.getByRole('button', { name: 'Done' }));
    // The page stays mounted, so this proves the mutation's copy is dropped
    // on purpose, not by an unmount.
    await waitFor(() => expect(secretGone(queryClient)).toBe(true));
  });

  it('disables an account after confirmation, sending only the status', async () => {
    useAccounts();
    server.use(accepts('put', '/service-accounts/ci-deployer', aServiceAccount({ status: 'disabled' }), {
      record: requests,
    }));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Disable' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Disable' }));

    await waitFor(() => expect(requests.calls.some((call) => call.method === 'PUT')).toBe(true));
    expect(JSON.parse(requests.calls.find((call) => call.method === 'PUT')!.body)).toEqual({
      status: 'disabled',
    });
  });

  it('deletes only once the ID is typed', async () => {
    useAccounts();
    server.use(noContent('delete', '/service-accounts/ci-deployer', { record: requests }));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Delete' }));
    const confirm = screen.getByRole('dialog');
    expect(within(confirm).getByRole('button', { name: 'Delete' })).toBeDisabled();
    await user.type(within(confirm).getByRole('textbox'), 'ci-deployer');
    await user.click(within(confirm).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(requests.calls.some((call) => call.method === 'DELETE')).toBe(true));
  });

  it('gets a token, sending the scopes the roles grant', async () => {
    useAccounts();
    server.use(
      resource(
        '/service-accounts/token',
        { access_token: 'eyJ.test.token', expires_in: 900, scope: 'ap:rest_api:read ap:gateway:read', token_type: 'Bearer' },
        { method: 'post', record: requests },
      ),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Get token' }));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Client secret/), SECRET);
    await user.click(within(dialog).getByRole('button', { name: 'Get token' }));

    expect(await within(dialog).findByLabelText('Access token')).toHaveValue('eyJ.test.token');
    expect(within(dialog).getByText(/Expires in 1[45]:/)).toBeInTheDocument();
    const form = new URLSearchParams(requests.calls.find((call) => call.method === 'POST')!.body);
    expect(form.get('client_id')).toBe(ciDeployer.clientId);
    expect(form.get('scope')).toBe('ap:rest_api:read ap:gateway:read');
  });

  it('says to check the secret when the server rejects the credentials', async () => {
    useAccounts();
    server.use(failure('post', '/service-accounts/token', 401, 'UNAUTHORIZED'));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Get token' }));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Client secret/), 'wrong');
    await user.click(within(dialog).getByRole('button', { name: 'Get token' }));

    expect(
      await within(dialog).findByText('Could not get a token. Check the secret, and that the account is enabled.'),
    ).toBeInTheDocument();
  });

  it('warns when an edit removes a role, and sends only what changed', async () => {
    useAccounts();
    server.use(
      accepts('put', '/service-accounts/nightly', nightly, { record: requests }),
      resource('/service-accounts/nightly', nightly),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for Nightly report' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
    const form = await screen.findByRole('dialog');
    expect(within(form).queryByText(/Removing a role stops every token/)).not.toBeInTheDocument();

    await user.click(within(form).getByRole('combobox', { name: /Roles/ }));
    await user.click(await screen.findByRole('option', { name: /ap_sa_deployer/ }));
    await user.keyboard('{Escape}');
    expect(within(form).getByText(/Removing a role stops every token/)).toBeInTheDocument();

    await user.click(within(form).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(requests.calls.some((call) => call.method === 'PUT')).toBe(true));
    expect(JSON.parse(requests.calls.find((call) => call.method === 'PUT')!.body)).toEqual({
      roles: ['ap_sa_reader'],
    });
  });

  it('offers a reload when someone else changed the account first', async () => {
    useAccounts();
    server.use(
      failure('put', '/service-accounts/ci-deployer', 409, 'CONFLICT'),
      resource('/service-accounts/ci-deployer', aServiceAccount({ owner: 'someone-else' })),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
    const form = await screen.findByRole('dialog');
    await user.clear(within(form).getByLabelText(/Owner/));
    await user.type(within(form).getByLabelText(/Owner/), 'new-owner');
    await user.click(within(form).getByRole('button', { name: 'Save' }));

    expect(await within(form).findByText(/Someone else changed this account/)).toBeInTheDocument();
    await user.click(within(form).getByRole('button', { name: 'Reload' }));
    await waitFor(() => expect(within(form).queryByText(/Someone else changed/)).not.toBeInTheDocument());
    // What the admin typed is kept.
    expect(within(form).getByLabelText(/Owner/)).toHaveValue('new-owner');
  });

  it('says the scopes are the problem when scope mode refuses them', async () => {
    useAccounts();
    server.use(failure('post', '/service-accounts/token', 400, 'SERVICE_ACCOUNT_INVALID_SCOPE'));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Get token' }));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Client secret/), SECRET);
    await user.click(within(dialog).getByRole('button', { name: 'Get token' }));

    expect(await within(dialog).findByText(/roles do not grant the requested scopes/)).toBeInTheDocument();
  });

  it('does not blame the secret for a server error', async () => {
    useAccounts();
    server.use(failure('post', '/service-accounts/token', 500, 'INTERNAL_ERROR'));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Get token' }));
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(/Client secret/), SECRET);
    await user.click(within(dialog).getByRole('button', { name: 'Get token' }));

    expect(await within(dialog).findByText('Could not get a token. Try again in a moment.')).toBeInTheDocument();
    expect(within(dialog).queryByText(/Check the secret/)).not.toBeInTheDocument();
  });

  it('sends the search to the server and starts again from the first page', async () => {
    const many = Array.from({ length: 25 }, (_, index) =>
      aServiceAccount({ clientId: `sa_acme_bot-${index}`, displayName: `Bot ${index}`, id: `bot-${index}` }),
    );
    server.use(
      collection('/service-accounts', many, {
        matches: (account, term) => account.displayName.toLowerCase().includes(term),
        record: requests,
      }),
      collection('/service-account-roles', roles),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Next page' }));
    expect(await screen.findByText('Bot 20')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search by name, ID or owner'), 'bot 7');

    expect(await screen.findByText('Bot 7')).toBeInTheDocument();
    await waitFor(() => expect(requests.last()?.params.get('query')).toBe('bot 7'));
    expect(requests.last()?.params.get('offset')).toBe('0');
    expect(screen.queryByText('Bot 20')).not.toBeInTheDocument();
  });

  it('steps back a page when the last row on the last page is deleted', async () => {
    const many = Array.from({ length: 21 }, (_, index) =>
      aServiceAccount({ clientId: `sa_acme_bot-${index}`, displayName: `Bot ${index}`, id: `bot-${index}` }),
    );
    let current = many;
    server.use(
      collection('/service-account-roles', roles),
      http.get(apiUrl('/service-accounts'), ({ request }) => {
        const params = new URL(request.url).searchParams;
        const offset = Number(params.get('offset') ?? 0);
        const limit = Number(params.get('limit') ?? 20);
        return HttpResponse.json(
          listEnvelope(current.slice(offset, offset + limit), { limit, offset, total: current.length }) as never,
        );
      }),
      http.delete(apiUrl('/service-accounts/bot-20'), () => {
        current = many.slice(0, 20);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Next page' }));
    await user.click(await screen.findByRole('button', { name: 'Actions for Bot 20' }));
    await user.click(screen.getByRole('menuitem', { name: 'Delete' }));
    const confirm = screen.getByRole('dialog');
    await user.type(within(confirm).getByRole('textbox'), 'bot-20');
    await user.click(within(confirm).getByRole('button', { name: 'Delete' }));

    expect(await screen.findByText('Bot 0')).toBeInTheDocument();
    expect(screen.queryByText('No matching service accounts')).not.toBeInTheDocument();
  });

  it('builds the edit form from a fresh read, not the cached list row', async () => {
    useAccounts();
    server.use(resource('/service-accounts/ci-deployer', aServiceAccount({ owner: 'changed-by-someone-else' })));
    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Actions for CI deployer' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit' }));
    const form = await screen.findByRole('dialog');

    expect(await within(form).findByDisplayValue('changed-by-someone-else')).toBeInTheDocument();
  });
});
