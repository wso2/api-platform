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
  TablePagination,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Pencil, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useDeleteMeter, useMeters, type Meter } from '@/api/resources/meters';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { Can } from '@/permissions';
import { MeterFormDialog } from './components/MeterFormDialog';
import { propertiesLabel, typeLabel } from './components/meterLabels';

const PAGE_SIZE_OPTIONS = [5, 10, 15, 20];
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  columnActions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.column.actions',
    defaultMessage: 'Actions',
  },
  columnFilters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.column.filters',
    defaultMessage: 'Filters',
    description: 'Column of the names of the properties the prices of each meter may filter on.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.column.name',
    defaultMessage: 'Name',
  },
  columnType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.column.type',
    defaultMessage: 'Type',
  },
  createButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.action.create',
    defaultMessage: 'Create meter',
  },
  deleteAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.action.delete',
    defaultMessage: 'Delete {name}',
    description: 'Accessible name of the per-row delete icon button; {name} is the meter.',
  },
  deleteConfirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.confirmAction',
    defaultMessage: 'Delete',
  },
  deleteFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.failed',
    defaultMessage: 'Failed to delete the meter.',
  },
  deleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.message',
    defaultMessage: 'This permanently deletes "{name}".',
  },
  deleteSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.succeeded',
    defaultMessage: 'Deleted meter "{name}".',
  },
  deleteBlockedMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.blockedMessage',
    defaultMessage: '"{name}" is in use and can\'t be deleted.',
    description: 'Shown in the delete dialog, with Delete disabled, for a meter that plans price.',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.delete.title',
    defaultMessage: 'Delete meter',
  },
  editAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.action.edit',
    defaultMessage: 'Edit {name}',
    description: 'Accessible name of the per-row edit icon button; {name} is the meter.',
  },
  emptyAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.empty.action',
    defaultMessage: 'Create meter',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.empty.description',
    defaultMessage:
      'A meter counts the usage a paid plan charges for, such as successful requests. Create a meter, then charge for it in a plan.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.empty.title',
    defaultMessage: 'Create your first meter',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.error.message',
    defaultMessage: 'Unable to load meters',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.loading',
    defaultMessage: 'Loading meters',
  },
  nextPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.pagination.next',
    defaultMessage: 'Next page',
  },
  noFilters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.noFilters',
    defaultMessage: 'None',
    description: 'Filters cell of a meter whose prices cannot filter on any property.',
  },
  noMatchesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.noMatches.description',
    defaultMessage: 'Try a different search term.',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.noMatches.title',
    defaultMessage: 'No matching meters',
  },
  pageRange: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.pagination.range',
    defaultMessage: '{from}–{to} of {count, number}',
    description: 'Pager summary: the meter rows shown on this page out of the total.',
  },
  previousPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.pagination.previous',
    defaultMessage: 'Previous page',
  },
  rowsPerPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.pagination.rowsPerPage',
    defaultMessage: 'Meters per page',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.searchPlaceholder',
    defaultMessage: 'Search meters',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.subtitle',
    defaultMessage: 'Define the usage that paid subscription plans charge for.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.MetersSettingsPage.title',
    defaultMessage: 'Meters',
  },
});

/**
 * Settings › Meters: what paid plans charge for, a page at a time, with a search
 * on name and id. Meters are kept in the Platform API, and a meter reaches an API
 * Portal only when a plan that charges for it is published there.
 */
export function MetersSettingsPage() {
  const intl = useIntl();
  const { notify } = useNotifications();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  const metersQuery = useMeters({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
  });
  const deleteMutation = useDeleteMeter();

  const [dialogTarget, setDialogTarget] = useState<'create' | Meter | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Meter | null>(null);

  const meters = metersQuery.data?.list ?? [];
  const total = metersQuery.data?.pagination.total ?? 0;
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  const currentPage = Math.min(page, lastPage);
  // The create prompt is for an organization with no meters, not for an empty search.
  const isFirstRun = total === 0 && debouncedSearch === '';

  // A new search starts from the first page.
  useEffect(() => setPage(0), [debouncedSearch]);

  // Deleting the only row on the last page leaves `page` past the end.
  useEffect(() => {
    if (page > lastPage) setPage(lastPage);
  }, [page, lastPage]);

  // A meter that plans price can't change: it opens with Save off.
  const isPriced = (meter: Meter) => (meter.planCount ?? 0) > 0;
  // Nor can it be deleted: the dialog says it is in use.
  const deleteBlocked = deleteTarget !== null && isPriced(deleteTarget);

  const confirmDelete = () => {
    if (!deleteTarget?.id) return;
    const { displayName, id } = deleteTarget;
    deleteMutation.mutate(
      { meterId: id },
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

  if (metersQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (metersQuery.error) return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;

  const table = (
    <ListingTable.Provider>
      <ListingTable.Container>
        <ListingTable>
          <ListingTable.Head>
            <ListingTable.Row>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnName} />
              </ListingTable.Cell>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnType} />
              </ListingTable.Cell>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnFilters} />
              </ListingTable.Cell>
              <ListingTable.Cell align="center" sx={{ width: 112 }}>
                <FormattedMessage {...messages.columnActions} />
              </ListingTable.Cell>
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {meters.map((meter) => {
              const filters = propertiesLabel(meter);
              const name = meter.displayName ?? meter.id ?? '';
              return (
                <ListingTable.Row key={meter.id ?? name}>
                  <ListingTable.Cell>
                    <Typography sx={{ fontWeight: 600 }} variant="body2">
                      {name}
                    </Typography>
                    {meter.description && (
                      <Typography color="text.secondary" variant="caption">
                        {meter.description}
                      </Typography>
                    )}
                  </ListingTable.Cell>
                  <ListingTable.Cell>{typeLabel(intl, meter)}</ListingTable.Cell>
                  <ListingTable.Cell>
                    <Typography color={filters ? 'text.primary' : 'text.secondary'} variant="body2">
                      {filters || intl.formatMessage(messages.noFilters)}
                    </Typography>
                  </ListingTable.Cell>
                  <ListingTable.Cell align="center">
                    <Stack direction="row" justifyContent="center" spacing={0.5}>
                      <Can do="UpdateMeter" denied="hide">
                        <Tooltip title={intl.formatMessage(messages.editAction, { name })}>
                          <IconButton
                            aria-label={intl.formatMessage(messages.editAction, { name })}
                            onClick={() => setDialogTarget(meter)}
                            size="small"
                          >
                            <Pencil size={16} />
                          </IconButton>
                        </Tooltip>
                      </Can>
                      <Can do="DeleteMeter" denied="hide">
                        <Tooltip title={intl.formatMessage(messages.deleteAction, { name })}>
                          <IconButton
                            aria-label={intl.formatMessage(messages.deleteAction, { name })}
                            color="error"
                            onClick={() => setDeleteTarget(meter)}
                            size="small"
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </Tooltip>
                      </Can>
                    </Stack>
                  </ListingTable.Cell>
                </ListingTable.Row>
              );
            })}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </ListingTable.Provider>
  );

  return (
    <>
      <PageTitle>
        <PageTitle.Header variant="h5">
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {!isFirstRun && (
          <PageTitle.Actions>
            <Can do="CreateMeter" denied="disable">
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
          onAction={() => setDialogTarget('create')}
          operationId="CreateMeter"
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

          {meters.length === 0 ? (
            <EmptyState
              description={intl.formatMessage(messages.noMatchesDescription)}
              title={intl.formatMessage(messages.noMatchesTitle)}
            />
          ) : (
            table
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

      <MeterFormDialog
        meter={dialogTarget === 'create' ? null : dialogTarget}
        onClose={() => setDialogTarget(null)}
        open={dialogTarget !== null}
        readOnly={dialogTarget !== null && dialogTarget !== 'create' && isPriced(dialogTarget)}
      />

      <ConfirmDialog
        confirmDisabled={deleteBlocked}
        confirmLabel={intl.formatMessage(messages.deleteConfirmAction)}
        destructive
        loading={deleteMutation.isPending}
        message={
          deleteTarget
            ? intl.formatMessage(
                deleteBlocked ? messages.deleteBlockedMessage : messages.deleteMessage,
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
