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

import { Alert, Box, Card, CardContent, Chip, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import {
  CircleCheck,
  CircleX,
  ListChecks,
  Minus,
  TriangleAlert,
} from '@wso2/oxygen-ui-icons-react';
import type { ReactElement } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import {
  REST_API_TYPE,
  useApiPublishedPlanIds,
  type PublishableApiType,
} from '@/api/resources/apiPublications';
import { useAllSubscriptionPlans } from '@/api/resources/subscriptionPlans';
import { SecondaryText } from '@/components/common/SecondaryText';
import { iconTileSx } from '@/theme/receipes';
import { conditionTypeName } from '../../orgManage/components/PlanConditionsEditor';
import {
  planEnforcement,
  SUBSCRIPTION_VALIDATION,
  type EnforcementRow,
  type EnforcementStatus,
} from './planPolicies';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.title',
    defaultMessage: 'Required by subscription plans',
    description: 'Heading of the notes on which policies the plans this API offers need.',
  },
  summary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.summary',
    defaultMessage: '{enforced, number} of {total, number} enforced',
    description: 'How many of the plan conditions listed below a policy on this API enforces.',
  },
  validationMissing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.validationMissing',
    defaultMessage:
      'Attach {policy} first. Without it no plan, condition or subscription is checked.',
    description: '{policy} is a policy name (subscription-validation); do not translate it.',
  },
  statusEnforced: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.statusEnforced',
    defaultMessage: 'Enforced',
  },
  statusNotEnforced: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.statusNotEnforced',
    defaultMessage: 'Not enforced',
  },
  statusNoPolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.statusNoPolicy',
    defaultMessage: 'No policy yet',
    description: 'Status of a plan condition that no gateway policy can enforce today.',
  },
  noteEnforced: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noteEnforced',
    defaultMessage: 'By {policy}',
    description: '{policy} is the name of the policy that enforces the condition.',
  },
  noteNotEnforced: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noteNotEnforced',
    defaultMessage: 'Not enforced. Attach {policy} and redeploy.',
    description: '{policy} is the name of the policy that would enforce the condition.',
  },
  noteNoPolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noteNoPolicy',
    defaultMessage: 'No policy enforces this yet',
  },
  noteNoReader: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noteNoReader',
    defaultMessage: 'No policy reads it yet',
    description:
      'Note on a custom plan condition, which only a policy written to read its ID uses.',
  },
  noPlans: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noPlans',
    defaultMessage:
      'This API offers no subscription plans. Plans you pick when you publish it to an API Portal show here, with the policies they need.',
  },
  noConditions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.noConditions',
    defaultMessage: 'The plans this API offers have no conditions: they are unlimited.',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.loadError',
    defaultMessage: 'Unable to load the subscription plans this API offers.',
  },
  hint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PlanPolicyNotes.hint',
    defaultMessage:
      'For the plans this API offers. A plan condition applies only where a policy enforces it. You pick the plans when you publish to the API Portal.',
  },
});

const STATUS: Record<
  EnforcementStatus,
  { color: 'success' | 'warning' | 'default'; icon: ReactElement; label: MessageDescriptor }
> = {
  enforced: { color: 'success', icon: <CircleCheck size={14} />, label: messages.statusEnforced },
  noPolicy: { color: 'default', icon: <Minus size={14} />, label: messages.statusNoPolicy },
  notEnforced: {
    color: 'warning',
    icon: <TriangleAlert size={14} />,
    label: messages.statusNotEnforced,
  },
};

const noteOf = (row: EnforcementRow): MessageDescriptor => {
  if (row.status === 'enforced') return messages.noteEnforced;
  if (row.status === 'notEnforced') return messages.noteNotEnforced;
  return row.type === 'custom' ? messages.noteNoReader : messages.noteNoPolicy;
};

/** One condition: its name and ID, whether it is enforced, and why. */
function NoteRow({ row }: { row: EnforcementRow }) {
  const intl = useIntl();
  const status = STATUS[row.status];
  return (
    <Box
      component="li"
      sx={{
        alignItems: 'center',
        borderBottom: 1,
        borderColor: 'divider',
        columnGap: 1.5,
        display: 'grid',
        gridTemplateColumns: { sm: 'minmax(0, 1fr) auto', xs: 'minmax(0, 1fr)' },
        py: 1.25,
        rowGap: 0.5,
        '&:last-of-type': { borderBottom: 0 },
      }}
    >
      <Stack alignItems="center" direction="row" spacing={1} sx={{ minWidth: 0 }}>
        <Typography noWrap sx={{ fontWeight: 600 }} variant="body2">
          {conditionTypeName(intl, row.type)}
        </Typography>
        <Chip
          label={row.id}
          size="small"
          sx={{ '& .MuiChip-label': { fontFamily: 'monospace' }, maxWidth: '100%' }}
          variant="outlined"
        />
      </Stack>
      <Chip
        color={status.color}
        icon={status.icon}
        label={<FormattedMessage {...status.label} />}
        size="small"
        sx={{ justifySelf: { sm: 'end', xs: 'start' } }}
      />
      <SecondaryText sx={{ gridColumn: '1 / -1' }} variant="body2">
        <FormattedMessage {...noteOf(row)} values={{ policy: row.policy }} />
      </SecondaryText>
    </Box>
  );
}

/**
 * "Required by subscription plans": what the plans this API offers need from its policies, as
 * notes only — attaching a policy is done in the panels above. The plans are those the API's live
 * listings hold on any API Portal, since plans are chosen when the API is published. `attached`
 * is every policy on the API, global or on a resource, as currently edited.
 */
export function PlanPolicyNotes({
  apiHandle,
  apiType = REST_API_TYPE,
  attached,
}: {
  apiHandle: string | undefined;
  /** The API's `{apiType}` on the publication endpoints. */
  apiType?: PublishableApiType;
  attached: ReadonlySet<string>;
}) {
  const published = useApiPublishedPlanIds(apiType, apiHandle);
  const plansQuery = useAllSubscriptionPlans();

  const isPending = published.isPending || plansQuery.isPending;
  const error = published.error ?? plansQuery.error;
  const planIds = new Set(published.planIds);
  const plans = (plansQuery.data?.list ?? []).filter((plan) => plan.id && planIds.has(plan.id));
  const enforcement = planEnforcement(plans, attached);
  const { rows } = enforcement;
  const ready = !isPending && !error;
  const anyNotEnforced = rows.some((row) => row.status === 'notEnforced');

  let body;
  if (isPending) {
    body = <Skeleton height={48} variant="rounded" />;
  } else if (error) {
    body = (
      <Typography color="error" variant="body2">
        <FormattedMessage {...messages.loadError} />
      </Typography>
    );
  } else if (plans.length === 0) {
    body = (
      <SecondaryText variant="body2">
        <FormattedMessage {...messages.noPlans} />
      </SecondaryText>
    );
  } else {
    body = (
      <>
        {enforcement.validationMissing && (
          <Alert icon={<CircleX size={18} />} severity="error">
            <FormattedMessage
              {...messages.validationMissing}
              values={{
                policy: (
                  <Box component="code" sx={{ fontFamily: 'monospace' }}>
                    {SUBSCRIPTION_VALIDATION}
                  </Box>
                ),
              }}
            />
          </Alert>
        )}
        {rows.length > 0 ? (
          <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
            {rows.map((row) => (
              <NoteRow key={`${row.type}:${row.id}`} row={row} />
            ))}
          </Box>
        ) : (
          <SecondaryText variant="body2">
            <FormattedMessage {...messages.noConditions} />
          </SecondaryText>
        )}
        <SecondaryText variant="caption">
          <FormattedMessage {...messages.hint} />
        </SecondaryText>
      </>
    );
  }

  return (
    <Card>
      <CardContent sx={{ p: 2 }}>
        <Stack alignItems="center" direction="row" spacing={1.5} sx={{ mb: 1.5 }}>
          <Box sx={(theme) => iconTileSx(theme, 36)}>
            <ListChecks size={18} />
          </Box>
          <Typography
            sx={{ flex: 1, fontWeight: 700, lineHeight: 1.3, minWidth: 0 }}
            variant="subtitle2"
          >
            <FormattedMessage {...messages.title} />
          </Typography>
          {ready && rows.length > 0 && (
            <Chip
              color={
                enforcement.enforced === rows.length
                  ? 'success'
                  : anyNotEnforced
                    ? 'warning'
                    : 'default'
              }
              label={
                <FormattedMessage
                  {...messages.summary}
                  values={{ enforced: enforcement.enforced, total: rows.length }}
                />
              }
              size="small"
              variant="outlined"
            />
          )}
        </Stack>
        <Stack spacing={1.5}>{body}</Stack>
      </CardContent>
    </Card>
  );
}
