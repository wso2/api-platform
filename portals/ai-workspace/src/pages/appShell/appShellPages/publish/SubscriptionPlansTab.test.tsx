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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import type { SubscriptionPlan } from '../../../../utils/types';
import { emptyDraftFormValues, type DraftFormValues } from './mcpPublishForm';
import { SubscriptionPlansTab, type SubscriptionPlansTabProps } from './SubscriptionPlansTab';

vi.mock('../../../../Components/AILoader', () => ({ default: () => <div data-testid="loader" /> }));

const plan = (id: string, status: 'ACTIVE' | 'INACTIVE' = 'ACTIVE', limitCount = 100): SubscriptionPlan => ({
  id,
  displayName: id[0].toUpperCase() + id.slice(1),
  status,
  limits: [{ limitType: 'REQUEST_COUNT', limitCount, timeAmount: 1, timeUnit: 'MONTH' }],
});

const valuesWith = (...subscriptionPlanIds: string[]): DraftFormValues => ({ ...emptyDraftFormValues, subscriptionPlanIds });

const renderTab = (props: Partial<SubscriptionPlansTabProps> = {}) => {
  const onChange = vi.fn();
  const view = renderWithProviders(
    <SubscriptionPlansTab
      onChange={onChange}
      plans={[plan('gold'), plan('silver'), plan('bronze', 'INACTIVE')]}
      values={valuesWith()}
      {...props}
    />
  );
  return { ...view, onChange };
};

/** The plan names in the order their cards appear. */
const cardOrder = () => screen.getAllByRole('checkbox').map((card) => card.textContent?.match(/^(Gold|Silver|Bronze|Platinum|Free)/)?.[0]);

describe('SubscriptionPlansTab', () => {
  describe('before the plans are known', () => {
    it('shows a loader while they load', () => {
      renderTab({ plans: undefined });

      expect(screen.getByText('Loading subscription plans')).toBeInTheDocument();
    });

    it('says so when they could not be read', () => {
      renderTab({ failed: true, plans: undefined });

      expect(screen.getByRole('alert')).toHaveTextContent('Unable to load subscription plans.');
    });
  });

  describe('editing', () => {
    it('offers the active plans only, hiding an inactive one nobody selected', () => {
      renderTab();

      expect(cardOrder()).toEqual(['Gold', 'Silver']);
    });

    it('counts the selected active plans against all active ones', () => {
      renderTab({ values: valuesWith('gold') });

      expect(screen.getByText(/of 2 plans selected/)).toHaveTextContent('1 of 2 plans selected');
    });

    it('selects a plan by its card, keeping what was already selected', async () => {
      const { onChange, user } = renderTab({ values: valuesWith('gold') });

      await user.click(screen.getByRole('checkbox', { name: /Silver/ }));

      expect(onChange).toHaveBeenCalledWith({ ...emptyDraftFormValues, subscriptionPlanIds: ['gold', 'silver'] });
    });

    it('clears a selected plan the same way', async () => {
      const { onChange, user } = renderTab({ values: valuesWith('gold', 'silver') });

      await user.click(screen.getByRole('checkbox', { name: /Gold/ }));

      expect(onChange).toHaveBeenCalledWith({ ...emptyDraftFormValues, subscriptionPlanIds: ['silver'] });
    });

    it('keeps the rest of the form as it was when the selection changes', async () => {
      const values = { ...valuesWith(), displayName: 'Travel MCP', docIds: ['guide'] };
      const { onChange, user } = renderTab({ values });

      await user.click(screen.getByRole('checkbox', { name: /Gold/ }));

      expect(onChange).toHaveBeenCalledWith({ ...values, subscriptionPlanIds: ['gold'] });
    });

    it('selects every active plan at once, then clears them all', async () => {
      const { onChange, rerender, user } = renderTab();

      await user.click(screen.getByRole('button', { name: 'Select all' }));
      expect(onChange).toHaveBeenLastCalledWith({ ...emptyDraftFormValues, subscriptionPlanIds: ['gold', 'silver'] });

      rerender(
        <SubscriptionPlansTab
          onChange={onChange}
          plans={[plan('gold'), plan('silver')]}
          values={valuesWith('gold', 'silver')}
        />
      );
      await user.click(screen.getByRole('button', { name: 'Clear all' }));
      expect(onChange).toHaveBeenLastCalledWith({ ...emptyDraftFormValues, subscriptionPlanIds: [] });
    });

    it('turns every card and Select all off while an action runs', async () => {
      const { onChange, user } = renderTab({ disabled: true });

      await user.click(screen.getByRole('checkbox', { name: /Gold/ }));

      expect(onChange).not.toHaveBeenCalled();
      expect(screen.getByRole('button', { name: 'Select all' })).toBeDisabled();
    });

    it('offers no inactive plan, so an all-inactive organization shows the empty state', () => {
      renderTab({ plans: [plan('bronze', 'INACTIVE')], values: valuesWith('bronze') });

      expect(screen.getByRole('heading', { name: 'No subscription plans' })).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Select all|Clear all/ })).not.toBeInTheDocument();
    });
  });

  describe('an inactive plan that is selected', () => {
    it('is not offered, and leaves no chip, count or warning behind', () => {
      renderTab({ values: valuesWith('bronze', 'silver') });

      expect(cardOrder()).toEqual(['Silver', 'Gold']);
      expect(screen.queryByText('Inactive')).not.toBeInTheDocument();
      expect(screen.queryByText(/inactive plan/)).not.toBeInTheDocument();
      expect(screen.getByText(/of 2 plans selected/)).toHaveTextContent('1 of 2 plans selected');
    });

    it('is not added back by Select all', async () => {
      const { onChange, user } = renderTab({ values: valuesWith('bronze') });

      await user.click(screen.getByRole('button', { name: 'Select all' }));

      expect(onChange).toHaveBeenCalledWith({ ...emptyDraftFormValues, subscriptionPlanIds: ['gold', 'silver'] });
    });
  });

  describe('card order', () => {
    it('puts the saved selection ahead of the rest, and does not move a card that was just clicked', async () => {
      const { user } = renderTab({
        plans: [plan('gold'), plan('silver'), plan('platinum')],
        savedPlanIds: ['silver'],
        values: valuesWith('silver'),
      });
      expect(cardOrder()).toEqual(['Silver', 'Gold', 'Platinum']);

      await user.click(screen.getByRole('checkbox', { name: /Platinum/ }));

      expect(cardOrder()).toEqual(['Silver', 'Gold', 'Platinum']);
    });
  });

  describe('plan limits', () => {
    it('shows each plan’s limit, and unlimited for a plan without one', () => {
      renderTab({
        plans: [plan('gold', 'ACTIVE', 1500), { id: 'free', displayName: 'Free', status: 'ACTIVE' }],
      });

      expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveTextContent('1,500');
      expect(screen.getByRole('checkbox', { name: /Free/ })).toHaveTextContent('Unlimited');
    });
  });

  describe('when empty', () => {
    it('invites the user to create plans when there are none to choose from', () => {
      renderTab({ plans: [] });

      expect(screen.getByRole('heading', { name: 'No subscription plans' })).toBeInTheDocument();
      expect(screen.getByText('Create subscription plans to choose from here.')).toBeInTheDocument();
    });
  });

  describe('read-only, for the published version', () => {
    it('lists exactly the plans the listing holds, active or not, an inactive one first and tagged', () => {
      renderTab({ readOnly: true, values: valuesWith('silver', 'bronze') });

      expect(cardOrder()).toEqual(['Bronze', 'Silver']);
      expect(screen.getByText('Inactive')).toBeInTheDocument();
    });

    it('shows the names stored at the last publish, not the current ones', () => {
      renderTab({ readOnly: true, storedNames: { silver: 'Silver (old)' }, values: valuesWith('silver') });

      expect(screen.getByRole('checkbox', { name: /Silver \(old\)/ })).toBeInTheDocument();
    });

    it('counts a deleted plan among the published plans, struck through with a since-last-publish line', () => {
      renderTab({
        deletedPlans: [{ id: 'platinum', displayName: 'Platinum' }],
        readOnly: true,
        values: valuesWith('silver'),
      });

      expect(screen.getByText(/plans published/)).toHaveTextContent('2 plans published');
      const card = screen.getByRole('checkbox', { name: /Platinum/ });
      expect(card.querySelector('del')).toHaveTextContent('Platinum');
      expect(card).toHaveTextContent('Deleted since last publish');
      expect(cardOrder()[0]).toBe('Platinum');
    });

    it('is not shown as empty when the listing holds only a deleted plan', () => {
      renderTab({ deletedPlans: [{ id: 'platinum', displayName: 'Platinum' }], readOnly: true, values: valuesWith() });

      expect(screen.queryByRole('heading', { name: 'This listing has no subscription plans.' })).not.toBeInTheDocument();
      expect(screen.getByRole('checkbox', { name: /Platinum/ })).toBeInTheDocument();
    });

    it('tags a plan edited since the last publish, after the inactive ones and before the rest', () => {
      renderTab({ editedPlanIds: ['silver'], readOnly: true, values: valuesWith('gold', 'silver') });

      expect(cardOrder()).toEqual(['Silver', 'Gold']);
      expect(screen.getByRole('checkbox', { name: /Silver/ })).toHaveTextContent('Edited since last publish');
    });

    it('shows no deleted plan in the draft, whatever it is given', () => {
      renderTab({ deletedPlans: [{ id: 'platinum', displayName: 'Platinum' }], values: valuesWith('gold') });

      expect(screen.queryByRole('checkbox', { name: /Platinum/ })).not.toBeInTheDocument();
    });

    it('counts them as published', () => {
      renderTab({ readOnly: true, values: valuesWith('silver', 'gold') });

      expect(screen.getByText(/plans published/)).toHaveTextContent('2 plans published');
    });

    it('cannot be changed, and has no Select all', () => {
      const { onChange } = renderTab({ readOnly: true, values: valuesWith('silver') });

      fireEvent.click(screen.getByRole('checkbox', { name: /Silver/ }));

      expect(onChange).not.toHaveBeenCalled();
      expect(screen.queryByRole('button', { name: /Select all|Clear all/ })).not.toBeInTheDocument();
    });

    it('says so when the listing holds none', () => {
      renderTab({ readOnly: true, values: valuesWith() });

      expect(screen.getByRole('heading', { name: 'This listing has no subscription plans.' })).toBeInTheDocument();
    });
  });
});
