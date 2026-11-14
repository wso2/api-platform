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
import {
  aPlanPricing,
  aSubscriptionPlan,
  collection,
  failure,
  recorder,
  type SubscriptionPlanFixture,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { emptyDraftFormValues, type DraftFormValues } from '../utils/publicationForm';
import { SubscriptionPlansTab, type SubscriptionPlansTabProps } from './SubscriptionPlansTab';

const ORG = 'api-platform-demo';
const PLANS_PATH = '/subscription-plans';

const bronze = aSubscriptionPlan({
  conditions: [
    {
      id: 'requests',
      params: { limits: [{ count: 1000, timeAmount: 1, timeUnit: 'HOUR' }], onExceed: 'reject' },
      type: 'request-count',
    },
  ],
  description: 'For trying the API out.',
  displayName: 'Bronze',
  id: 'bronze',
  limits: [
    { limitCount: 1000, limitType: 'REQUEST_COUNT', stopOnQuotaReach: true, timeAmount: 1, timeUnit: 'HOUR' },
  ],
});
const gold = aSubscriptionPlan({ description: 'For production traffic.', displayName: 'Gold', id: 'gold' });
const legacy = aSubscriptionPlan({ displayName: 'Legacy', id: 'legacy', status: 'INACTIVE' });

const silver = aSubscriptionPlan({ displayName: 'Silver', id: 'silver' });
const zinc = aSubscriptionPlan({ displayName: 'Zinc', id: 'zinc' });

const plans: SubscriptionPlanFixture[] = [bronze, gold, legacy];

const values = (overrides: Partial<DraftFormValues> = {}): DraftFormValues => ({
  ...emptyDraftFormValues,
  ...overrides,
});

/** The plan names in the order the cards are shown. */
const cardOrder = (names: string[]) =>
  screen.getAllByRole('checkbox').map((card) => names.find((name) => card.textContent?.startsWith(name)));

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
  it('lists the active plans only, counting the selected ones, and leaves an inactive plan out with no trace', async () => {
    server.use(collection(PLANS_PATH, [legacy, bronze, gold]));

    renderTab({ values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    await screen.findByText('Bronze');
    expect(cardOrder(['Bronze', 'Gold', 'Legacy'])).toEqual(['Bronze', 'Gold']);
    expect(screen.getByText('For trying the API out.')).toBeInTheDocument();
    expect(screen.getByText('For production traffic.')).toBeInTheDocument();
    expect(screen.queryByText(/Legacy|Inactive/)).not.toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
  });

  it("orders the saved selection first, then the rest, each in the server's order", async () => {
    server.use(collection(PLANS_PATH, [bronze, gold, silver, legacy, zinc]));

    renderTab({ values: values({ subscriptionPlanIds: ['zinc', 'gold'] }) });

    await screen.findByText('Bronze');
    const names = ['Gold', 'Zinc', 'Bronze', 'Silver'];
    expect(cardOrder(names)).toEqual(names);
  });

  it('orders by the saved selection, so a plan selected but not yet saved stays where it was', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold, silver]));

    renderTab({ savedPlanIds: ['gold'], values: values({ subscriptionPlanIds: ['gold', 'silver'] }) });

    await screen.findByText('Bronze');
    expect(cardOrder(['Gold', 'Bronze', 'Silver'])).toEqual(['Gold', 'Bronze', 'Silver']);
  });

  it('keeps a saved plan in the saved group when it is deselected, so a click never moves a card', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold, silver]));

    renderTab({ savedPlanIds: ['silver'], values: values() });

    await screen.findByText('Bronze');
    expect(cardOrder(['Silver', 'Bronze', 'Gold'])).toEqual(['Silver', 'Bronze', 'Gold']);
  });

  it('read-only: lists an inactive plan the listing holds before the active ones', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold, legacy]));

    renderTab({ readOnly: true, values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    await screen.findByText('Legacy');
    expect(cardOrder(['Legacy', 'Bronze'])).toEqual(['Legacy', 'Bronze']);
  });

  it("shows each plan's name and description only: no limit and no price", async () => {
    const starter = aSubscriptionPlan({
      description: 'Small teams.',
      displayName: 'Starter',
      expiryTime: '2027-03-31T00:00:00Z',
      id: 'starter',
      pricing: aPlanPricing({
        subscriptionPlanId: 'starter',
        pricingType: 'FLAT_RATE',
        flatAmount: '29.00',
        prices: [],
      }),
    });
    // No meter handler: a card no longer reads one, and an unhandled request fails the test.
    server.use(collection(PLANS_PATH, [bronze, starter]));

    renderTab();

    const card = await screen.findByRole('checkbox', { name: /Starter/ });
    expect(card).toHaveTextContent('Small teams.');
    expect(card).toHaveTextContent(/Expires .*2027/);
    expect(card).not.toHaveTextContent('$');
    const bronzeCard = screen.getByRole('checkbox', { name: /Bronze/ });
    expect(bronzeCard).not.toHaveTextContent(/1,000|requests|Unlimited/);
    expect(bronzeCard.querySelectorAll('.MuiChip-root')).toHaveLength(0);
  });

  describe('a REST API: the plans attached in Develop › Usage Plans', () => {
    it('offers only the attached plans, and no note on policies', async () => {
      server.use(collection(PLANS_PATH, [bronze, gold, silver]));

      renderTab({ attachedPlanIds: ['bronze', 'silver'], values: values({ subscriptionPlanIds: ['bronze'] }) });

      await screen.findByText('Bronze');
      expect(cardOrder(['Bronze', 'Gold', 'Silver'])).toEqual(['Bronze', 'Silver']);
      expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('keeps a selected plan detached since, ticked, so it can be unticked', async () => {
      server.use(collection(PLANS_PATH, [bronze, gold]));

      renderTab({ attachedPlanIds: ['bronze'], values: values({ subscriptionPlanIds: ['gold'] }) });

      await screen.findByText('Gold');
      expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveAttribute('aria-checked', 'true');
      expect(screen.getByRole('checkbox', { name: /Bronze/ })).toHaveAttribute('aria-checked', 'false');
    });

    it('keeps a saved plan detached since listed after it is unticked, so it can be ticked again', async () => {
      server.use(collection(PLANS_PATH, [bronze, gold]));

      renderTab({
        attachedPlanIds: ['bronze'],
        savedPlanIds: ['gold'],
        values: values({ subscriptionPlanIds: [] }),
      });

      await screen.findByText('Gold');
      expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveAttribute('aria-checked', 'false');
    });

    it('points to Usage Plans when no plan is attached', async () => {
      server.use(collection(PLANS_PATH, [bronze, gold]));

      renderTab({ attachedPlanIds: [], usagePlansPath: '/o/p/apis/a/develop/usage-plans' });

      expect(await screen.findByText('No usage plans attached')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Attach plans in Develop › Usage Plans' })).toHaveAttribute(
        'href',
        '/o/p/apis/a/develop/usage-plans',
      );
      expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    });
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

  it('offers no inactive plan, even a selected one, and shows no chip, tooltip or warning for it', async () => {
    server.use(collection(PLANS_PATH, plans));

    const { user } = renderTab({ values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }) });

    await screen.findByText('Bronze');
    expect(screen.queryByRole('checkbox', { name: /Legacy/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/inactive/i)).not.toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();

    await user.hover(screen.getByRole('checkbox', { name: /Bronze/ }));
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('Select all selects every active plan; Clear all clears everything', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { rerender, user } = renderTab({ onChange, values: values() });

    await screen.findByText('Bronze');
    await user.click(screen.getByRole('button', { name: 'Select all' }));

    const selectedIds: string[] = onChange.mock.calls[0][0].subscriptionPlanIds;
    expect([...selectedIds].sort()).toEqual(['bronze', 'gold']);

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

  it('read-only: shows exactly the plans the live listing holds, an inactive one tagged, and nothing can be changed', async () => {
    server.use(collection(PLANS_PATH, plans));
    const onChange = vi.fn();

    const { user } = renderTab({
      onChange,
      readOnly: true,
      values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }),
    });

    await screen.findByText('Bronze');
    expect(screen.queryByRole('checkbox', { name: /Gold/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /select all|clear all/i })).not.toBeInTheDocument();
    expect(summaryText('2 plans published')).toBeInTheDocument();
    expect(screen.queryByText(/deleted plan.* selected/)).not.toBeInTheDocument();
    expect(screen.getByText('Inactive')).toBeInTheDocument();

    for (const name of [/Bronze/, /Legacy/]) {
      const card = screen.getByRole('checkbox', { name });
      expect(card).toHaveAttribute('aria-checked', 'true');
      expect(card).toHaveAttribute('aria-disabled', 'true');
      expect(card).toHaveAttribute('tabindex', '-1');
      await user.click(card);
    }
    expect(onChange).not.toHaveBeenCalled();
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

  it('draft: shows no deleted plan, whatever it is given', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold]));

    renderTab({
      deletedPlans: [{ id: 'old', displayName: 'Platinum' }],
      values: values({ subscriptionPlanIds: ['bronze'] }),
    });

    await screen.findByText('Bronze');
    expect(screen.queryByText('Platinum')).not.toBeInTheDocument();
    expect(summaryText('1 of 2 plans selected')).toBeInTheDocument();
    expect(screen.queryByText(/deleted/i)).not.toBeInTheDocument();
  });

  it('read-only: shows the names stored at the last publish, not the current ones', async () => {
    server.use(collection(PLANS_PATH, [bronze]));

    renderTab({
      readOnly: true,
      storedNames: { bronze: 'Bronze (old name)' },
      values: values({ subscriptionPlanIds: ['bronze'] }),
    });

    expect(await screen.findByRole('checkbox', { name: /Bronze \(old name\)/ })).toBeInTheDocument();
  });

  it('read-only: counts a deleted plan among the published plans, struck through, with no chip or limit, and a since-last-publish line', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold]));

    renderTab({
      deletedPlans: [{ id: 'old', displayName: 'Platinum' }],
      readOnly: true,
      values: values({ subscriptionPlanIds: ['bronze'] }),
    });

    const card = await screen.findByRole('checkbox', { name: /Platinum/ });
    expect(card.querySelector('del')).toHaveTextContent('Platinum');
    expect(card.querySelectorAll('.MuiChip-root')).toHaveLength(0);
    expect(card).not.toHaveTextContent('For trying the API out.');
    expect(card).toHaveTextContent('Deleted since last publish');
    expect(cardOrder(['Platinum', 'Bronze'])[0]).toBe('Platinum');
    expect(summaryText('2 plans published')).toBeInTheDocument();
  });

  it('read-only: a listing holding only a deleted plan is not shown as empty', async () => {
    server.use(collection(PLANS_PATH, [bronze]));

    renderTab({ deletedPlans: [{ id: 'old', displayName: 'Platinum' }], readOnly: true, values: values() });

    expect(await screen.findByRole('checkbox', { name: /Platinum/ })).toBeInTheDocument();
    expect(screen.queryByText('This listing has no subscription plans.')).not.toBeInTheDocument();
  });

  it('tags a plan edited since it was published in the published view only, with a since-last-publish line', async () => {
    server.use(collection(PLANS_PATH, [bronze, gold]));

    const { rerender } = renderTab({ editedPlanIds: ['bronze'], values: values({ subscriptionPlanIds: ['bronze', 'gold'] }) });

    const draftCard = await screen.findByRole('checkbox', { name: /Bronze/ });
    expect(draftCard).not.toHaveTextContent('Edited');
    expect(draftCard).toHaveTextContent('For trying the API out.');

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <SubscriptionPlansTab editedPlanIds={['bronze']} readOnly values={values({ subscriptionPlanIds: ['bronze', 'gold'] })} />
      </ApiScopeProvider>,
    );
    const published = await screen.findByRole('checkbox', { name: /Bronze/ });
    const chip = published.querySelector('.MuiChip-root');
    expect(chip).toHaveTextContent('Edited');
    expect(chip).toHaveClass('MuiChip-outlined');
    expect(chip?.querySelector('svg')).toBeNull();
    expect(published).toHaveTextContent('Edited since last publish');
    expect(published).not.toHaveTextContent('For trying the API out.');
    expect(screen.getByRole('checkbox', { name: /Gold/ })).not.toHaveTextContent('Edited');
    expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveTextContent('For production traffic.');
  });

  it('draft: shows no chip on any card', async () => {
    server.use(collection(PLANS_PATH, [bronze, legacy]));

    renderTab({
      editedPlanIds: ['legacy', 'bronze'],
      values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }),
    });

    const card = await screen.findByRole('checkbox', { name: /Bronze/ });
    expect(card.querySelectorAll('.MuiChip-root')).toHaveLength(0);
    expect(document.querySelectorAll('.MuiChip-root')).toHaveLength(0);
  });

  it('explains a deleted, edited or inactive plan in a tooltip in the published view', async () => {
    server.use(collection(PLANS_PATH, [bronze, legacy]));

    const { user } = renderTab({
      deletedPlans: [{ id: 'old', displayName: 'Platinum' }],
      editedPlanIds: ['bronze'],
      readOnly: true,
      values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }),
    });

    await user.hover(await screen.findByRole('checkbox', { name: /Platinum/ }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('This plan was deleted. Republish to remove it from the listing.');
    await user.unhover(screen.getByRole('checkbox', { name: /Platinum/ }));

    await user.hover(screen.getByRole('checkbox', { name: /Bronze/ }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Plan was edited after publishing. Republish to apply changes.');
    await user.unhover(screen.getByRole('checkbox', { name: /Bronze/ }));

    await user.hover(screen.getByRole('checkbox', { name: /Legacy/ }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Plan is inactive. Republish to remove it from the listing.');
  });

  it('read-only: shows one outlined chip per card, Inactive over Edited, and none on a deleted plan', async () => {
    server.use(collection(PLANS_PATH, [bronze, legacy]));

    renderTab({
      deletedPlans: [{ id: 'old', displayName: 'Platinum' }],
      editedPlanIds: ['legacy', 'bronze'],
      readOnly: true,
      values: values({ subscriptionPlanIds: ['bronze', 'legacy'] }),
    });

    const inactiveEdited = await screen.findByRole('checkbox', { name: /Legacy/ });
    expect(inactiveEdited).toHaveTextContent('Inactive');
    expect(inactiveEdited.querySelectorAll('.MuiChip-root')).toHaveLength(1);
    expect(inactiveEdited).not.toHaveTextContent('For');
    expect(inactiveEdited).toHaveTextContent('Edited since last publish');
    expect(screen.getByRole('checkbox', { name: /Bronze/ }).querySelectorAll('.MuiChip-root')).toHaveLength(1);
    expect(screen.getByRole('checkbox', { name: /Bronze/ })).toHaveTextContent('Edited');
    expect(screen.getByRole('checkbox', { name: /Platinum/ }).querySelectorAll('.MuiChip-root')).toHaveLength(0);
    expect(cardOrder(['Platinum', 'Legacy', 'Bronze'])).toEqual(['Platinum', 'Legacy', 'Bronze']);
  });
});
