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

import { useState } from 'react';
import {
  Box,
  Button,
  Chip,
  IconButton,
  ListingTable,
  SearchBar,
  Stack,
  Switch,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Pencil, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  useDeleteSubscriptionPlan,
  useSubscriptionPlans,
  useUpdateSubscriptionPlan,
  type SubscriptionPlan,
} from '@/api/resources/subscriptionPlans';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { SubscriptionPlanFormDialog } from './components/SubscriptionPlanFormDialog';

const messages = defineMessages({
  activated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.notification.activated',
    defaultMessage: 'Activated plan "{name}".',
  },
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.column.actions',
    defaultMessage: 'Actions',
  },
  columnLimits: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.column.limits',
    defaultMessage: 'Limits',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.column.name',
    defaultMessage: 'Name',
  },
  columnStatus: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.column.status',
    defaultMessage: 'Status',
  },
  createButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.action.create',
    defaultMessage: 'Create plan',
  },
  deactivated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.notification.deactivated',
    defaultMessage: 'Deactivated plan "{name}".',
  },
  deleteAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.action.delete',
    defaultMessage: 'Delete plan',
  },
  deleteConfirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.confirmAction',
    defaultMessage: 'Delete',
  },
  deleteConfirmInputLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.confirmInputLabel',
    defaultMessage: 'Type the plan name to confirm',
  },
  deleteFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.failed',
    defaultMessage: 'Failed to delete the plan.',
  },
  deleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.message',
    defaultMessage: 'This permanently deletes "{name}". This can’t be undone.',
  },
  deleteSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.succeeded',
    defaultMessage: 'Deleted plan "{name}".',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.delete.title',
    defaultMessage: 'Delete plan',
  },
  editAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.action.edit',
    defaultMessage: 'Edit plan',
  },
  emptyAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.empty.action',
    defaultMessage: 'Create plan',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.empty.description',
    defaultMessage:
      'Plans define how many requests an application can make in a time window. Create a plan, then enable it on the APIs you publish.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.empty.title',
    defaultMessage: 'Create your first subscription plan',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.error.message',
    defaultMessage: 'Unable to load subscription plans',
  },
  limitChip: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.limitChip',
    defaultMessage:
      'req · {count} / {amount, plural, one {} other {# }}{unit, select, MINUTE {min} HOUR {hr} DAY {day} MONTH {mo} other {}}',
    description:
      'Compact monospace tag summarizing one throttling limit, e.g. "req · 1000 / hr" or "req · 100000 / 2 hr".',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.loading',
    defaultMessage: 'Loading subscription plans',
  },
  noMatchesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.noMatches.description',
    defaultMessage: 'Try a different search term.',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.noMatches.title',
    defaultMessage: 'No matching plans',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.searchPlaceholder',
    defaultMessage: 'Search plans',
  },
  statusActive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.status.active',
    defaultMessage: 'Active',
  },
  statusInactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.status.inactive',
    defaultMessage: 'Inactive',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.subtitle',
    defaultMessage: 'Define the rate and quota tiers applications can subscribe to.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.title',
    defaultMessage: 'Subscription plans',
  },
  unlimitedChip: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.unlimitedChip',
    defaultMessage: 'req · ∞',
    description: 'Chip shown for a plan with no configured limits.',
  },
  updateFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansSettingsPage.update.failed',
    defaultMessage: 'Failed to update the plan.',
  },
});

/** Which limits show as a chip on one row, `req · {count} / {window}` per
 * entry, or a single unlimited chip when the plan has none configured. */
const useLimitChipLabels = () => {
  const intl = useIntl();
  return (plan: SubscriptionPlan): string[] => {
    const limits = plan.limits ?? [];
    if (limits.length === 0) return [intl.formatMessage(messages.unlimitedChip)];
    return limits.map((limit) =>
      intl.formatMessage(messages.limitChip, {
        amount: limit.timeAmount ?? 1,
        count: limit.limitCount,
        unit: limit.timeUnit,
      }),
    );
  };
};

export function SubscriptionPlansSettingsPage() {
  const intl = useIntl();
  const { notify } = useNotifications();
  const plansQuery = useSubscriptionPlans();
  const updateMutation = useUpdateSubscriptionPlan();
  const deleteMutation = useDeleteSubscriptionPlan();
  const limitChipLabels = useLimitChipLabels();

  const [search, setSearch] = useState('');
  const [dialogTarget, setDialogTarget] = useState<'create' | SubscriptionPlan | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SubscriptionPlan | null>(null);

  const plans = plansQuery.data?.list ?? [];
  const term = search.trim().toLowerCase();
  const filtered = term
    ? plans.filter((plan) => `${plan.displayName} ${plan.id ?? ''}`.toLowerCase().includes(term))
    : plans;
  const isFirstRun = plans.length === 0;

  const toggleStatus = (plan: SubscriptionPlan) => {
    if (!plan.id) return;
    const nextStatus = plan.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
    updateMutation.mutate(
      { body: { ...plan, status: nextStatus }, subscriptionPlanId: plan.id },
      {
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.updateFailed), 'error'),
        onSuccess: () =>
          notify(
            intl.formatMessage(
              nextStatus === 'ACTIVE' ? messages.activated : messages.deactivated,
              {
                name: plan.displayName,
              },
            ),
            'success',
          ),
      },
    );
  };

  const confirmDelete = () => {
    if (!deleteTarget?.id) return;
    const { displayName, id } = deleteTarget;
    deleteMutation.mutate(
      { subscriptionPlanId: id },
      {
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.deleteFailed), 'error'),
        onSuccess: () => {
          notify(intl.formatMessage(messages.deleteSucceeded, { name: displayName }), 'success');
          setDeleteTarget(null);
        },
      },
    );
  };

  if (plansQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (plansQuery.error) return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;

  return (
    <>
      <Stack spacing={2.5}>
        <Box
          sx={{
            alignItems: 'flex-start',
            display: 'flex',
            flexWrap: 'wrap',
            gap: 2,
            justifyContent: 'space-between',
          }}
        >
          <Stack spacing={0.5} sx={{ minWidth: 260 }}>
            <Typography sx={{ fontWeight: 700 }} variant="h5">
              <FormattedMessage {...messages.title} />
            </Typography>
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.subtitle} />
            </Typography>
          </Stack>
          {!isFirstRun && (
            <Can do="CreateSubscriptionPlan" denied="disable">
              <Button
                onClick={() => setDialogTarget('create')}
                startIcon={<Plus size={18} />}
                variant="contained"
              >
                <FormattedMessage {...messages.createButton} />
              </Button>
            </Can>
          )}
        </Box>

        {isFirstRun ? (
          <EmptyState
            actionIcon={<Plus size={18} />}
            actionLabel={intl.formatMessage(messages.emptyAction)}
            description={intl.formatMessage(messages.emptyDescription)}
            onAction={() => setDialogTarget('create')}
            operationId="CreateSubscriptionPlan"
            title={intl.formatMessage(messages.emptyTitle)}
          />
        ) : (
          <>
            <SearchBar
              fullWidth
              onChange={(event) => setSearch(event.target.value)}
              placeholder={intl.formatMessage(messages.searchPlaceholder)}
              value={search}
            />

            {filtered.length === 0 ? (
              <EmptyState
                description={intl.formatMessage(messages.noMatchesDescription)}
                title={intl.formatMessage(messages.noMatchesTitle)}
              />
            ) : (
              <ListingTable.Provider>
                <ListingTable.Container>
                  <ListingTable>
                    <ListingTable.Head>
                      <ListingTable.Row>
                        <ListingTable.Cell>
                          <FormattedMessage {...messages.columnName} />
                        </ListingTable.Cell>
                        <ListingTable.Cell>
                          <FormattedMessage {...messages.columnLimits} />
                        </ListingTable.Cell>
                        <ListingTable.Cell align="center" sx={{ width: 96 }}>
                          <FormattedMessage {...messages.columnStatus} />
                        </ListingTable.Cell>
                        <ListingTable.Cell align="center" sx={{ width: 112 }}>
                          <FormattedMessage {...messages.columnActions} />
                        </ListingTable.Cell>
                      </ListingTable.Row>
                    </ListingTable.Head>
                    <ListingTable.Body>
                      {filtered.map((plan) => (
                        <ListingTable.Row key={plan.id ?? plan.displayName}>
                          <ListingTable.Cell>
                            <Typography sx={{ fontWeight: 600 }} variant="body2">
                              {plan.displayName}
                            </Typography>
                          </ListingTable.Cell>
                          <ListingTable.Cell>
                            <Stack direction="row" flexWrap="wrap" gap={0.75}>
                              {limitChipLabels(plan).map((label, index) => (
                                <Chip
                                  key={label + index}
                                  label={label}
                                  size="small"
                                  sx={{
                                    fontFamily: 'monospace',
                                    fontWeight: 500,
                                    opacity: index === 0 ? 1 : 0.6,
                                    typography: 'caption',
                                  }}
                                  variant="filled"
                                />
                              ))}
                            </Stack>
                          </ListingTable.Cell>
                          <ListingTable.Cell align="center">
                            <Can do="UpdateSubscriptionPlan" denied="disable">
                              <Switch
                                checked={plan.status === 'ACTIVE'}
                                disabled={updateMutation.isPending}
                                onChange={() => toggleStatus(plan)}
                                size="small"
                                slotProps={{
                                  input: {
                                    'aria-label': intl.formatMessage(
                                      plan.status === 'ACTIVE'
                                        ? messages.statusActive
                                        : messages.statusInactive,
                                    ),
                                  },
                                }}
                              />
                            </Can>
                          </ListingTable.Cell>
                          <ListingTable.Cell align="center">
                            <Stack direction="row" justifyContent="center" spacing={0.5}>
                              <Can do="UpdateSubscriptionPlan" denied="hide">
                                <Tooltip title={intl.formatMessage(messages.editAction)}>
                                  <IconButton
                                    aria-label={intl.formatMessage(messages.editAction)}
                                    onClick={() => setDialogTarget(plan)}
                                    size="small"
                                  >
                                    <Pencil size={16} />
                                  </IconButton>
                                </Tooltip>
                              </Can>
                              <Can do="DeleteSubscriptionPlan" denied="hide">
                                <Tooltip title={intl.formatMessage(messages.deleteAction)}>
                                  <IconButton
                                    aria-label={intl.formatMessage(messages.deleteAction)}
                                    color="error"
                                    onClick={() => setDeleteTarget(plan)}
                                    size="small"
                                  >
                                    <Trash2 size={16} />
                                  </IconButton>
                                </Tooltip>
                              </Can>
                            </Stack>
                          </ListingTable.Cell>
                        </ListingTable.Row>
                      ))}
                    </ListingTable.Body>
                  </ListingTable>
                </ListingTable.Container>
              </ListingTable.Provider>
            )}
          </>
        )}
      </Stack>

      <SubscriptionPlanFormDialog
        onClose={() => setDialogTarget(null)}
        open={dialogTarget !== null}
        plan={dialogTarget === 'create' ? null : dialogTarget}
      />

      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel)}
        confirmLabel={intl.formatMessage(messages.deleteConfirmAction)}
        confirmPhrase={deleteTarget?.displayName ?? ''}
        destructive
        loading={deleteMutation.isPending}
        message={
          deleteTarget
            ? intl.formatMessage(messages.deleteMessage, { name: deleteTarget.displayName })
            : ''
        }
        onCancel={() => setDeleteTarget(null)}
        onConfirm={confirmDelete}
        open={deleteTarget !== null}
        title={intl.formatMessage(messages.deleteTitle)}
      />
    </>
  );
}
