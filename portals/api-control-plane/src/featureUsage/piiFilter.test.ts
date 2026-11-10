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

import { describe, expect, it } from 'vitest';

import { routes, SELECT_SCOPE_SEGMENT } from '@/routes/paths';
import { filterMoesifRequest, maskPath, maskUri } from './piiFilter';

const ORG = 'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f';
const USER = '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b';
const ANON = '5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f';
const TX = '5f0c8a9e-3b1d-4c6e-9a2f-1d7e4b8c0a11';
const ORIGIN = 'https://console.example.com';
const HOST = 'https://api.moesif.net';

describe('maskPath', () => {
  it.each([
    ['the API creation wizard', routes.newApi('acme-corp', 'payments'), routes.newApi()],
    ['an API overview', routes.api('acme-corp', 'payments', 'orders-api'), routes.api()],
    ['the organization home', routes.organizationHome('acme-corp'), routes.organizationHome()],
    ['a GraphQL API', routes.graphqlApi('acme-corp', 'payments', 'countries'), routes.graphqlApi()],
  ])('reduces %s to its route template', (_name, path, template) => {
    expect(maskPath(path)).toBe(template);
    expect(maskPath(path)).not.toMatch(/acme-corp|payments|orders-api|countries/);
  });

  it('keeps a literal segment rather than reading it as a handle', () => {
    expect(maskPath(routes.newProject('acme-corp'))).toBe(routes.newProject());
  });

  it('masks every unknown segment of a path no template matches', () => {
    expect(maskPath(`/organizations/acme-corp/${SELECT_SCOPE_SEGMENT}/payments/unknown-page`)).toBe(
      `/organizations/:id/${SELECT_SCOPE_SEGMENT}/:id/:id`,
    );
  });

  it('keeps the root as it is', () => {
    expect(maskPath('/')).toBe('/');
  });
});

describe('maskUri', () => {
  it('keeps the origin, masks the path, and drops the query string and fragment', () => {
    expect(
      maskUri(`${ORIGIN}${routes.newApi('acme-corp', 'payments')}?search=Payments+API#step-2`),
    ).toBe(`${ORIGIN}${routes.newApi()}`);
  });

  it('keeps the deployment’s base path and masks what follows it', () => {
    expect(maskUri(`${ORIGIN}/console${routes.organizationHome('acme-corp')}`, '/console')).toBe(
      `${ORIGIN}/console${routes.organizationHome()}`,
    );
  });

  it.each([undefined, 42, 'not a url'])('falls back to "/" for %s', (value) => {
    expect(maskUri(value)).toBe('/');
  });
});

describe('filterMoesifRequest', () => {
  /** An action as moesif-browser-js builds it, plus fields that must not survive. */
  const sdkAction = (overrides: Record<string, unknown> = {}) => ({
    action_name: 'apip.manage-apis.rest-api-create-click',
    anonymous_id: ANON,
    company_id: ORG,
    email: 'alice@example.com',
    metadata: {
      action: 'REST API Creation Click',
      apiName: 'Payments API',
      category: 'manage-apis',
      creation_method: 'scratch',
      event_category: 'engagement',
      initiating_product: 'apip',
      platform: 'web',
      tier: 'core',
    },
    request: {
      ip_address: '203.0.113.7',
      uri: `${ORIGIN}${routes.newApi('acme-corp', 'payments')}?q=secret`,
      user_agent_string: 'Mozilla/5.0',
      verb: 'GET',
    },
    transaction_id: TX,
    user_id: USER,
    ...overrides,
  });

  it('rebuilds an action from the allowlist only', () => {
    expect(filterMoesifRequest(`${HOST}/v1/actions`, sdkAction())).toEqual({
      action_name: 'apip.manage-apis.rest-api-create-click',
      anonymous_id: ANON,
      company_id: ORG,
      metadata: {
        action: 'REST API Creation Click',
        category: 'manage-apis',
        creation_method: 'scratch',
        event_category: 'engagement',
        initiating_product: 'apip',
        platform: 'web',
        tier: 'core',
      },
      request: {
        uri: `${ORIGIN}${routes.newApi()}`,
        user_agent_string: 'Mozilla/5.0',
        verb: 'GET',
      },
      transaction_id: TX,
      user_id: USER,
    });
  });

  it('filters each action of a batch and drops the ones that are not ours', () => {
    const filtered = filterMoesifRequest(`${HOST}/v1/actions/batch`, [
      sdkAction(),
      sdkAction({ action_name: 'Some-Other-Event' }),
    ]) as unknown[];

    expect(filtered).toHaveLength(1);
    expect(JSON.stringify(filtered)).not.toMatch(
      /acme-corp|payments|secret|alice|Payments API|203\.0/,
    );
  });

  it('leaves out IDs that are not UUIDs', () => {
    const filtered = filterMoesifRequest(
      `${HOST}/v1/actions`,
      sdkAction({ company_id: 'acme-corp', user_id: 'alice@example.com' }),
    );

    expect(filtered).not.toHaveProperty('company_id');
    expect(filtered).not.toHaveProperty('user_id');
  });

  it('drops a dimension value outside its enum', () => {
    const filtered = filterMoesifRequest(
      `${HOST}/v1/actions`,
      sdkAction({ metadata: { creation_method: 'Payments API' } }),
    ) as { metadata: Record<string, unknown> };

    expect(filtered.metadata).not.toHaveProperty('creation_method');
  });

  it('keeps only the IDs of a user update', () => {
    expect(
      filterMoesifRequest(`${HOST}/v1/users`, {
        anonymous_id: ANON,
        campaign: { utm_source: 'newsletter' },
        company_id: ORG,
        metadata: { email: 'alice@example.com', name: 'Alice' },
        user_id: USER,
      }),
    ).toEqual({ anonymous_id: ANON, company_id: ORG, user_id: USER });
  });

  it('keeps only the ID of a company update', () => {
    expect(
      filterMoesifRequest(`${HOST}/v1/companies`, {
        campaign: { utm_source: 'newsletter' },
        company_domain: 'acme.com',
        company_id: ORG,
        metadata: { name: 'Acme Corporation' },
      }),
    ).toEqual({ company_id: ORG });
  });

  it.each([
    ['a user update without a UUID', '/v1/users', { user_id: 'alice@example.com' }],
    ['a company update without a UUID', '/v1/companies', { company_id: 'acme-corp' }],
    [
      'an action that is not in the taxonomy',
      '/v1/actions',
      { action_name: 'Landing-SignIn-Viewed' },
    ],
    ['a batch with nothing left in it', '/v1/actions/batch', [{ action_name: 'x' }]],
    [
      'a captured API call',
      '/v1/events',
      { request: { uri: `${ORIGIN}/proxy/api/v0.9/rest-apis` } },
    ],
    ['a captured API call batch', '/v1/events/batch', []],
    ['an unknown route', '/v1/somewhere', {}],
  ])('drops %s', (_name, path, data) => {
    expect(filterMoesifRequest(`${HOST}${path}`, data)).toBeNull();
  });
});
