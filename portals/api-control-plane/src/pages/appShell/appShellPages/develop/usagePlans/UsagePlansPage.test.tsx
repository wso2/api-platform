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
import {
  accepts,
  aGraphQLApiDetail,
  aRestApi,
  aSubscriptionPlan,
  collection,
  failure,
  recorder,
  resource,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { GraphqlUsagePlansPanel, graphqlUsagePlansMetadata, UsagePlansPanel } from './UsagePlansPage';

const ORG = 'api-platform-demo';

const window10 = { limits: [{ count: 10, timeAmount: 1, timeUnit: 'MINUTE' }], onExceed: 'reject' };
const gold = aSubscriptionPlan({
  conditions: [{ id: 'requests', params: window10, type: 'request-count' }],
  description: 'For production traffic.',
  displayName: 'Gold',
  id: 'gold',
});
const silver = aSubscriptionPlan({
  conditions: [{ id: 'requests', params: window10, type: 'request-count' }],
  displayName: 'Silver',
  id: 'silver',
});
const legacy = aSubscriptionPlan({ displayName: 'Legacy', id: 'legacy', status: 'INACTIVE' });

const renderPanel = (overrides: Parameters<typeof aRestApi>[0] = {}) => {
  const api = aRestApi({ policies: [{ name: 'cors', params: {}, version: 'v1' }], ...overrides });
  return {
    api,
    ...renderWithProviders(
      <ApiScopeProvider orgId={ORG}>
        <UsagePlansPanel api={api} />
      </ApiScopeProvider>,
    ),
  };
};

beforeEach(() => {
  resetHttpClient();
  server.use(collection('/subscription-plans', [gold, silver, legacy]));
});

const rowOf = (name: string) => screen.getByText(name).closest('[role="row"]') as HTMLElement;
const boxOf = async (name: string) => {
  await screen.findByText(name);
  return within(rowOf(name)).getByRole('checkbox');
};

describe('UsagePlansPanel', () => {
  it('lists the active plans with their descriptions, ticking those the API offers', async () => {
    renderPanel({ subscriptionPlans: [{ id: 'silver' }] });

    expect(await boxOf('Silver')).toBeChecked();
    expect(await boxOf('Gold')).not.toBeChecked();
    expect(screen.queryByText('Legacy')).not.toBeInTheDocument();
    expect(screen.getByText('For production traffic.')).toBeInTheDocument();
    expect(screen.queryByText('Required by usage plans')).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Saving the changes stores the plans and policies on the API. The gateway applies them the next time you deploy it.',
      ),
    ).toBeInTheDocument();
  });

  it('lists only the required policies the API has not attached yet', async () => {
    renderPanel({ subscriptionPlans: [{ id: 'silver' }] });

    const note = (await screen.findByText('You have not yet attached the following required policies:')).closest(
      '[role="alert"]',
    ) as HTMLElement;
    expect(within(note).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
      'subscription-validation v1',
    ]);
  });

  it('shows no missing-policy note once the policies are attached', async () => {
    renderPanel({
      policies: [{ name: 'subscription-validation', params: {}, version: 'v1' }],
      subscriptionPlans: [{ id: 'silver' }],
    });

    await screen.findByText('Silver');
    expect(screen.queryByText('You have not yet attached the following required policies:')).not.toBeInTheDocument();
  });

  it('offers to attach the policies a ticked plan needs, as a list, and attaches them on confirm', async () => {
    const { user } = renderPanel();

    await user.click(await boxOf('Gold'));

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('Gold needs these policies, which this API does not have yet.');
    expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
      'subscription-validation v1',
    ]);
    expect(dialog).toHaveTextContent('They are added to the API’s global policies. Save and deploy the API for them to take effect.');
    await user.click(within(dialog).getByRole('button', { name: 'Attach policies' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.queryByText('You have not yet attached the following required policies:')).not.toBeInTheDocument();
  });

  it('keeps the plan ticked but adds no policy when the offer is declined', async () => {
    const { user } = renderPanel();

    await user.click(await boxOf('Gold'));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Skip' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    expect(await boxOf('Gold')).toBeChecked();
    expect(await screen.findByText('You have not yet attached the following required policies:')).toBeInTheDocument();
  });

  it('offers removal, as a list, only once no ticked plan needs the policy', async () => {
    const { user } = renderPanel({
      policies: [{ name: 'subscription-validation', params: {}, version: 'v1' }],
      subscriptionPlans: [{ id: 'gold' }, { id: 'silver' }],
    });

    await user.click(await boxOf('Gold'));
    // Silver still needs subscription-validation.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(await boxOf('Silver'));
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('No plan attached to this API uses these policies any more.');
    expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
      'subscription-validation v1',
    ]);
    expect(dialog).toHaveTextContent('They are removed from the API’s global policies when you save.');
    await user.click(within(dialog).getByRole('button', { name: 'Remove policies' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('attaching every plan at once asks once, for all of them', async () => {
    const { user } = renderPanel();

    await screen.findByText('Gold');
    await user.click(await screen.findByRole('checkbox', { name: 'Attach all plans' }));

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('The selected plans need these policies, which this API does not have yet.');
    await user.click(within(dialog).getByRole('button', { name: 'Skip' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(await boxOf('Gold')).toBeChecked();
    expect(await boxOf('Silver')).toBeChecked();
  });

  it('saves the plans and policies in one update of the API', async () => {
    const updates = recorder();
    const { api, user } = renderPanel();
    server.use(accepts('put', '/rest-apis/:restApiId', api, { record: updates }));

    await user.click(await boxOf('Gold'));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Attach policies' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await user.click(await screen.findByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(updates.count()).toBe(1));
    expect(JSON.parse(updates.last()?.body ?? '{}')).toMatchObject({
      policies: [
        { name: 'cors', version: 'v1' },
        { name: 'subscription-validation', params: { subscriptionKeyHeader: 'Subscription-Key' }, version: 'v1' },
      ],
      subscriptionPlans: [{ id: 'gold' }],
    });
  });

  describe('subscription-validation on the resources', () => {
    const regional = aSubscriptionPlan({
      conditions: [{ id: 'region', params: { policy: { name: 'region-guard', version: 'v1' } }, type: 'custom' }],
      displayName: 'Regional',
      id: 'regional',
    });
    const sv = (header: string) => ({
      name: 'subscription-validation',
      params: { subscriptionKeyHeader: header },
      version: 'v1',
    });
    const op = (path: string, policies: { name: string; params: Record<string, unknown>; version: string }[]) => ({
      name: path,
      request: { method: 'GET' as const, path, policies },
    });
    beforeEach(() => server.use(collection('/subscription-plans', [regional])));

    it('moves it to the API, listed as moved, and saves it once with its key header', async () => {
      const updates = recorder();
      const { api, user } = renderPanel({
        operations: [op('/a', [sv('X-Key')]), op('/b', [{ name: 'cors', params: {}, version: 'v1' }, sv('X-Key')])],
      });
      server.use(accepts('put', '/rest-apis/:restApiId', api, { record: updates }));

      await user.click(await boxOf('Regional'));
      const dialog = await screen.findByRole('dialog');
      expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
        'subscription-validation v1',
        'region-guard v1',
      ]);
      expect(within(dialog).getAllByRole('listitem')[0]).toHaveTextContent('Moved from the API’s resources to the API');
      expect(within(dialog).getAllByRole('listitem')[1]).not.toHaveTextContent('Moved');
      await user.click(within(dialog).getByRole('button', { name: 'Attach policies' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      await user.click(await screen.findByRole('button', { name: 'Save changes' }));

      await waitFor(() => expect(updates.count()).toBe(1));
      const body = JSON.parse(updates.last()?.body ?? '{}') as {
        operations: { request: { policies?: { name: string }[] } }[];
        policies: { name: string; params?: unknown }[];
      };
      // Validation first, before the policy that reads it; the header the resources used is kept.
      expect(body.policies.map((policy) => policy.name)).toEqual(['subscription-validation', 'cors', 'region-guard']);
      expect(body.policies[0].params).toEqual({ subscriptionKeyHeader: 'X-Key' });
      expect(body.operations.map((o) => (o.request.policies ?? []).map((policy) => policy.name))).toEqual([[], ['cors']]);
    });

    it('attaches nothing when only some resources have it, and points to the Policies page', async () => {
      const { user } = renderPanel({ operations: [op('/a', [sv('X-Key')]), op('/b', [])] });

      await user.click(await boxOf('Regional'));
      const dialog = await screen.findByRole('dialog');
      expect(dialog).toHaveTextContent('Attach the policies on the Policies page');
      expect(dialog).toHaveTextContent('Only some of this API’s resources have Subscription Validation');
      expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
        'region-guard v1',
      ]);
      await user.click(within(dialog).getByRole('button', { name: 'OK' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(await boxOf('Regional')).toBeChecked();
      expect(await screen.findByText('You have not yet attached the following required policies:')).toBeInTheDocument();
    });

    it('attaches nothing when the resources configure it differently', async () => {
      const updates = recorder();
      const { api, user } = renderPanel({ operations: [op('/a', [sv('X-Key')]), op('/b', [sv('Y-Key')])] });
      server.use(accepts('put', '/rest-apis/:restApiId', api, { record: updates }));

      await user.click(await boxOf('Regional'));
      const dialog = await screen.findByRole('dialog');
      expect(dialog).toHaveTextContent('configure Subscription Validation differently');
      await user.click(within(dialog).getByRole('button', { name: 'OK' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      await user.click(await screen.findByRole('button', { name: 'Save changes' }));

      await waitFor(() => expect(updates.count()).toBe(1));
      const body = JSON.parse(updates.last()?.body ?? '{}') as {
        operations: { request: { policies?: { params?: unknown }[] } }[];
        policies: { name: string }[];
      };
      expect(body.policies.map((policy) => policy.name)).toEqual(['cors']);
      expect(body.operations.map((o) => o.request.policies?.[0]?.params)).toEqual([
        { subscriptionKeyHeader: 'X-Key' },
        { subscriptionKeyHeader: 'Y-Key' },
      ]);
    });

    it('leaves the resources alone when the API already has it', async () => {
      const { user } = renderPanel({
        operations: [op('/a', [sv('X-Key')])],
        policies: [sv('Subscription-Key')],
      });

      await user.click(await boxOf('Regional'));
      const dialog = await screen.findByRole('dialog');
      expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
        'region-guard v1',
      ]);
      expect(dialog).not.toHaveTextContent('Moved');
    });
  });

  it('cannot change a gateway-owned API', async () => {
    renderPanel({ readOnly: true });
    expect(await boxOf('Gold')).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
  });
});

describe('GraphqlUsagePlansPanel', () => {
  const renderGraphql = (
    overrides: Parameters<typeof aGraphQLApiDetail>[0] = {},
    sdl = resource('/graphql-apis/:graphqlApiId/sdl', { sdl: 'type Query { a: String }' }),
  ) => {
    const api = aGraphQLApiDetail({
      id: 'countries',
      policies: [],
      schemaSource: 'introspection',
      subscriptionPlans: [],
      ...overrides,
    });
    server.use(sdl);
    return {
      api,
      ...renderWithProviders(
        <ApiScopeProvider orgId={ORG}>
          <GraphqlUsagePlansPanel api={api} />
        </ApiScopeProvider>,
      ),
    };
  };

  it('lists the plans and offers the policies a ticked plan needs, as for a REST API', async () => {
    const { user } = renderGraphql();

    await user.click(await boxOf('Gold'));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getAllByRole('listitem').map((item) => item.getAttribute('aria-label'))).toEqual([
      'subscription-validation v1',
    ]);
  });

  it('saves through the GraphQL update endpoint', async () => {
    const updates = recorder();
    const { api, user } = renderGraphql();
    server.use(accepts('put', '/graphql-apis/:graphqlApiId', api, { record: updates }));
    await user.click(await boxOf('Gold'));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Attach policies' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await user.click(await screen.findByRole('button', { name: 'Save changes' }));

    await waitFor(() => expect(updates.count()).toBe(1));
    expect(updates.last()?.method).toBe('PUT');
  });

  it('builds the GraphQL update metadata with the plans, the policies and the schema resupplied', () => {
    const policies = [{ name: 'subscription-validation', params: {}, version: 'v1.0' }];
    const introspected = aGraphQLApiDetail({ id: 'countries', schemaSource: 'introspection' });
    expect(graphqlUsagePlansMetadata(introspected, ['gold'], policies, undefined)).toMatchObject({
      id: 'countries',
      policies: [{ name: 'subscription-validation', version: 'v1' }],
      schemaSource: 'introspection',
      subscriptionPlans: [{ id: 'gold' }],
    });
    const inline = aGraphQLApiDetail({ id: 'countries', schemaSource: 'inline' });
    expect(graphqlUsagePlansMetadata(inline, [], [], 'type Query { a: String }')).toMatchObject({
      schemaSource: 'inline',
      sdl: 'type Query { a: String }',
      subscriptionPlans: [],
    });
    // No SDL for an inline schema: nothing to send, rather than an empty schema.
    expect(graphqlUsagePlansMetadata(inline, [], [], undefined)).toBeNull();
    expect(graphqlUsagePlansMetadata(inline, [], [], '')).toBeNull();
  });

  it('keeps Save off and offers a retry while an inline schema can’t be loaded', async () => {
    const { user } = renderGraphql(
      { schemaSource: 'inline' },
      failure('get', '/graphql-apis/:graphqlApiId/sdl', 500, 'INTERNAL_ERROR'),
    );
    expect(
      await screen.findByText('Unable to load the API’s schema, so the plans and policies can’t be saved yet.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    await user.click(await boxOf('Gold'));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Skip' }));
    expect(await screen.findByRole('button', { name: 'Save changes' })).toBeDisabled();
  });

  it('cannot change a gateway-owned GraphQL API', async () => {
    renderGraphql({ readOnly: true });
    expect(await boxOf('Gold')).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
  });
});
