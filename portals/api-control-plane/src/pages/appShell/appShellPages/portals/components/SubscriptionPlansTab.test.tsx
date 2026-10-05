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
import { aSubscriptionPlan, collection, failure, recorder, type SubscriptionPlanFixture } from '@/test/msw';
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
  screen.getAllByText((_content, element) => element?.textContent === text)[0];

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
  it('lists active plans first, then the selected inactive one, flagging only the inactive one, counting only the active plans', async () => {
    server.use(collection(PLANS_PATH, [legacy, bronze, gold]));

    renderTab({ values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    await screen.findByText('Bronze');
    const names = ['Bronze', 'Gold', 'Legacy'];
    expect(
      screen.getAllByRole('checkbox').map((card) => names.find((name) => card.textContent?.startsWith(name))),
    ).toEqual(names);
    expect(screen.getByText('1,000')).toBeInTheDocument();
    expect(screen.getByText('requests / hour')).toBeInTheDocument();
    expect(screen.getAllByText('Unlimited')).toHaveLength(2);
    expect(screen.getByRole('checkbox', { name: /Legacy/ })).not.toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('checkbox', { name: /Gold/ })).not.toHaveAttribute('aria-disabled', 'true');
    // Only the inactive plan carries a status pill.
    expect(screen.queryByText('Active')).not.toBeInTheDocument();
    expect(screen.getByText('Inactive')).toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
  });

  it('loads every page of plans, not just the first 20', async () => {
    const many = Array.from({ length: 120 }, (_, i) =>
      aSubscriptionPlan({ displayName: `Plan ${i + 1}`, id: `plan-${i + 1}` }),
    );
    const requests = recorder();
    server.use(collection(PLANS_PATH, many, { record: requests }));

    renderTab({ values: values({ subscriptionPlanIds: ['plan-120'] }) });

    expect(await screen.findByRole('checkbox', { name: /Plan 120/ })).toBeInTheDocument();
    expect(screen.getAllByRole('checkbox')).toHaveLength(120);
    expect(summaryText('1 of 120 plans selected')).toBeInTheDocument();
    expect(requests.calls.map((call) => [call.params.get('limit'), call.params.get('offset')])).toEqual([
      ['100', '0'],
      ['100', '100'],
    ]);
  });

  it('shows an empty state and no Select all button only when the org has no plans at all', async () => {
    server.use(collection(PLANS_PATH, []));

    renderTab();

    expect(await screen.findByText('No subscription plans')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /select all|clear all/i })).not.toBeInTheDocument();
  });

  it('offers no inactive plan that is not selected, so an all-inactive org shows the empty state', async () => {
    server.use(collection(PLANS_PATH, [legacy]));

    renderTab();

    expect(await screen.findByText('No subscription plans')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /Legacy/ })).not.toBeInTheDocument();
  });

  it('shows an error state when the plans request fails', async () => {
    server.use(failure('get', PLANS_PATH, 500, 'INTERNAL_ERROR'));

    renderTab();

    expect(await screen.findByText('Unable to load subscription plans.')).toBeInTheDocument();
  });

  it('toggles an active plan on click, reporting the full updated selection', async () => {
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

  it('does not offer an inactive plan that is not selected', async () => {
    server.use(collection(PLANS_PATH, plans));

    renderTab();

    await screen.findByText('Bronze');
    expect(screen.queryByRole('checkbox', { name: /Legacy/ })).not.toBeInTheDocument();
    expect(summaryText('0 of 2 plans selected')).toBeInTheDocument();
  });

  it('keeps a selected inactive plan at full strength, counts it separately, and lets it be cleared', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { user } = renderTab({ onChange, values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    const legacyCard = await screen.findByRole('checkbox', { name: /Legacy/ });
    expect(legacyCard).toHaveAttribute('aria-checked', 'true');
    expect(legacyCard).not.toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByText('Inactive')).toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
    expect(screen.getByText('1 inactive plan selected')).toBeInTheDocument();

    await user.click(legacyCard);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ subscriptionPlanIds: ['bronze'] }));
  });

  it('Select all selects every active plan and keeps a selected inactive one; Clear all clears everything', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { rerender, user } = renderTab({ onChange, values: values({ subscriptionPlanIds: ['legacy'] }) });

    await screen.findByText('Bronze');
    await user.click(screen.getByRole('button', { name: 'Select all' }));

    const selectedIds: string[] = onChange.mock.calls[0][0].subscriptionPlanIds;
    expect([...selectedIds].sort()).toEqual(['bronze', 'gold', 'legacy']);

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

  it('read-only: shows exactly the plans the live listing holds, an inactive one badged, and nothing can be changed', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    renderTab({ onChange, readOnly: true, values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    await screen.findByText('Bronze');
    expect(screen.queryByRole('checkbox', { name: /Gold/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /select all|clear all/i })).not.toBeInTheDocument();
    expect(summaryText('2 plans published')).toBeInTheDocument();
    expect(screen.getByText('Inactive')).toBeInTheDocument();

    for (const name of [/Bronze/, /Legacy/]) {
      const card = screen.getByRole('checkbox', { name });
      expect(card).toHaveAttribute('aria-checked', 'true');
      expect(card).toHaveAttribute('aria-disabled', 'true');
      expect(card).toHaveAttribute('tabindex', '-1');
      // `pointerEvents: none` (asserted directly, since a real click can't even
      // reach an element in this state) is what makes the card inert — no click
      // handler is wired at all when `readOnly`.
      expect(card).toHaveStyle({ pointerEvents: 'none' });
    }
  });

  it('read-only: an inactive plan the listing holds is shown even when every plan is inactive, and an unpublished selection says so', async () => {
    server.use(collection(PLANS_PATH, [legacy]));

    const { rerender } = renderTab({ readOnly: true, values: values({ subscriptionPlanIds: ['legacy'] }) });

    expect(await screen.findByRole('checkbox', { name: /Legacy/ })).toHaveAttribute('aria-checked', 'true');
    expect(screen.queryByText('No subscription plans')).not.toBeInTheDocument();

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <SubscriptionPlansTab readOnly values={values()} />
      </ApiScopeProvider>,
    );
    expect(await screen.findByText('This listing has no subscription plans.')).toBeInTheDocument();
  });
});
