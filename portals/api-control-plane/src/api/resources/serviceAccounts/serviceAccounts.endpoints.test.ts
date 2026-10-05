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

import {
  aServiceAccount,
  aServiceAccountRole,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { CSRF_HEADER } from '../../../contexts/auth/authConstants';
import { resetHttpClient } from '../../core/http';
import { onSessionExpired, resetSessionExpiryNotice } from '../../core/sessionEvents';
import {
  createServiceAccount,
  deleteServiceAccount,
  issueServiceAccountToken,
  listServiceAccountRoles,
  listServiceAccounts,
  regenerateServiceAccountSecret,
  updateServiceAccount,
} from './serviceAccounts.endpoints';

/**
 * Contract tests for `/service-accounts`. The token exchange is the one
 * form-encoded operation in the console, and its 401 is about the caller's own
 * credentials, so those two get the most attention.
 */

let requests: Recorder;

const credentials = {
  clientId: 'sa_acme_ci-deployer_3f9a1c',
  clientSecret: 'apsa_' + 'a'.repeat(64),
  serviceAccount: aServiceAccount(),
};

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
  resetSessionExpiryNotice();
});

describe('service account reads', () => {
  it('lists accounts with paging', async () => {
    server.use(collection('/service-accounts', [aServiceAccount()], { record: requests }));

    const response = await listServiceAccounts({ query: { limit: 20, offset: 0 } });

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/service-accounts');
    expect(requests.last()?.params.get('limit')).toBe('20');
    expect(response.list[0]).not.toHaveProperty('clientSecret');
  });

  it('lists assignable roles from the sibling path, not under /service-accounts', async () => {
    server.use(collection('/service-account-roles', [aServiceAccountRole()], { record: requests }));

    const response = await listServiceAccountRoles();

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/service-account-roles');
    expect(response.list[0]?.name).toBe('ap_sa_reader');
  });
});

describe('service account writes', () => {
  it('creates with a JSON body and returns the credentials', async () => {
    server.use(accepts('post', '/service-accounts', credentials, { record: requests }));

    const result = await createServiceAccount({
      description: 'deploys',
      displayName: 'CI deployer',
      id: 'ci-deployer',
      owner: 'team',
      roles: ['ap_sa_reader'],
    });

    expect(JSON.parse(requests.last()!.body)).toMatchObject({ id: 'ci-deployer', roles: ['ap_sa_reader'] });
    expect(result.clientSecret).toBe(credentials.clientSecret);
  });

  it('sends only the fields given on update', async () => {
    server.use(accepts('put', '/service-accounts/ci-deployer', aServiceAccount(), { record: requests }));

    await updateServiceAccount('ci-deployer', { status: 'disabled' });

    expect(JSON.parse(requests.last()!.body)).toEqual({ status: 'disabled' });
  });

  it('regenerates through the action path', async () => {
    server.use(
      accepts('post', '/service-accounts/ci-deployer/regenerate-secret', credentials, {
        record: requests,
        status: 200,
      }),
    );

    await regenerateServiceAccountSecret('ci-deployer');

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/service-accounts/ci-deployer/regenerate-secret');
  });

  it('deletes the account', async () => {
    server.use(noContent('delete', '/service-accounts/ci-deployer', { record: requests }));

    await deleteServiceAccount('ci-deployer');

    expect(requests.last()?.method).toBe('DELETE');
  });
});

describe('issueServiceAccountToken', () => {
  const body = {
    client_id: 'sa_acme_ci-deployer_3f9a1c',
    client_secret: 'apsa_secret',
    grant_type: 'client_credentials' as const,
    scope: 'ap:rest_api:read ap:gateway:read',
  };

  it('sends the grant form-encoded, with the CSRF header the BFF requires', async () => {
    server.use(
      resource(
        '/service-accounts/token',
        { access_token: 'tok', expires_in: 900, token_type: 'Bearer' },
        { method: 'post', record: requests },
      ),
    );

    await issueServiceAccountToken(body);

    const sent = requests.last()!;
    expect(sent.headers.get('Content-Type')).toContain('application/x-www-form-urlencoded');
    expect(sent.headers.get(CSRF_HEADER)).toBeTruthy();
    const form = new URLSearchParams(sent.body);
    expect(form.get('grant_type')).toBe('client_credentials');
    expect(form.get('client_secret')).toBe('apsa_secret');
    expect(form.get('scope')).toBe('ap:rest_api:read ap:gateway:read');
  });

  it('omits scope when none is given', async () => {
    server.use(
      resource(
        '/service-accounts/token',
        { access_token: 'tok', expires_in: 900, token_type: 'Bearer' },
        { method: 'post', record: requests },
      ),
    );

    await issueServiceAccountToken({ ...body, scope: undefined });

    expect(new URLSearchParams(requests.last()!.body).has('scope')).toBe(false);
  });

  it('does not treat a wrong secret as an expired session', async () => {
    server.use(failure('post', '/service-accounts/token', 401, 'UNAUTHORIZED'));
    const notified = vi.fn();
    const unsubscribe = onSessionExpired(notified);

    await expect(issueServiceAccountToken(body)).rejects.toMatchObject({ status: 401 });

    expect(notified).not.toHaveBeenCalled();
    unsubscribe();
  });
});
