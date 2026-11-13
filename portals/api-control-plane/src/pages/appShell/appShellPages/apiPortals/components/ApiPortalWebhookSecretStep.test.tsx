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

import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { accepts, failure, recorder, resource, type Recorder } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiPortalWebhookSecretStep } from './ApiPortalWebhookSecretStep';

const ORG = 'api-platform-demo';
const PORTAL_ID = 'partner-portal';
const SECRET_PATH = `/webhook-secrets/${PORTAL_ID}`;
const ISSUED = {
  id: PORTAL_ID,
  name: 'Partner Portal',
  type: 'PORTAL',
  versions: [],
  secret: 'whsec_0f3c9a1b2c3d4e5f60718293a4b5c6d7',
};

let writes: Recorder;

function renderStep() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <ApiPortalWebhookSecretStep apiPortalId={PORTAL_ID} portalName="Partner Portal" />
    </ApiScopeProvider>,
    { route: `/organizations/${ORG}/api-portals/${PORTAL_ID}`, scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  writes = recorder();
  resetHttpClient();
});

describe('ApiPortalWebhookSecretStep', () => {
  it('generates a secret for a portal that has none and shows it once', async () => {
    server.use(
      failure('get', SECRET_PATH, 404, 'NOT_FOUND'),
      accepts('post', '/webhook-secrets', ISSUED, { record: writes }),
    );

    const { user } = renderStep();

    await user.click(await screen.findByRole('button', { name: 'Generate webhook secret' }));

    await waitFor(() => expect(writes.count()).toBe(1));
    expect(JSON.parse(writes.last()!.body)).toEqual({
      id: PORTAL_ID,
      name: 'Partner Portal',
      type: 'PORTAL',
    });
    expect(await screen.findByText(/Copy this secret now/)).toBeInTheDocument();
    // The secret appears on its own and inside the subscriber body.
    expect(screen.getAllByText(new RegExp(ISSUED.secret))).toHaveLength(2);
  });

  it('rotates the secret of a portal that already has one', async () => {
    server.use(
      resource(SECRET_PATH, { id: PORTAL_ID, name: 'Partner Portal', type: 'PORTAL', versions: [] }),
      accepts('post', `${SECRET_PATH}/rotate`, ISSUED, { record: writes }),
    );

    const { user } = renderStep();

    expect(await screen.findByText(/already has a webhook secret/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Rotate webhook secret' }));

    await waitFor(() => expect(writes.count()).toBe(1));
    expect(await screen.findByText(/Copy this secret now/)).toBeInTheDocument();
  });

  it('shows the subscriber setup with placeholders before a secret is issued', async () => {
    server.use(failure('get', SECRET_PATH, 404, 'NOT_FOUND'));

    renderStep();

    expect(await screen.findByRole('button', { name: 'Generate webhook secret' })).toBeInTheDocument();
    expect(
      screen.getAllByText(/<platform-api-url>\/api\/internal\/v0\.9\/webhook\/events/).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText(/<webhook-secret>/)).toBeInTheDocument();
  });

  it('says so when the secret cannot be checked, and offers no action', async () => {
    server.use(failure('get', SECRET_PATH, 500, 'INTERNAL_SERVER_ERROR'));

    renderStep();

    expect(await screen.findByText(/Could not check whether this portal has a webhook secret/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /webhook secret/ })).not.toBeInTheDocument();
  });
});
