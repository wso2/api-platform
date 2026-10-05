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

import { useAllSubscriptionPlans, type SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { getPlanLimitDisplay } from '../utils/subscriptionPlanLimit';
import type { DraftFormValues } from '../utils/publicationForm';
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
  inactiveBlockedToast: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.inactiveBlockedToast',
    defaultMessage: '"{name}" is inactive. Activate it in Settings.',
    description: 'Warning shown when the user tries to select an inactive plan. {name} is the plan\'s display name.',
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
  disabled?: boolean;
  /** Shows the live selection without letting it be changed; `onChange` is then never called. */
  readOnly?: boolean;
  onChange?: (values: DraftFormValues) => void;
  values: DraftFormValues;
};

/** A plan this tab can actually render — `id` is optional on the generated type but always present in practice. */
type DisplayablePlan = SubscriptionPlan & { id: string };

/**
 * "Subscription Plans": which of the organization's plans this API offers on the
 * portal. A plan's active/inactive status is set in Settings, not here.
 *
 * Status limits what can be *chosen*, never what is shown of the selection: the
 * draft lists every plan (active first), an inactive one refuses to be selected,
 * but one already selected stays visible and can be cleared. The published view
 * lists exactly the plans the live listing holds.
 */
export function SubscriptionPlansTab({ disabled, onChange, readOnly, values }: SubscriptionPlansTabProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const plansQuery = useAllSubscriptionPlans();
  const selectedIds = values.subscriptionPlanIds;
  const selectedSet = new Set(selectedIds);
  const allPlans = (plansQuery.data?.list ?? []).filter((plan): plan is DisplayablePlan => Boolean(plan.id));
  const activePlans = allPlans.filter((plan) => plan.status === 'ACTIVE');
  const inactivePlans = allPlans.filter((plan) => plan.status !== 'ACTIVE');
  const inactiveSelectedPlans = inactivePlans.filter((plan) => selectedSet.has(plan.id));
  const orderedPlans = [...activePlans, ...inactivePlans];
  const shownPlans = readOnly ? orderedPlans.filter((plan) => selectedSet.has(plan.id)) : orderedPlans;

  if (plansQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (plansQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (shownPlans.length === 0) {
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
    commitSelection(allActiveSelected ? [] : [...activePlans, ...inactiveSelectedPlans].map((plan) => plan.id));

  const warnInactive = (plan: DisplayablePlan) =>
    notify(intl.formatMessage(messages.inactiveBlockedToast, { name: plan.displayName }), 'warning');

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
                  count: shownPlans.length,
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
          {!readOnly && inactiveSelectedPlans.length > 0 && (
            <Typography color="warning.main" variant="body2">
              <FormattedMessage
                {...messages.inactiveSelectedSummary}
                values={{ count: inactiveSelectedPlans.length }}
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
          // Four per row, stepping down only when a card would drop below ~250px wide.
          gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
          '@container (max-width: 1059px)': { gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' },
          '@container (max-width: 809px)': { gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' },
          '@container (max-width: 519px)': { gridTemplateColumns: 'minmax(0, 1fr)' },
        }}
      >
        {shownPlans.map((plan) => (
          <PlanCard
            disabled={disabled}
            displayName={plan.displayName}
            inactive={plan.status !== 'ACTIVE'}
            key={plan.id}
            limit={getPlanLimitDisplay(plan)}
            onBlocked={() => warnInactive(plan)}
            onToggle={() => toggle(plan.id)}
            readOnly={readOnly}
            selected={selectedSet.has(plan.id)}
          />
        ))}
      </Box>
    </Box>
  );
}
