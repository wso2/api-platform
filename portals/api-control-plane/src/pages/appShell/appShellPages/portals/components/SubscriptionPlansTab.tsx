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

import { Box, Button } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useMetersById } from '@/api/resources/meters';
import { useAllSubscriptionPlans, type SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { pricingSummary, summaryMeterIds } from '../../settings/components/pricingFormat';
import { getPlanLimitDisplay } from '../utils/subscriptionPlanLimit';
import { toggleSelection, type DeletedSelection, type DraftFormValues } from '../utils/publicationForm';
import { PlanCard } from './PlanCard';
import { SelectionSummary } from './SelectionSummary';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.loading',
    defaultMessage: 'Loading subscription plans',
  },
  error: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.error',
    defaultMessage: 'Unable to load subscription plans.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.emptyTitle',
    defaultMessage: 'No subscription plans',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.emptyDescription',
    defaultMessage: 'Create subscription plans in Settings to choose from here.',
  },
  publishedEmptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.publishedEmptyTitle',
    defaultMessage: 'This listing has no subscription plans.',
  },
  selectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.selectedSummary',
    defaultMessage: '{count} of {total} plans selected',
    description: '{count} is a styled number rendered as a React element, not plain text; {total} is a plain number.',
  },
  publishedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.publishedSummary',
    defaultMessage: '{count, plural, one {<n>#</n> plan published} other {<n>#</n> plans published}}',
    description: '<n> wraps the number, which is styled.',
  },
  selectAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.selectAll',
    defaultMessage: 'Select all',
  },
  clearAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.clearAll',
    defaultMessage: 'Clear all',
  },
});

export type SubscriptionPlansTabProps = {
  /** Published view only: plans the listing holds whose subscription plan was deleted. They show struck through and count as published. */
  deletedPlans?: readonly DeletedSelection[];
  disabled?: boolean;
  /** Ids of selected plans changed in Settings since the listing was published; the published view tags them "Edited". */
  editedPlanIds?: readonly string[];
  /** The names stored at the last publish, by id. The published view shows these rather than the plan's current name. */
  storedNames?: Readonly<Record<string, string>>;
  /** Shows the live selection without letting it be changed; `onChange` is then never called. */
  readOnly?: boolean;
  onChange?: (values: DraftFormValues) => void;
  /** The selection as last saved (the draft's, or the live listing's). It orders the cards, so clicking one never moves it. Defaults to `values`. */
  savedPlanIds?: string[];
  values: DraftFormValues;
};

/** A plan this tab can actually render — `id` is optional on the generated type but always present in practice. */
type DisplayablePlan = SubscriptionPlan & { id: string };

/**
 * "Subscription Plans": which of the organization's plans this API offers on the
 * portal. A plan's active/inactive status is set in Settings, not here.
 *
 * The draft offers the active plans only. An inactive or deleted plan never shows there, with no
 * chip or warning: the page drops it from the selection, so the next save or publish leaves it out.
 * The published view lists exactly the plans the live listing holds, under the names stored at the
 * last publish, with deleted plans struck through and inactive or edited ones tagged by an outlined chip.
 */
export function SubscriptionPlansTab({
  deletedPlans = [],
  disabled,
  editedPlanIds = [],
  onChange,
  readOnly,
  savedPlanIds,
  storedNames,
  values,
}: SubscriptionPlansTabProps) {
  const intl = useIntl();
  const plansQuery = useAllSubscriptionPlans();
  const selectedIds = values.subscriptionPlanIds;
  const selectedSet = new Set(selectedIds);
  const editedSet = new Set(editedPlanIds);
  const savedSet = new Set(savedPlanIds ?? selectedIds);
  const allPlans = (plansQuery.data?.list ?? []).filter((plan): plan is DisplayablePlan => Boolean(plan.id));
  const activePlans = allPlans.filter((plan) => plan.status === 'ACTIVE');
  const shownDeleted = readOnly ? deletedPlans : [];
  const rank = (plan: DisplayablePlan) => {
    if (!readOnly) return savedSet.has(plan.id) ? 0 : 1;
    if (plan.status !== 'ACTIVE') return 0;
    return editedSet.has(plan.id) ? 1 : 2;
  };
  // The published view lists whatever the listing holds, active or not; the draft offers the active plans.
  const shownPlans = (readOnly ? allPlans.filter((plan) => selectedSet.has(plan.id)) : [...activePlans]).sort(
    (a, b) => rank(a) - rank(b),
  );
  // The unit a per-unit price counts (call or token) comes from its meter.
  const { meters } = useMetersById(shownPlans.flatMap((plan) => summaryMeterIds(plan.pricing)));

  if (plansQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (plansQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (shownPlans.length === 0 && shownDeleted.length === 0) {
    return (
      <Box component="section" sx={{ p: 3 }}>
        <EmptyState
          description={readOnly ? undefined : intl.formatMessage(messages.emptyDescription)}
          title={intl.formatMessage(readOnly ? messages.publishedEmptyTitle : messages.emptyTitle)}
        />
      </Box>
    );
  }

  const selectedActiveCount = activePlans.filter((plan) => selectedSet.has(plan.id)).length;
  const allActiveSelected = activePlans.length > 0 && selectedActiveCount === activePlans.length;

  const commitSelection = (nextIds: string[]) => {
    onChange?.({ ...values, subscriptionPlanIds: nextIds });
  };

  const toggle = (planId: string) => commitSelection(toggleSelection(selectedIds, planId));

  const toggleAll = () => commitSelection(allActiveSelected ? [] : activePlans.map((plan) => plan.id));

  return (
    <Box component="section" sx={{ containerType: 'inline-size', display: 'flex', flexDirection: 'column', gap: 2.5, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <SelectionSummary
          count={readOnly ? shownPlans.length + shownDeleted.length : selectedActiveCount}
          published={messages.publishedSummary}
          readOnly={readOnly}
          selected={messages.selectedSummary}
          total={activePlans.length}
        />
        {!readOnly && (
          <Button disabled={disabled || activePlans.length === 0} onClick={toggleAll} size="small" variant="text">
            <FormattedMessage {...(allActiveSelected ? messages.clearAll : messages.selectAll)} />
          </Button>
        )}
      </Box>

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridAutoRows: '1fr', // every row as tall as its tallest card
          // Four per row, stepping down only when a card would drop below ~250px wide.
          gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
          '@container (max-width: 1059px)': { gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' },
          '@container (max-width: 809px)': { gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' },
          '@container (max-width: 519px)': { gridTemplateColumns: 'minmax(0, 1fr)' },
        }}
      >
        {shownDeleted.map((plan) => (
          <PlanCard deleted displayName={plan.displayName} key={plan.id} readOnly selected />
        ))}
        {shownPlans.map((plan) => (
          <PlanCard
            disabled={disabled}
            displayName={(readOnly && storedNames?.[plan.id]) || plan.displayName}
            edited={editedSet.has(plan.id)}
            inactive={plan.status !== 'ACTIVE'}
            key={plan.id}
            limit={getPlanLimitDisplay(plan)}
            onToggle={() => toggle(plan.id)}
            pricing={plan.pricing ? pricingSummary(intl, plan.pricing, meters) : undefined}
            readOnly={readOnly}
            selected={selectedSet.has(plan.id)}
          />
        ))}
      </Box>
    </Box>
  );
}
