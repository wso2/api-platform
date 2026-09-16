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

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aGraphQLApi, aRestApi, collection } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, within } from '@/test/utils';
import { ProjectStatistics } from './ProjectStatistics';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';

const renderStats = () =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <ProjectStatistics onTypeFilterChange={vi.fn()} selectedType={null} />
    </ApiScopeProvider>,
  );

beforeEach(() => {
  resetHttpClient();
});

describe('ProjectStatistics — GraphQL APIs count alongside REST', () => {
  it('adds GraphQL APIs into the combined total, not just REST', async () => {
    server.use(
      collection('/rest-apis', [aRestApi({ id: 'orders-api' })]),
      collection('/graphql-apis', [
        aGraphQLApi({ id: 'countries-graphql-api' }),
        aGraphQLApi({ id: 'books-graphql-api' }),
      ]),
    );

    renderStats();

    // 1 REST + 2 GraphQL — not "1", which is what the REST-only total was
    // before GraphQL APIs were wired into this count.
    expect(await screen.findByText('3')).toBeInTheDocument();
  });

  it('reflects a newly created GraphQL API in the "GraphQL" stat card, not a permanent zero', async () => {
    server.use(
      collection('/rest-apis', []),
      collection('/graphql-apis', [aGraphQLApi({ id: 'countries-graphql-api' })]),
    );

    renderStats();

    const graphqlCard = screen.getByRole('button', { name: 'Filter APIs by GraphQL' });
    expect(await within(graphqlCard).findByText('1')).toBeInTheDocument();
  });

  it('shows zero for GraphQL when the project genuinely has none', async () => {
    server.use(
      collection('/rest-apis', [aRestApi({ id: 'orders-api' })]),
      collection('/graphql-apis', []),
    );

    renderStats();

    const graphqlCard = screen.getByRole('button', { name: 'Filter APIs by GraphQL' });
    expect(await within(graphqlCard).findByText('0')).toBeInTheDocument();
  });

  it('counts GraphQL APIs as published so the caption sums to the total', async () => {
    // Regression test: this app has no field to read a GraphQL API's real
    // dev-portal publish state from (a deliberate backend removal, not a
    // missing feature — see ProjectStatistics.tsx), so excluding them from
    // this caption entirely left it never summing to the total above it (1
    // REST created + 2 GraphQL = "3" total, but "0 published · 1 created").
    server.use(
      collection('/rest-apis', [aRestApi({ id: 'orders-api', lifeCycleStatus: 'CREATED' })]),
      collection('/graphql-apis', [
        aGraphQLApi({ id: 'countries-graphql-api' }),
        aGraphQLApi({ id: 'books-graphql-api' }),
      ]),
    );

    renderStats();

    expect(await screen.findByText('3')).toBeInTheDocument();
    expect(screen.getByText('2 published · 1 created')).toBeInTheDocument();
  });
});
