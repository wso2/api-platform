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

import { useMemo } from 'react';
import { Box, Button, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useActiveSubscriptionPlans, type SubscriptionPlan } from '@/api/resources/subscriptionPlans';
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
    defaultMessage: 'No active subscription plans',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.emptyDescription',
    defaultMessage: 'Create a subscription plan for your organization before offering it on this portal.',
  },
  selectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.selectedSummary',
    defaultMessage: '{count} of {total} plans selected',
    description: '{count} is a styled number rendered as a React element, not plain text; {total} is a plain number.',
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

/**
 * "Subscription Plans": which of the organization's active plans this API
 * offers to consumers on the portal. A plan's own active/inactive status is
 * set on the Subscription Plans settings page, not here — this tab only
 * controls which of the already-active plans are offered for this one API.
 */
/** A plan this tab can actually render — `id` is optional on the generated type but always present in practice. */
type DisplayablePlan = SubscriptionPlan & { id: string };

export function SubscriptionPlansTab({ disabled, onChange, readOnly, values }: SubscriptionPlansTabProps) {
  const intl = useIntl();
  const plansQuery = useActiveSubscriptionPlans();
  const activePlans = useMemo(
    () => (plansQuery.data ?? []).filter((plan): plan is DisplayablePlan => Boolean(plan.id)),
    [plansQuery.data],
  );
  const selectedIds = values.subscriptionPlanIds;
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  if (plansQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (plansQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.error)} />;
  }
  if (activePlans.length === 0) {
    return (
      <Box component="section" sx={{ p: 3 }}>
        <EmptyState
          description={intl.formatMessage(messages.emptyDescription)}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      </Box>
    );
  }

  const selectedCount = activePlans.filter((plan) => selectedSet.has(plan.id)).length;
  const availableCount = activePlans.length;
  const allSelected = selectedCount === availableCount;

  const commitSelection = (nextIds: string[]) => {
    onChange?.({ ...values, subscriptionPlanIds: nextIds });
  };

  const toggle = (planId: string) => {
    const next = new Set(selectedIds);
    if (next.has(planId)) next.delete(planId);
    else next.add(planId);
    commitSelection(Array.from(next));
  };

  const toggleAll = () => {
    commitSelection(allSelected ? [] : activePlans.map((plan) => plan.id));
  };

  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 2.5, p: 3 }}>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 2, justifyContent: 'space-between', minHeight: 32 }}>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage
            {...messages.selectedSummary}
            values={{
              count: (
                <Typography
                  component="span"
                  sx={{ color: readOnly ? 'success.main' : 'primary.main', fontSize: 20, fontWeight: 500 }}
                >
                  {selectedCount}
                </Typography>
              ),
              total: availableCount,
            }}
          />
        </Typography>
        {!readOnly && (
          <Button disabled={disabled} onClick={toggleAll} size="small" variant="text">
            <FormattedMessage {...(allSelected ? messages.clearAll : messages.selectAll)} />
          </Button>
        )}
      </Box>

      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))' }}>
        {activePlans.map((plan) => (
          <PlanCard
            disabled={disabled}
            displayName={plan.displayName}
            key={plan.id}
            limit={getPlanLimitDisplay(plan)}
            onToggle={() => toggle(plan.id)}
            readOnly={readOnly}
            selected={selectedSet.has(plan.id)}
          />
        ))}
      </Box>
    </Box>
  );
}
