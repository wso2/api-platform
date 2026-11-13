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

import { useEffect, useState } from 'react';
import {
  Button,
  IconButton,
  ListingTable,
  PageTitle,
  SearchBar,
  Stack,
  Switch,
  TablePagination,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Pencil, Plus, SearchX, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useMetersById } from '@/api/resources/meters';
import {
  useDeleteSubscriptionPlan,
  useSubscriptionPlan,
  useSubscriptionPlans,
  useUpdateSubscriptionPlan,
  type SubscriptionPlan,
} from '@/api/resources/subscriptionPlans';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { SubscriptionPlanIllustration } from '@/components/illustrations/SubscriptionPlanIllustration';
import { EmptyResults } from '@/components/common/EmptyResults';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { Can } from '@/permissions';
import { PlanConditionChips } from './components/PlanConditionChips';
import { SubscriptionPlanFormDialog } from './components/SubscriptionPlanFormDialog';
import { pricingSummary, summaryMeterIds } from './components/pricingFormat';

const PAGE_SIZE_OPTIONS = [5, 10, 15, 20];
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  activated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.notification.activated',
    defaultMessage: 'Activated plan "{name}".',
  },
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.column.actions',
    defaultMessage: 'Actions',
  },
  columnConditions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.SubscriptionPlansPage.column.conditions',
    defaultMessage: 'Conditions',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.column.name',
    defaultMessage: 'Name',
  },
  columnPricing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.column.pricing',
    defaultMessage: 'Pricing',
  },
  columnStatus: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.column.status',
    defaultMessage: 'Status',
  },
  createButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.action.create',
    defaultMessage: 'Create plan',
  },
  deactivated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.notification.deactivated',
    defaultMessage: 'Deactivated plan "{name}".',
  },
  deleteAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.action.delete',
    defaultMessage: 'Delete {name}',
    description: 'Accessible name of the per-row delete icon button; {name} is the plan.',
  },
  deleteBlockedMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.blockedMessage',
    defaultMessage: '"{name}" is in use and can\'t be deleted.',
    description: 'Shown in the delete dialog, with Delete disabled, for a plan in use.',
  },
  deleteChecking: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.checking',
    defaultMessage: 'Checking whether "{name}" is in use…',
    description:
      "Shown in the delete dialog while the plan's usage is read; Delete is disabled meanwhile.",
  },
  deleteConfirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.confirmAction',
    defaultMessage: 'Delete',
  },
  deleteConfirmInputLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.confirmInputLabel',
    defaultMessage: 'Type the plan name to confirm',
  },
  deleteFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.failed',
    defaultMessage: 'Failed to delete the plan.',
  },
  deleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.message',
    defaultMessage: 'This permanently deletes "{name}". This can’t be undone.',
  },
  deleteSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.succeeded',
    defaultMessage: 'Deleted plan "{name}".',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.delete.title',
    defaultMessage: 'Delete plan',
  },
  editAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.action.edit',
    defaultMessage: 'Edit {name}',
    description: 'Accessible name of the per-row edit icon button; {name} is the plan.',
  },
  emptyAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.empty.action',
    defaultMessage: 'Create plan',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.empty.description',
    defaultMessage:
      'Plans set the conditions an app subscribes to, such as how many requests, tool calls or tokens it can use in a time window. Enable one on the APIs you publish.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.empty.title',
    defaultMessage: 'Create your first subscription plan',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.error.message',
    defaultMessage: 'Unable to load subscription plans',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.loading',
    defaultMessage: 'Loading subscription plans',
  },
  nextPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.pagination.next',
    defaultMessage: 'Next page',
  },
  clearSearch: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.SubscriptionPlansPage.clearSearch',
    defaultMessage: 'Clear search',
  },
  noMatchesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.noMatches.description',
    defaultMessage: 'Try a different search term.',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.noMatches.title',
    defaultMessage: 'No matching plans',
  },
  pageRange: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.pagination.range',
    defaultMessage: '{from}–{to} of {count, number}',
    description: 'Pager summary: the plan rows shown on this page out of the total.',
  },
  previousPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.pagination.previous',
    defaultMessage: 'Previous page',
  },
  rowsPerPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.pagination.rowsPerPage',
    defaultMessage: 'Plans per page',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.searchPlaceholder',
    defaultMessage: 'Search plans',
  },
  statusActive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.status.active',
    defaultMessage: '{name}, active',
    description: 'Accessible name of the per-row status switch when the plan is active.',
  },
  statusInactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.status.inactive',
    defaultMessage: '{name}, inactive',
    description: 'Accessible name of the per-row status switch when the plan is inactive.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.subtitle',
    defaultMessage: 'Define the plans your APIs offer, and the limits and pricing each one gives a subscriber.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.title',
    defaultMessage: 'Subscription Plans',
  },
  updateFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SubscriptionPlansPage.update.failed',
    defaultMessage: 'Failed to update the plan.',
  },
});

export function SubscriptionPlansPage() {
  const intl = useIntl();
  const { notify } = useNotifications();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  const plansQuery = useSubscriptionPlans({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
  });
  // The unit a per-unit price counts (call or token) comes from its meter: only the page's are
  // read.
  const { meters } = useMetersById(
    (plansQuery.data?.list ?? []).flatMap((plan) => summaryMeterIds(plan.pricing)),
  );
  const updateMutation = useUpdateSubscriptionPlan();
  const deleteMutation = useDeleteSubscriptionPlan();

  const [dialogTarget, setDialogTarget] = useState<'create' | SubscriptionPlan | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SubscriptionPlan | null>(null);

  const plans = plansQuery.data?.list ?? [];
  const total = plansQuery.data?.pagination.total ?? 0;
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  const currentPage = Math.min(page, lastPage);
  // The create prompt is for an organization with no plans, not for an empty search.
  const isFirstRun = total === 0 && debouncedSearch === '';

  // A new search starts from the first page.
  useEffect(() => setPage(0), [debouncedSearch]);

  // Deleting the only row on the last page leaves `page` past the end.
  useEffect(() => {
    if (page > lastPage) setPage(lastPage);
  }, [page, lastPage]);

  const toggleStatus = (plan: SubscriptionPlan) => {
    if (!plan.id) return;
    const nextStatus = plan.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE';
    updateMutation.mutate(
      {
        // Sends a partial update so a stale plan can't overwrite its conditions.
        body: { displayName: plan.displayName, id: plan.id, status: nextStatus },
        subscriptionPlanId: plan.id,
      },
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

  // A plan in use can't be deleted: the dialog says so instead of asking to confirm.
  const deleteUsageQuery = useSubscriptionPlan(deleteTarget?.id);
  const checkingUsage = deleteTarget !== null && deleteUsageQuery.isPending;
  const deleteBlocked = deleteUsageQuery.data?.usage?.inUse === true;

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
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {!isFirstRun && (
          <PageTitle.Actions>
            <Can do="CreateSubscriptionPlan" denied="disable">
              <Button
                onClick={() => setDialogTarget('create')}
                startIcon={<Plus size={18} />}
                variant="contained"
              >
                <FormattedMessage {...messages.createButton} />
              </Button>
            </Can>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isFirstRun ? (
        <EmptyState
          actionIcon={<Plus size={18} />}
          actionLabel={intl.formatMessage(messages.emptyAction)}
          description={intl.formatMessage(messages.emptyDescription)}
          illustration={<SubscriptionPlanIllustration />}
          onAction={() => setDialogTarget('create')}
          operationId="CreateSubscriptionPlan"
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Stack spacing={2.5}>
          <SearchBar
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            value={search}
          />

          {plans.length === 0 ? (
            <EmptyResults
              actionLabel={intl.formatMessage(messages.clearSearch)}
              description={intl.formatMessage(messages.noMatchesDescription)}
              illustration={<SearchX size={34} strokeWidth={1.5} />}
              onAction={() => setSearch('')}
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
                        <FormattedMessage {...messages.columnConditions} />
                      </ListingTable.Cell>
                      <ListingTable.Cell>
                        <FormattedMessage {...messages.columnPricing} />
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
                    {plans.map((plan) => (
                      <ListingTable.Row key={plan.id ?? plan.displayName}>
                        <ListingTable.Cell sx={{ maxWidth: 240 }}>
                          <Typography noWrap sx={{ fontWeight: 600 }} title={plan.displayName} variant="body2">
                            {plan.displayName}
                          </Typography>
                        </ListingTable.Cell>
                        <ListingTable.Cell>
                          {/* One line in every row: three chips at most, the rest in "+N". */}
                          <PlanConditionChips conditions={plan.conditions} singleLine />
                        </ListingTable.Cell>
                        <ListingTable.Cell>
                          <Typography
                            color={plan.pricing ? 'text.primary' : 'text.secondary'}
                            noWrap
                            variant="body2"
                          >
                            {pricingSummary(intl, plan.pricing, meters)}
                          </Typography>
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
                                    { name: plan.displayName },
                                  ),
                                },
                              }}
                            />
                          </Can>
                        </ListingTable.Cell>
                        <ListingTable.Cell align="center">
                          <Stack direction="row" justifyContent="center" spacing={0.5}>
                            <Can do="UpdateSubscriptionPlan" denied="hide">
                              <Tooltip
                                title={intl.formatMessage(messages.editAction, {
                                  name: plan.displayName,
                                })}
                              >
                                <IconButton
                                  aria-label={intl.formatMessage(messages.editAction, {
                                    name: plan.displayName,
                                  })}
                                  onClick={() => setDialogTarget(plan)}
                                  size="small"
                                >
                                  <Pencil size={16} />
                                </IconButton>
                              </Tooltip>
                            </Can>
                            <Can do="DeleteSubscriptionPlan" denied="hide">
                              <Tooltip
                                title={intl.formatMessage(messages.deleteAction, {
                                  name: plan.displayName,
                                })}
                              >
                                <IconButton
                                  aria-label={intl.formatMessage(messages.deleteAction, {
                                    name: plan.displayName,
                                  })}
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

          {total > PAGE_SIZE_OPTIONS[0] && (
            <TablePagination
              component="div"
              count={total}
              getItemAriaLabel={(type) =>
                intl.formatMessage(type === 'next' ? messages.nextPage : messages.previousPage)
              }
              labelDisplayedRows={({ count, from, to }) =>
                intl.formatMessage(messages.pageRange, { count, from, to })
              }
              labelRowsPerPage={intl.formatMessage(messages.rowsPerPage)}
              onPageChange={(_, nextPage) => setPage(nextPage)}
              onRowsPerPageChange={(event) => {
                setRowsPerPage(parseInt(event.target.value, 10));
                setPage(0);
              }}
              page={currentPage}
              rowsPerPage={rowsPerPage}
              rowsPerPageOptions={PAGE_SIZE_OPTIONS}
            />
          )}
        </Stack>
      )}

      <SubscriptionPlanFormDialog
        onClose={() => setDialogTarget(null)}
        open={dialogTarget !== null}
        plan={dialogTarget === 'create' ? null : dialogTarget}
      />

      <ConfirmDialog
        confirmDisabled={checkingUsage || deleteBlocked}
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel)}
        confirmLabel={intl.formatMessage(messages.deleteConfirmAction)}
        confirmPhrase={
          checkingUsage || deleteBlocked ? undefined : (deleteTarget?.displayName ?? '')
        }
        destructive
        loading={deleteMutation.isPending}
        message={
          deleteTarget
            ? intl.formatMessage(
                checkingUsage
                  ? messages.deleteChecking
                  : deleteBlocked
                    ? messages.deleteBlockedMessage
                    : messages.deleteMessage,
                { name: deleteTarget.displayName },
              )
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
