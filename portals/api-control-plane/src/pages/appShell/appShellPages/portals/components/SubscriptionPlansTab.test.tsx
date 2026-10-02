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
import { aSubscriptionPlan, collection, failure, type SubscriptionPlanFixture } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { emptyDraftFormValues, type DraftFormValues } from '../utils/publicationForm';
import { SubscriptionPlansTab, type SubscriptionPlansTabProps } from './SubscriptionPlansTab';

const ORG = 'api-platform-demo';
const PLANS_PATH = '/subscription-plans';

const bronze = aSubscriptionPlan({
  displayName: 'Bronze',
  id: 'bronze',
  limits: [
    { limitCount: 1000, limitType: 'REQUEST_COUNT', stopOnQuotaReach: true, timeAmount: 1, timeUnit: 'HOUR' },
  ],
});
const gold = aSubscriptionPlan({ displayName: 'Gold', id: 'gold' }); // no limits — "Unlimited"
const legacy = aSubscriptionPlan({ displayName: 'Legacy', id: 'legacy', status: 'INACTIVE' });

const plans: SubscriptionPlanFixture[] = [bronze, gold, legacy];

const values = (overrides: Partial<DraftFormValues> = {}): DraftFormValues => ({
  ...emptyDraftFormValues,
  ...overrides,
});

/** Matches text split across the summary row's nested count element. */
const summaryText = (text: string) =>
  screen.getByText((_content, element) => element?.textContent === text);

function renderTab(props: Partial<SubscriptionPlansTabProps> = {}) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <SubscriptionPlansTab values={values()} {...props} />
    </ApiScopeProvider>,
    { scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('SubscriptionPlansTab', () => {
  it('lists only active plans with their limit display and the selected count', async () => {
    server.use(collection(PLANS_PATH, plans));

    renderTab({ values: values({ subscriptionPlanIds: ['bronze'] }) });

    expect(await screen.findByText('Bronze')).toBeInTheDocument();
    expect(screen.getByText('Gold')).toBeInTheDocument();
    expect(screen.queryByText('Legacy')).not.toBeInTheDocument();
    expect(screen.getByText('1,000')).toBeInTheDocument();
    expect(screen.getByText('requests / hour')).toBeInTheDocument();
    expect(screen.getByText('Unlimited')).toBeInTheDocument();
    expect(screen.getByText('no request cap')).toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
  });

  it('shows an empty state and no Select all button when the org has no active plans', async () => {
    server.use(collection(PLANS_PATH, [legacy]));

    renderTab();

    expect(await screen.findByText('No active subscription plans')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /select all|clear all/i })).not.toBeInTheDocument();
  });

  it('shows an error state when the plans request fails', async () => {
    server.use(failure('get', PLANS_PATH, 500, 'INTERNAL_ERROR'));

    renderTab();

    expect(await screen.findByText('Unable to load subscription plans.')).toBeInTheDocument();
  });

  it('toggles a plan on click, reporting the full updated selection', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { user } = renderTab({ onChange, values: values({ subscriptionPlanIds: ['bronze'] }) });

    await screen.findByText('Gold');
    await user.click(screen.getByRole('checkbox', { name: /Gold/ }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ subscriptionPlanIds: expect.arrayContaining(['bronze', 'gold']) }),
    );
    expect(onChange.mock.calls[0][0].subscriptionPlanIds).toHaveLength(2);
  });

  it('Select all selects every active plan; the label becomes Clear all once all are selected', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { rerender, user } = renderTab({ onChange, values: values() });

    await screen.findByText('Bronze');
    await user.click(screen.getByRole('button', { name: 'Select all' }));

    const selectedIds: string[] = onChange.mock.calls[0][0].subscriptionPlanIds;
    expect(selectedIds.sort()).toEqual(['bronze', 'gold']);

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <SubscriptionPlansTab onChange={onChange} values={values({ subscriptionPlanIds: selectedIds })} />
      </ApiScopeProvider>,
    );

    expect(await screen.findByRole('button', { name: 'Clear all' })).toBeInTheDocument();
    expect(summaryText('2 of 2 plans selected')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Clear all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ subscriptionPlanIds: [] }));
  });

  it('renders the published selection read-only: no Select all button, and cards cannot be changed', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    renderTab({ onChange, readOnly: true, values: values({ subscriptionPlanIds: ['bronze'] }) });

    await screen.findByText('Bronze');
    expect(screen.queryByRole('button', { name: /select all|clear all/i })).not.toBeInTheDocument();

    const bronzeCard = screen.getByRole('checkbox', { name: /Bronze/ });
    expect(bronzeCard).toHaveAttribute('aria-checked', 'true');
    expect(bronzeCard).toHaveAttribute('aria-disabled', 'true');
    expect(bronzeCard).toHaveAttribute('tabindex', '-1');
    // `pointerEvents: none` (asserted directly, since a real click can't even
    // reach an element in this state) is what makes the card inert — no click
    // handler is wired at all when `readOnly`.
    expect(bronzeCard).toHaveStyle({ pointerEvents: 'none' });
  });
});
