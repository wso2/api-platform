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
import { makeAuthState } from '@/test/mockAuthState';
import {
  aCustomPolicy,
  accepts,
  collection,
  failure,
  recorder,
  resource,
  type CustomPolicyFixture,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { GatewayPoliciesPanel, type GatewayPoliciesPanelProps } from './GatewayPoliciesPanel';

const ORG = 'api-platform-demo';
const GATEWAY_ID = 'self-hosted-gw';

const customPolicy = (version: string, name = 'hmac-signer') => ({
  description: 'Signs requests with an HMAC.',
  isCustomPolicy: true,
  name,
  version,
});

const hubPolicy = { isCustomPolicy: false, name: 'cors', version: 'v1.2.0' };

const organizationPolicy = (version: string, name = 'hmac-signer'): CustomPolicyFixture =>
  aCustomPolicy({ name, uuid: `${name}-uuid`, version });

let organizationReads: Recorder;
let syncs: Recorder;

beforeEach(() => {
  organizationReads = recorder();
  syncs = recorder();
  resetHttpClient();
});

function renderPanel(
  props: Partial<GatewayPoliciesPanelProps> = {},
  options: Parameters<typeof renderWithProviders>[1] = {},
) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GatewayPoliciesPanel gatewayId={GATEWAY_ID} {...props} />
    </ApiScopeProvider>,
    options,
  );
}

const rowOf = async (policyName: string) =>
  (await screen.findByText(policyName)).closest('tr') as HTMLElement;

describe('GatewayPoliciesPanel — custom policy sync', () => {
  it('syncs an unsynced custom policy with the version the gateway reported', async () => {
    const organization: CustomPolicyFixture[] = [];
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [customPolicy('v1.0.0'), hubPolicy] }),
      collection('/gateway-custom-policies', organization, { record: organizationReads }),
      accepts('post', '/gateway-custom-policies/sync', organizationPolicy('v1.0.0'), {
        record: syncs,
      }),
    );

    const { user } = renderPanel();
    const row = await rowOf('hmac-signer');

    expect(await rowOf('cors')).toHaveTextContent('Built-in');

    organization.push(organizationPolicy('v1.0.0'));
    await user.click(await within(row).findByRole('button', { name: 'Sync' }));

    await waitFor(() => expect(syncs.count()).toBe(1));
    const params = syncs.last()!.url.searchParams;
    expect(params.get('gatewayId')).toBe(GATEWAY_ID);
    expect(params.get('policyName')).toBe('hmac-signer');
    expect(params.get('policyVersion')).toBe('v1.0.0');

    expect(
      await screen.findByText('Synced hmac-signer v1.0.0 to the organization.'),
    ).toBeInTheDocument();
    expect(await within(row).findByText('Synced')).toBeInTheDocument();
    expect(within(row).queryByRole('button', { name: 'Sync' })).not.toBeInTheDocument();
  });

  it('offers an update when the gateway runs a higher minor than the organization holds', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [customPolicy('v1.4.0')] }),
      collection('/gateway-custom-policies', [organizationPolicy('v1.2.0')]),
    );

    renderPanel();
    const row = await rowOf('hmac-signer');

    expect(await within(row).findByRole('button', { name: 'Sync' })).toBeEnabled();
    expect(row).toHaveTextContent('Replaces v1.2.0');
  });

  it('offers no sync the server would refuse, and names the version the organization holds', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', {
        policies: [customPolicy('v1.2.0'), customPolicy('v2.0.1', 'jwt-signer')],
      }),
      collection('/gateway-custom-policies', [
        organizationPolicy('v1.4.0'),
        organizationPolicy('v2.0.0', 'jwt-signer'),
      ]),
    );

    renderPanel();

    const older = await rowOf('hmac-signer');
    expect(await within(older).findByText('Synced')).toBeInTheDocument();
    expect(older).toHaveTextContent('Organization version: v1.4.0');

    const patch = await rowOf('jwt-signer');
    expect(await within(patch).findByText('Synced')).toBeInTheDocument();
    expect(patch).toHaveTextContent('Organization version: v2.0.0');

    expect(screen.queryByRole('button', { name: 'Sync' })).not.toBeInTheDocument();
  });

  it('reads every page of the organization’s policies before judging a policy unsynced', async () => {
    const many = Array.from({ length: 120 }, (_unused, index) =>
      organizationPolicy('v1.0.0', `policy-${String(index).padStart(3, '0')}`),
    );
    server.use(
      resource('/gateways/:gatewayId/manifest', {
        policies: [customPolicy('v1.0.0', 'policy-119')],
      }),
      collection('/gateway-custom-policies', many, { record: organizationReads }),
    );

    renderPanel();
    const row = await rowOf('policy-119');

    // policy-119 is on the second page of 100.
    expect(await within(row).findByText('Synced')).toBeInTheDocument();
    expect(organizationReads.count()).toBe(2);
  });

  it('leaves the sync status column out, and reads nothing, for a gateway with no custom policy', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [hubPolicy] }),
      collection('/gateway-custom-policies', [], { record: organizationReads }),
    );

    renderPanel();
    await screen.findByText('cors');

    expect(screen.queryByRole('columnheader', { name: 'Sync status' })).not.toBeInTheDocument();
    // Give a stray request time to land before asserting there was none.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(organizationReads.count()).toBe(0);
  });

  it('says the sync status is unknown when the organization’s policies cannot be read', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [customPolicy('v1.0.0')] }),
      failure('get', '/gateway-custom-policies', 500, 'INTERNAL_ERROR'),
    );

    renderPanel();
    const row = await rowOf('hmac-signer');

    expect(await within(row).findByText('Unknown')).toBeInTheDocument();
    expect(
      await screen.findByText(
        'The organization’s custom policies could not be loaded, so whether each one is synced is not shown.',
      ),
    ).toBeInTheDocument();
    expect(within(row).queryByRole('button', { name: 'Sync' })).not.toBeInTheDocument();
  });

  it('disables Sync for a user who may read custom policies but not sync them', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [customPolicy('v1.0.0')] }),
      collection('/gateway-custom-policies', []),
    );

    renderPanel(
      {},
      {
        authState: makeAuthState({
          user: {
            email: 'viewer@example.com',
            name: 'Viewer',
            scopes: ['ap:gateway:manifest:read', 'ap:gateway_custom_policy:read'],
          },
        }),
        permissionMode: 'enforce',
      },
    );
    const row = await rowOf('hmac-signer');

    expect(await within(row).findByRole('button', { name: 'Sync' })).toBeDisabled();
  });

  it('drops its own card and heading when embedded in a host that frames it', async () => {
    server.use(resource('/gateways/:gatewayId/manifest', { policies: [hubPolicy] }));

    renderPanel({ variant: 'embedded' });
    await screen.findByText('cors');

    expect(screen.queryByRole('heading', { name: 'Policies' })).not.toBeInTheDocument();
  });
});

describe('GatewayPoliciesPanel — finding policies', () => {
  const jwtPolicy = {
    description: 'Validates JWT access tokens.',
    isCustomPolicy: false,
    name: 'jwt-auth',
    version: 'v1.3.0',
  };

  const visibleNames = () =>
    screen
      .getAllByRole('row')
      .slice(1)
      .map((row) => within(row).getAllByRole('cell')[0].textContent);

  it('searches policy names and descriptions', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', {
        policies: [customPolicy('v1.0.0'), hubPolicy, jwtPolicy],
      }),
      collection('/gateway-custom-policies', []),
    );

    const { user } = renderPanel();
    await screen.findByText('jwt-auth');
    const search = screen.getByPlaceholderText('Search policies');

    await user.type(search, 'HMAC');
    await waitFor(() => expect(visibleNames()).toEqual(['hmac-signer']));

    await user.clear(search);
    await user.type(search, 'access tokens');
    await waitFor(() => expect(visibleNames()).toEqual(['jwt-auth']));
  });

  it('filters by policy type', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', {
        policies: [customPolicy('v1.0.0'), hubPolicy, jwtPolicy],
      }),
      collection('/gateway-custom-policies', []),
    );

    const { user } = renderPanel();
    await screen.findByText('jwt-auth');
    const filter = screen.getByRole('group', { name: 'Filter by policy type' });

    await user.click(within(filter).getByRole('button', { name: 'Custom' }));
    expect(visibleNames()).toEqual(['hmac-signer']);

    await user.click(within(filter).getByRole('button', { name: 'Policy Hub' }));
    expect(visibleNames()).toEqual(['cors', 'jwt-auth']);
  });

  it('offers to clear a search and filter that match nothing', async () => {
    server.use(
      resource('/gateways/:gatewayId/manifest', { policies: [customPolicy('v1.0.0'), hubPolicy] }),
      collection('/gateway-custom-policies', []),
    );

    const { user } = renderPanel();
    await screen.findByText('cors');

    await user.click(screen.getByRole('button', { name: 'Custom' }));
    await user.type(screen.getByPlaceholderText('Search policies'), 'cors');
    expect(await screen.findByText('No matching policies')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Clear search and filter' }));
    expect(visibleNames()).toEqual(['cors', 'hmac-signer']);
  });

  it('pages through more policies than fit on one page, without a type filter when none is custom', async () => {
    const policies = Array.from({ length: 12 }, (_, index) => ({
      isCustomPolicy: false,
      name: `policy-${String(index + 1).padStart(2, '0')}`,
      version: 'v1.0.0',
    }));
    server.use(resource('/gateways/:gatewayId/manifest', { policies }));

    const { user } = renderPanel();
    await screen.findByText('policy-01');

    expect(visibleNames()).toHaveLength(5);
    expect(screen.getByText('1–5 of 12')).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Filter by policy type' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Next page' }));
    expect(visibleNames()).toEqual([
      'policy-06',
      'policy-07',
      'policy-08',
      'policy-09',
      'policy-10',
    ]);
  });
});
