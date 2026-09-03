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
  Box,
  Button,
  SearchBar,
  Stack,
  TablePagination,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { LayoutGrid, List, Plus } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { useDeleteRestApi, useRestApis, type RestApi } from '@/api/resources/restApis';
import { ApiGridView } from './ApiGridView';
import { ApiListView } from './ApiListView';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { MonitorIllustration } from '@/components/illustrations/MonitorIllustration';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { routes } from '@/routes/paths';

type ViewMode = 'grid' | 'list';

/** Divisible by the grid's 2/3/4 columns, so no page ends in a ragged row. */
const PAGE_SIZE_OPTIONS = [12, 24, 48];
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  apiCount: {
    id: 'apiListPage.count',
    defaultMessage: '{count, plural, one {# API} other {# APIs}}',
    description: 'Heading above the listing, counting every match, not the page.',
  },
  createApiButton: {
    id: 'apiListPage.createApiButton',
    defaultMessage: 'Create API',
    description: 'Button label for creating a new API',
  },
  deleteConfirm: {
    id: 'apiListPage.delete.confirmLabel',
    defaultMessage: 'Delete',
  },
  deleteConfirmInputLabel: {
    id: 'apiListPage.delete.confirmInputLabel',
    defaultMessage: 'Type "{name}" to confirm',
    description: 'Label for the type-to-confirm field guarding an irreversible delete.',
  },
  deleteFailed: {
    id: 'apiListPage.delete.failed',
    defaultMessage: 'Delete failed',
    description: 'Fallback toast when the server gives no reason for a failure.',
  },
  deleteMessage: {
    id: 'apiListPage.delete.message',
    defaultMessage:
      'This permanently deletes the API "{name}" and all related details. This action is irreversible.',
  },
  deleteSucceeded: {
    id: 'apiListPage.delete.succeeded',
    defaultMessage: 'Deleted "{name}".',
  },
  deleteTitle: {
    id: 'apiListPage.delete.title',
    defaultMessage: 'Delete API',
  },
  emptyAction: {
    id: 'apiListPage.empty.action',
    defaultMessage: 'Create API',
  },
  emptyDescription: {
    id: 'apiListPage.empty.description',
    defaultMessage:
      'Set up an API to expose your backend and manage access across your applications.',
    description: 'Body of the first-run prompt shown to a project with no APIs.',
  },
  emptyTitle: {
    id: 'apiListPage.empty.title',
    defaultMessage: 'Create your first API',
  },
  errorMessage: {
    id: 'apiListPage.error.message',
    defaultMessage: 'Unable to load APIs',
  },
  gridView: {
    id: 'apiListPage.view.grid',
    defaultMessage: 'Grid view',
    description: 'Accessible label for the button switching to the card grid.',
  },
  listView: {
    id: 'apiListPage.view.list',
    defaultMessage: 'List view',
    description: 'Accessible label for the button switching to compact rows.',
  },
  loading: {
    id: 'apiListPage.loading',
    defaultMessage: 'Loading APIs',
  },
  noMatchesDescription: {
    id: 'apiListPage.noMatches.description',
    defaultMessage: 'Try a different API name or clear the search.',
  },
  noMatchesTitle: {
    id: 'apiListPage.noMatches.title',
    defaultMessage: 'No matching APIs',
  },
  rowsPerPage: {
    id: 'apiListPage.rowsPerPage',
    defaultMessage: 'APIs per page',
  },
  searchPlaceholder: {
    id: 'apiListPage.searchPlaceholder',
    defaultMessage: 'Search APIs',
  },
});

export function ApiList() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const deleteApiMutation = useDeleteRestApi();
  const { notify } = useNotifications();

  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);
  const [view, setView] = useState<ViewMode>('grid');
  const [toDelete, setToDelete] = useState<RestApi | null>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  // Reset to page 1 when the filter or sort changes.
  useEffect(() => setPage(0), [debouncedSearch]);

  // Server-side, so search and pagination agree: `query` is a substring match
  // on the API handle, applied across the whole collection rather than to the
  // page already in cache.
  const apisQuery = useRestApis({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
    sortBy: 'createdAt',
    sortOrder: 'desc',
  });

  const apis = apisQuery.data?.list ?? [];
  const total = apisQuery.data?.pagination?.total ?? apis.length;
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  // Deleting the last card of the last page leaves `page` past the end. Render
  // the clamped value (an out-of-range `page` makes TablePagination complain),
  // and correct the state so the next request asks for a window that exists.
  const currentPage = Math.min(page, lastPage);
  const isSearching = debouncedSearch.length > 0;
  // Show the create prompt only for an empty project, not an empty search.
  const isFirstRun = total === 0 && !isSearching;

  useEffect(() => {
    if (page > lastPage) setPage(lastPage);
  }, [page, lastPage]);

  const confirmDelete = () => {
    if (!toDelete?.id) return;
    const { displayName } = toDelete;
    deleteApiMutation.mutate(
      { restApiId: toDelete.id },
      {
        onSuccess: () => {
          notify(intl.formatMessage(messages.deleteSucceeded, { name: displayName }), 'success');
          setToDelete(null);
        },
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.deleteFailed), 'error'),
      },
    );
  };

  const openApi = (api: RestApi) => navigate(routes.api(orgHandle, projectHandler, api.id ?? ''));
  const createApi = () => navigate(routes.newApi(orgHandle, projectHandler));

  // `isPending` rather than `isLoading`: the query stays disabled until the
  // route's org/project resolve, and in that window `isLoading` is already
  // false with no data, which would flash the "No APIs yet" empty state.
  if (apisQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (apisQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  return (
    <>
      {isFirstRun ? (
        <EmptyState
          actionIcon={<Plus />}
          actionLabel={intl.formatMessage(messages.emptyAction)}
          description={intl.formatMessage(messages.emptyDescription)}
          illustration={<MonitorIllustration />}
          onAction={createApi}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Stack spacing={2.5} sx={{ flexGrow: 1 }}>
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 2,
              justifyContent: 'space-between',
            }}
          >
            <Stack alignItems="baseline" direction="row" spacing={1.25}>
              <Typography sx={{ fontWeight: 800 }} variant="h4">
                <FormattedMessage
                  defaultMessage="APIs"
                  description="Page title for the API list page"
                  id="apiListPage.title"
                />
              </Typography>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.apiCount} values={{ count: total }} />
              </Typography>
            </Stack>
            <Stack alignItems="center" direction="row" spacing={1.5}>
              <SearchBar
                onChange={(event) => setSearch(event.target.value)}
                placeholder={intl.formatMessage(messages.searchPlaceholder)}
                size="small"
                sx={{ minWidth: 320 }}
                value={search}
              />
              <Button onClick={createApi} startIcon={<Plus size={18} />} variant="contained">
                <FormattedMessage {...messages.createApiButton} />
              </Button>
              <ToggleButtonGroup
                exclusive
                onChange={(_event, value: ViewMode | null) => {
                  if (value) setView(value);
                }}
                size="small"
                value={view}
              >
                <ToggleButton aria-label={intl.formatMessage(messages.gridView)} value="grid">
                  <LayoutGrid size={16} />
                </ToggleButton>
                <ToggleButton aria-label={intl.formatMessage(messages.listView)} value="list">
                  <List size={16} />
                </ToggleButton>
              </ToggleButtonGroup>
            </Stack>
          </Box>

          {apis.length === 0 ? (
            <EmptyState
              description={intl.formatMessage(messages.noMatchesDescription)}
              title={intl.formatMessage(messages.noMatchesTitle)}
            />
          ) : (
            <>
              {/* Keeps pagination pinned and the page stable during loading. */}
              <Box
                sx={{
                  flexGrow: 1,
                  opacity: apisQuery.isPlaceholderData ? 0.6 : 1,
                  transition: 'opacity .15s ease',
                }}
              >
                {view === 'grid' ? (
                  <ApiGridView apis={apis} onDelete={setToDelete} onOpen={openApi} />
                ) : (
                  <ApiListView apis={apis} onDelete={setToDelete} onOpen={openApi} />
                )}
              </Box>
              {total > PAGE_SIZE_OPTIONS[0] && (
                <Box>
                  <TablePagination
                    component="div"
                    count={total}
                    labelRowsPerPage={intl.formatMessage(messages.rowsPerPage)}
                    onPageChange={(_event, nextPage) => setPage(nextPage)}
                    onRowsPerPageChange={(event) => {
                      setRowsPerPage(parseInt(event.target.value, 10));
                      setPage(0);
                    }}
                    page={currentPage}
                    rowsPerPage={rowsPerPage}
                    rowsPerPageOptions={PAGE_SIZE_OPTIONS}
                  />
                </Box>
              )}
            </>
          )}
        </Stack>
      )}

      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel, {
          name: toDelete?.displayName ?? '',
        })}
        confirmLabel={intl.formatMessage(messages.deleteConfirm)}
        confirmPhrase={toDelete?.displayName ?? ''}
        destructive
        loading={deleteApiMutation.isPending}
        message={
          toDelete
            ? intl.formatMessage(messages.deleteMessage, {
                name: toDelete.displayName,
              })
            : ''
        }
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
        open={toDelete !== null}
        title={intl.formatMessage(messages.deleteTitle)}
      />
    </>
  );
}
