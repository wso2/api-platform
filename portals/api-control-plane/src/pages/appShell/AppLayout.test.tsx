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

import type { GraphQLApiDetail } from '@/api/resources/graphqlApis';
import type { RestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';
import { buildScopeCrumbs } from './AppLayout';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const HOME = 'Home';

describe('buildScopeCrumbs', () => {
  it('stops at the project when no API is in scope', () => {
    const crumbs = buildScopeCrumbs({ orgHandle: ORG, projectHandler: PROJECT }, HOME);

    expect(crumbs.map((crumb) => crumb.key)).toEqual(['org', 'project']);
  });

  it('adds the REST API crumb, preferring its display name over the raw handle', () => {
    const crumbs = buildScopeCrumbs(
      { apiHandler: 'orders-api', orgHandle: ORG, projectHandler: PROJECT },
      HOME,
      undefined,
      { displayName: 'Orders API' } as RestApi,
    );

    expect(crumbs.at(-1)).toEqual({
      key: 'api',
      label: 'Orders API',
      path: routes.api(ORG, PROJECT, 'orders-api'),
    });
  });

  /*
   * A GraphQL API sets `params.graphqlApiHandler`, never `params.apiHandler`
   * (see `graphqlApiPath`'s doc comment) — without its own branch reading
   * `graphqlApiHandler`/`graphqlComponent`, the trail silently stopped one
   * level short, at the project, for every GraphQL page.
   */
  it('adds the GraphQL API crumb, preferring its display name over the raw handle', () => {
    const crumbs = buildScopeCrumbs(
      { graphqlApiHandler: 'countries-graphql-api', orgHandle: ORG, projectHandler: PROJECT },
      HOME,
      undefined,
      undefined,
      { displayName: 'Countries API' } as GraphQLApiDetail,
    );

    expect(crumbs.at(-1)).toEqual({
      key: 'api',
      label: 'Countries API',
      path: routes.graphqlApi(ORG, PROJECT, 'countries-graphql-api'),
    });
  });

  it('falls back to the raw handle when the GraphQL API hasn’t loaded yet', () => {
    const crumbs = buildScopeCrumbs(
      { graphqlApiHandler: 'countries-graphql-api', orgHandle: ORG, projectHandler: PROJECT },
      HOME,
    );

    expect(crumbs.at(-1)?.label).toBe('countries-graphql-api');
  });
});
