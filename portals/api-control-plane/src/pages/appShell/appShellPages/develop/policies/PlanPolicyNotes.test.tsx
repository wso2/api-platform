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
  aPublication,
  aPublicationSummary,
  aSubscriptionPlan,
  collection,
  failure,
  resource,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { PlanPolicyNotes } from './PlanPolicyNotes';

const ORG = 'api-platform-demo';
const API = 'catfacts-api';

const window1k = { limits: [{ count: 1000, timeAmount: 1, timeUnit: 'HOUR' }], onExceed: 'reject' };

const gold = aSubscriptionPlan({
  conditions: [
    { id: 'requests', params: window1k, type: 'request-count' },
    { id: 'tokens', params: { ...window1k, tokenType: 'total' }, type: 'token-count' },
    { id: 'business-hours', params: { from: 9, to: 17 }, type: 'custom' },
  ],
  displayName: 'Gold',
  id: 'gold',
});
const silver = aSubscriptionPlan({
  conditions: [
    { id: 'requests', params: window1k, type: 'request-count' },
    { id: 'tool-calls', params: window1k, type: 'tool-call-count' },
  ],
  displayName: 'Silver',
  id: 'silver',
});
/** Not offered by the API: its conditions must not show. */
const bronze = aSubscriptionPlan({
  conditions: [{ id: 'events', params: window1k, type: 'event-count' }],
  displayName: 'Bronze',
  id: 'bronze',
});

const publicationPath = (portal: string) =>
  `/api-portals/${portal}/apis/rest-api/${API}/publication`;
const selection = (
  id: string,
  sourceStatus: 'UNCHANGED' | 'UPDATED' | 'DELETED' = 'UNCHANGED',
) => ({
  displayName: id,
  id,
  sourceStatus,
});

/** The API live on two portals (one deprecated) and only drafted on a third. */
const serveOffered = () =>
  server.use(
    collection('/api-publications', [
      aPublicationSummary({ apiPortalId: 'portal-a', status: 'PUBLISHED' }),
      aPublicationSummary({ apiPortalId: 'portal-b', status: 'DEPRECATED' }),
      aPublicationSummary({ apiPortalId: 'portal-c', status: 'NOT_PUBLISHED' }),
    ]),
    resource(publicationPath('portal-a'), aPublication({ subscriptionPlans: [selection('gold')] })),
    resource(
      publicationPath('portal-b'),
      aPublication({ subscriptionPlans: [selection('silver'), selection('bronze', 'DELETED')] }),
    ),
    collection('/subscription-plans', [gold, silver, bronze]),
  );

const renderNotes = (attached: string[]) =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <PlanPolicyNotes apiHandle={API} attached={new Set(attached)} />
    </ApiScopeProvider>,
  );

/** The notes row of one condition ID. */
const rowOf = (id: string) => screen.getByText(id).closest('li') as HTMLElement;

beforeEach(() => {
  resetHttpClient();
});

describe('PlanPolicyNotes', () => {
  it('lists one row per built-in type, then each custom ID, for the plans the live listings hold', async () => {
    serveOffered();

    renderNotes(['subscription-validation']);

    expect(await screen.findByText('requests')).toBeInTheDocument();
    const ids = screen
      .getAllByRole('listitem')
      .map((item) => item.querySelector('.MuiChip-label')?.textContent);
    expect(ids).toEqual(['requests', 'tool-calls', 'tokens', 'business-hours']);
    // Bronze was deleted from the deprecated listing, and portal-c is only a draft.
    expect(screen.queryByText('events')).not.toBeInTheDocument();
  });

  it('says what enforces each condition when subscription-validation is attached', async () => {
    serveOffered();

    renderNotes(['jwt-auth', 'subscription-validation']);

    await screen.findByText('requests');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(rowOf('requests')).toHaveTextContent('Request count');
    expect(rowOf('requests')).toHaveTextContent('Enforced');
    expect(rowOf('requests')).toHaveTextContent('By subscription-validation');
    for (const id of ['tool-calls', 'tokens']) {
      expect(rowOf(id)).toHaveTextContent('No policy yet');
      expect(rowOf(id)).toHaveTextContent('No policy enforces this yet');
    }
    expect(rowOf('business-hours')).toHaveTextContent('Custom');
    expect(rowOf('business-hours')).toHaveTextContent('No policy reads it yet');
    expect(screen.getByText('1 of 4 enforced')).toBeInTheDocument();
    expect(
      screen.getByText(
        'For the plans this API offers. A plan condition applies only where a policy enforces it. You pick the plans when you publish to the API Portal.',
      ),
    ).toBeInTheDocument();
  });

  it('asks for subscription-validation first when it is missing, and marks request-count not enforced', async () => {
    serveOffered();

    renderNotes(['jwt-auth']);

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Attach subscription-validation first. Without it no plan, condition or subscription is checked.',
    );
    expect(rowOf('requests')).toHaveTextContent('Not enforced');
    expect(rowOf('requests')).toHaveTextContent(
      'Not enforced. Attach subscription-validation and redeploy.',
    );
    expect(screen.getByText('0 of 4 enforced')).toBeInTheDocument();
  });

  it('offers notes only: no button to add or attach anything', async () => {
    serveOffered();

    renderNotes([]);

    await screen.findByText('requests');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('says the API offers no plans when no live listing holds one', async () => {
    server.use(
      collection('/api-publications', [
        aPublicationSummary({ apiPortalId: 'portal-c', status: 'NOT_PUBLISHED' }),
      ]),
      collection('/subscription-plans', [gold]),
    );

    renderNotes([]);

    expect(await screen.findByText(/This API offers no subscription plans\./)).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByText(/enforced/)).not.toBeInTheDocument();
  });

  it('says the plans are unlimited when they hold no conditions, still asking for subscription-validation', async () => {
    server.use(
      collection('/api-publications', [
        aPublicationSummary({ apiPortalId: 'portal-a', status: 'PUBLISHED' }),
      ]),
      resource(
        publicationPath('portal-a'),
        aPublication({ subscriptionPlans: [selection('free')] }),
      ),
      collection('/subscription-plans', [aSubscriptionPlan({ displayName: 'Free', id: 'free' })]),
    );

    renderNotes([]);

    expect(
      await screen.findByText('The plans this API offers have no conditions: they are unlimited.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('Attach subscription-validation first.');
  });

  it('says so when the plans cannot be loaded', async () => {
    server.use(
      failure('get', '/api-publications', 500, 'INTERNAL_ERROR'),
      collection('/subscription-plans', [gold]),
    );

    renderNotes([]);

    expect(
      await screen.findByText('Unable to load the subscription plans this API offers.'),
    ).toBeInTheDocument();
  });
});
