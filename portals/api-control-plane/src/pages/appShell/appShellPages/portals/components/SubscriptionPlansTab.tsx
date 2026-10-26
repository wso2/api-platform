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

import { Box, Button, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useMetersById } from '@/api/resources/meters';
import { useAllSubscriptionPlans, type SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { pricingSummary, summaryMeterIds } from '../../settings/components/pricingFormat';
import { getPlanLimitDisplay } from '../utils/subscriptionPlanLimit';
import type { DeletedSelection, DraftFormValues } from '../utils/publicationForm';
import { PlanCard } from './PlanCard';

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
  inactiveSelectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.inactiveSelectedSummary',
    defaultMessage: '{count, plural, one {# inactive plan selected} other {# inactive plans selected}}',
    description: 'Warning next to the selection summary: selected plans that are no longer active.',
  },
  deletedSelectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.deletedSelectedSummary',
    defaultMessage: '{count, plural, one {# deleted plan selected} other {# deleted plans selected}}',
    description: 'Warning next to the selection summary: selected plans whose subscription plan no longer exists.',
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
  /** Ids in `deletedPlans` the user cleared. Their cards stay, unticked, until the draft is saved. */
  clearedDeletedIds?: readonly string[];
  /** Selected plans whose subscription plan was deleted. They show as "Deleted" cards and count as published. */
  deletedPlans?: readonly DeletedSelection[];
  disabled?: boolean;
  /** Ids of selected plans changed in Settings since the listing was published; the published view tags them "Edited". */
  editedPlanIds?: readonly string[];
  /** Clears a deleted plan from the draft, or puts it back if already cleared. */
  onToggleDeleted?: (planId: string) => void;
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
 * Status limits what can be *chosen*, never what is shown of the selection: the
 * draft offers the active plans, and an inactive one only while the saved selection
 * holds it, so it can be cleared. A cleared inactive or deleted plan stays on screen,
 * unticked, until the draft is saved, and can be put back. The published view lists
 * exactly the plans the live listing holds, under the names stored at the last publish.
 *
 * Cards are ordered deleted plans first (they are gone from Settings and only
 * linger until the next save or publish), then inactive plans (they need
 * clearing), then the saved selection, then the rest; each group keeps the
 * server's order.
 */
export function SubscriptionPlansTab({
  clearedDeletedIds = [],
  deletedPlans = [],
  disabled,
  editedPlanIds = [],
  onChange,
  onToggleDeleted,
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
  const clearedSet = new Set(clearedDeletedIds);
  const savedSet = new Set(savedPlanIds ?? selectedIds);
  const allPlans = (plansQuery.data?.list ?? []).filter((plan): plan is DisplayablePlan => Boolean(plan.id));
  const activePlans = allPlans.filter((plan) => plan.status === 'ACTIVE');
  // An inactive plan is offered only while the saved selection holds it, so it can still be cleared.
  const inactiveShownPlans = allPlans.filter(
    (plan) => plan.status !== 'ACTIVE' && (selectedSet.has(plan.id) || savedSet.has(plan.id)),
  );
  // A deleted plan the user cleared is no longer selected, though its card stays until the draft is saved.
  const deletedSelectedCount = deletedPlans.filter((plan) => !clearedSet.has(plan.id)).length;
  const inactiveSelectedCount = inactiveShownPlans.filter((plan) => selectedSet.has(plan.id)).length;
  const rank = (plan: DisplayablePlan) => {
    if (plan.status !== 'ACTIVE') return 0;
    return savedSet.has(plan.id) ? 1 : 2;
  };
  const offeredPlans = [...activePlans, ...inactiveShownPlans].sort((a, b) => rank(a) - rank(b));
  const shownPlans = readOnly ? offeredPlans.filter((plan) => selectedSet.has(plan.id)) : offeredPlans;
  // The unit a per-unit price counts (call or token) comes from its meter.
  const { meters } = useMetersById(shownPlans.flatMap((plan) => summaryMeterIds(plan.pricing)));

  if (plansQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (plansQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (shownPlans.length === 0 && deletedPlans.length === 0) {
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

  const toggle = (planId: string) => {
    const next = new Set(selectedIds);
    if (next.has(planId)) next.delete(planId);
    else next.add(planId);
    commitSelection(Array.from(next));
  };

  // Select all keeps an already-selected inactive plan; Clear all clears everything.
  const toggleAll = () =>
    commitSelection(
      allActiveSelected
        ? []
        : [...activePlans, ...inactiveShownPlans.filter((plan) => selectedSet.has(plan.id))].map((plan) => plan.id),
    );

  const countStyle = {
    color: readOnly ? 'success.main' : 'primary.main',
    fontSize: 20,
    fontWeight: 500,
  } as const;

  return (
    <Box component="section" sx={{ containerType: 'inline-size', display: 'flex', flexDirection: 'column', gap: 2.5, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <Box sx={{ alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: 1.5 }}>
          <Typography color="text.secondary" variant="body2">
            {readOnly ? (
              <FormattedMessage
                {...messages.publishedSummary}
                values={{
                  count: shownPlans.length + deletedPlans.length,
                  n: (chunks) => (
                    <Typography component="span" sx={countStyle}>
                      {chunks}
                    </Typography>
                  ),
                }}
              />
            ) : (
              <FormattedMessage
                {...messages.selectedSummary}
                values={{
                  count: (
                    <Typography component="span" sx={countStyle}>
                      {selectedActiveCount}
                    </Typography>
                  ),
                  total: activePlans.length,
                }}
              />
            )}
          </Typography>
          {!readOnly && deletedSelectedCount > 0 && (
            <Typography color="error.main" variant="body2">
              <FormattedMessage {...messages.deletedSelectedSummary} values={{ count: deletedSelectedCount }} />
            </Typography>
          )}
          {!readOnly && inactiveSelectedCount > 0 && (
            <Typography color="warning.main" variant="body2">
              <FormattedMessage
                {...messages.inactiveSelectedSummary}
                values={{ count: inactiveSelectedCount }}
              />
            </Typography>
          )}
        </Box>
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
        {deletedPlans.map((plan) => (
          <PlanCard
            cleared={clearedSet.has(plan.id)}
            deleted
            disabled={disabled}
            displayName={plan.displayName}
            key={plan.id}
            onToggle={() => onToggleDeleted?.(plan.id)}
            readOnly={readOnly}
            selected={!clearedSet.has(plan.id)}
          />
        ))}
        {shownPlans.map((plan) => (
          <PlanCard
            cleared={!readOnly && plan.status !== 'ACTIVE' && !selectedSet.has(plan.id)}
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
