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
import { buildAction, maskPath, maskUri, type ActionContext } from './actions';

const ORG = 'b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f';
const USER = '0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b';
const SESSION = '5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f';
const ORIGIN = 'https://console.example.com';

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

describe('buildAction', () => {
  const context: ActionContext = {
    pageUrl: `${ORIGIN}${routes.newApi('acme-corp', 'payments')}?q=secret#step-2`,
    sessionToken: SESSION,
    time: new Date('2026-11-10T09:30:00.000Z'),
    userAgent: 'Mozilla/5.0',
  };

  it('builds the action from the taxonomy, with the page reduced to its route template', () => {
    expect(
      buildAction(
        'apip.manage-apis.rest-api-create-click',
        { creation_method: 'scratch' },
        { companyId: ORG, userId: USER },
        context,
      ),
    ).toEqual({
      action_name: 'apip.manage-apis.rest-api-create-click',
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
        time: '2026-11-10T09:30:00.000Z',
        uri: `${ORIGIN}${routes.newApi()}`,
        user_agent_string: 'Mozilla/5.0',
      },
      session_token: SESSION,
      user_id: USER,
    });
  });

  it('keeps only the dimensions the feature declares, and only values in their enums', () => {
    const { metadata } = buildAction(
      'apip.manage-apis.rest-api-create-click',
      { apiName: 'Payments API', creation_method: 'Payments API' } as never,
      {},
      context,
    );

    expect(metadata).not.toHaveProperty('apiName');
    expect(metadata).not.toHaveProperty('creation_method');
  });

  it('leaves out IDs that are not UUIDs, and fields it does not have', () => {
    const action = buildAction(
      'apip.manage-apis.rest-api-create-click',
      { creation_method: 'scratch' },
      { companyId: 'acme-corp', userId: 'alice@example.com' },
      { pageUrl: context.pageUrl, sessionToken: 'not-a-uuid', time: context.time },
    );

    expect(action).not.toHaveProperty('company_id');
    expect(action).not.toHaveProperty('user_id');
    expect(action).not.toHaveProperty('session_token');
    expect(action.request).not.toHaveProperty('user_agent_string');
    expect(JSON.stringify(action)).not.toMatch(/acme-corp|payments|secret|alice|step-2/);
  });

  it('keeps the deployment’s base path in the page URL', () => {
    const { request } = buildAction(
      'apip.manage-apis.rest-api-create-click',
      { creation_method: 'scratch' },
      {},
      {
        ...context,
        basePath: '/console',
        pageUrl: `${ORIGIN}/console${routes.organizationHome('acme-corp')}`,
      },
    );

    expect(request.uri).toBe(`${ORIGIN}/console${routes.organizationHome()}`);
  });
});
