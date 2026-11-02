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

import { type ReactNode, useEffect, useRef, useState } from 'react';
import {
  Box,
  Button,
  SearchBar,
  Stack,
  TablePagination,
  ToggleButton,
  ToggleButtonGroup,
} from '@wso2/oxygen-ui';
import { LayoutGrid, List, Plus, SearchX } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { useAllGraphQLApis, useDeleteGraphQLApi } from '@/api/resources/graphqlApis';
import { useAllRestApis, useDeleteRestApi } from '@/api/resources/restApis';
import { toGraphQLListableApi, toRestListableApi, type ListableApi } from './apiListItem';
import { ApiGridView } from './ApiGridView';
import { ApiListView } from './ApiListView';
import { EmptyResults } from '@/components/common/EmptyResults';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { ImportApiIllustration } from '@/components/illustrations/ImportApiIllustration';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { routes } from '@/routes/paths';
import { matchesApiType, type ApiTypeFilter } from './apiTypeFilter';
import { CanCreate } from '@/permissions/CanCreate';

type ViewMode = 'grid' | 'list';

/**
 * Page sizes per view; the first is the default. The grid's are divisible by
 * its 2/3/4 columns, so no page ends in a ragged row.
 */
const PAGE_SIZE_OPTIONS: Record<ViewMode, number[]> = {
  list: [10, 20, 50],
  grid: [12, 24, 48],
};
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  clearSearch: { id: 'apiListPage.clearSearch', defaultMessage: 'Clear search' },
  clearFilters: { id: 'apiListPage.clearFilters', defaultMessage: 'Clear filters' },
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
    defaultMessage: 'Try a different API name or clear the active filter.',
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

export function ApiList({
  typeFilter = null,
  sidebar,
  onClearTypeFilter,
}: {
  typeFilter?: ApiTypeFilter | null;
  sidebar?: ReactNode;
  onClearTypeFilter?: () => void;
}) {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const deleteRestApiMutation = useDeleteRestApi();
  const deleteGraphQLApiMutation = useDeleteGraphQLApi();
  const { notify } = useNotifications();

  const [search, setSearch] = useState('');
  const [view, setView] = useState<ViewMode>('list');
  // Each view pages on its own: switching views keeps both where you left them.
  const [paging, setPaging] = useState<Record<ViewMode, { page: number; rowsPerPage: number }>>({
    list: { page: 0, rowsPerPage: PAGE_SIZE_OPTIONS.list[0] },
    grid: { page: 0, rowsPerPage: PAGE_SIZE_OPTIONS.grid[0] },
  });
  const { page, rowsPerPage } = paging[view];
  const pageSizeOptions = PAGE_SIZE_OPTIONS[view];
  const setPage = (next: number) =>
    setPaging((current) => ({ ...current, [view]: { ...current[view], page: next } }));
  const setRowsPerPage = (next: number) =>
    setPaging((current) => ({ ...current, [view]: { page: 0, rowsPerPage: next } }));
  const [toDelete, setToDelete] = useState<ListableApi | null>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  // Back to page 1 when the search or type filter changes. Adjusted while
  // rendering rather than in an effect: an effect runs after a render that has
  // already requested the old, deeper page's window from the server.
  const filterKey = `${debouncedSearch}\u0000${typeFilter ?? ''}`;
  const [pagedFilterKey, setPagedFilterKey] = useState(filterKey);
  if (pagedFilterKey !== filterKey) {
    setPagedFilterKey(filterKey);
    setPaging((current) => ({
      list: { ...current.list, page: 0 },
      grid: { ...current.grid, page: 0 },
    }));
  }

  // Two independent resource types, each already sorted `createdAt desc` by
  // the server, merged into one newest-first list. Paging is server-side: the
  // newest `end` items of the merged list are always within the newest `end`
  // of each source, so fetching just that window from both (not the whole
  // collection) is enough to slice the page out exactly. Search is a
  // server-side substring match on each source.
  //
  // A type filter changes that: `kind` is not a query parameter, so filtering
  // by it needs every item and is done locally; GraphQL has its own endpoint,
  // so it alone is fetched, and windowed.
  const start = page * rowsPerPage;
  const end = start + rowsPerPage;
  const filterByKind = typeFilter !== null && typeFilter !== 'graphql';
  const windowSize = filterByKind ? undefined : end;
  const listFilters = {
    query: debouncedSearch || undefined,
    sortBy: 'createdAt',
    sortOrder: 'desc',
  } as const;
  const restEnabled = typeFilter !== 'graphql';
  const restApisQuery = useAllRestApis(listFilters, {}, { enabled: restEnabled, max: windowSize });
  const graphqlApisQuery = useAllGraphQLApis(listFilters, {}, { max: windowSize });

  // A disabled query never leaves `isPending`; ignore it rather than hang.
  const isPending = (restEnabled && restApisQuery.isPending) || graphqlApisQuery.isPending;
  const error = (restEnabled ? restApisQuery.error : null) ?? graphqlApisQuery.error;
  const isPlaceholderData =
    (restEnabled && restApisQuery.isPlaceholderData) || graphqlApisQuery.isPlaceholderData;

  const restItems = restEnabled ? (restApisQuery.data?.list ?? []).map(toRestListableApi) : [];
  const graphqlItems = (graphqlApisQuery.data?.list ?? []).map(toGraphQLListableApi);
  const mergedApis: ListableApi[] = [...restItems, ...graphqlItems].sort((a, b) =>
    (b.createdAt ?? '').localeCompare(a.createdAt ?? ''),
  );

  // GraphQL items are known-good by construction (they only ever came from
  // the GraphQL endpoint) — matched on `apiType`, not `kind`. The backend's
  // `GraphQLAPIListItem.kind` is a real field, but relying on it here would
  // make this filter fragile to a server omission again (see the "GraphQL"
  // filter matching zero rows despite APIs existing — the list item's `kind`
  // was simply never populated server-side; fixed, but the client should not
  // depend on it for the one type it can determine for certain another way).
  const filteredApis = filterByKind
    ? mergedApis.filter((api) => matchesApiType(api.kind, typeFilter))
    : mergedApis;
  // The server's totals, except under a kind filter, which only the client
  // can count.
  const total = filterByKind
    ? filteredApis.length
    : (restEnabled ? (restApisQuery.data?.pagination.total ?? 0) : 0) +
      (graphqlApisQuery.data?.pagination.total ?? 0);
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  const pageApis = filteredApis.slice(start, end);
  // While the next page loads, `keepPreviousData` leaves the old window in
  // place; re-slicing it at the new offset would flash an empty page, so keep
  // showing the page that was last complete. Only when it came from the same
  // search, type filter and view, though: rows from another query (say,
  // unfiltered ones under a REST filter) must never be shown as if they matched.
  const viewKey = `${filterKey}\u0000${view}`;
  const settledApis = useRef<{ key: string; apis: ListableApi[] }>({ key: '', apis: [] });
  useEffect(() => {
    if (!isPlaceholderData) settledApis.current = { key: viewKey, apis: pageApis };
  });
  const apis =
    isPlaceholderData && settledApis.current.key === viewKey ? settledApis.current.apis : pageApis;
  // Deleting the last card of the last page leaves `page` past the end. Render
  // the clamped value (an out-of-range `page` makes TablePagination complain),
  // and correct the state so the next request asks for a window that exists.
  const currentPage = Math.min(page, lastPage);
  const isSearching = debouncedSearch.length > 0 || typeFilter !== null;
  // Show the create prompt only for an empty project, not an empty search.
  const isFirstRun = total === 0 && !isSearching;

  useEffect(() => {
    if (page > lastPage) setPaging((current) => ({ ...current, [view]: { ...current[view], page: lastPage } }));
  }, [page, lastPage, view]);

  const confirmDelete = () => {
    if (!toDelete?.id) return;
    const { apiType, displayName, id } = toDelete;
    const onSettled = {
      onSuccess: () => {
        notify(intl.formatMessage(messages.deleteSucceeded, { name: displayName }), 'success');
        setToDelete(null);
      },
      onError: (error: { message?: string }) =>
        notify(error.message || intl.formatMessage(messages.deleteFailed), 'error'),
    };
    if (apiType === 'graphql') {
      deleteGraphQLApiMutation.mutate({ graphqlApiId: id }, onSettled);
    } else {
      deleteRestApiMutation.mutate({ restApiId: id }, onSettled);
    }
  };

  const openApi = (api: ListableApi) =>
    navigate(
      api.apiType === 'graphql'
        ? routes.graphqlApi(orgHandle, projectHandler, api.id ?? '')
        : routes.api(orgHandle, projectHandler, api.id ?? ''),
    );
  const createApi = () => navigate(routes.newApi(orgHandle, projectHandler));

  const layoutSx = {
    display: 'grid',
    gridTemplateColumns: {
      xs: 'minmax(0, 1fr)',
      lg: sidebar ? 'minmax(0, 1fr) 280px' : 'minmax(0, 1fr)',
    },
    gap: 4,
    alignItems: 'start',
  } as const;

  // `isPending` rather than `isLoading`: the queries stay disabled until the
  // route's org/project resolve, and in that window `isLoading` is already
  // false with no data, which would flash the "No APIs yet" empty state.
  if (isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (error) {
    // The sidebar (the type statistics and their filters) outlives a failed list.
    return (
      <Box sx={layoutSx}>
        <ErrorState message={intl.formatMessage(messages.errorMessage)} />
        {sidebar && <Box sx={{ minWidth: 0 }}>{sidebar}</Box>}
      </Box>
    );
  }

  return (
    <>
      {isFirstRun ? (
        /*
          Same `restApis` verdict as the populated list's create button below:
          an empty project says nothing about the organization's quota, which
          is counted across every project, so a capped organization reaches
          its first-run prompt here with no room to act on it.
        */
        <EmptyState
          actionIcon={<Plus />}
          actionLabel={intl.formatMessage(messages.emptyAction)}
          component="restApis"
          description={intl.formatMessage(messages.emptyDescription)}
          illustration={<ImportApiIllustration />}
          onAction={createApi}
          title={intl.formatMessage(messages.emptyTitle)}
          operationId="CreateRESTAPI"
        />
      ) : (
        <Box sx={layoutSx}>
          <Stack spacing={2.5} sx={{ minWidth: 0 }}>
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                flexWrap: 'wrap',
                gap: 2,
                justifyContent: 'space-between',
              }}
            >
              <SearchBar
                onChange={(event) => setSearch(event.target.value)}
                placeholder={intl.formatMessage(messages.searchPlaceholder)}
                size="small"
                sx={{ flex: 1, minWidth: { xs: '100%', sm: 160 } }}
                value={search}
              />
              <ToggleButtonGroup
                exclusive
                onChange={(_event, value: ViewMode | null) => {
                  if (value) setView(value);
                }}
                size="small"
                value={view}
              >
                <ToggleButton aria-label={intl.formatMessage(messages.listView)} value="list">
                  <List size={18} />
                </ToggleButton>
                <ToggleButton aria-label={intl.formatMessage(messages.gridView)} value="grid">
                  <LayoutGrid size={18} />
                </ToggleButton>
              </ToggleButtonGroup>
              <CanCreate component="restApis" denied="hide" do="CreateRESTAPI">
                <Button onClick={createApi} startIcon={<Plus size={18} />} variant="contained">
                  <FormattedMessage {...messages.createApiButton} />
                </Button>
              </CanCreate>
            </Box>

            {apis.length === 0 ? (
              <EmptyResults
                title={intl.formatMessage(messages.noMatchesTitle)}
                description={intl.formatMessage(messages.noMatchesDescription)}
                illustration={<SearchX size={34} strokeWidth={1.5} />}
                actionLabel={intl.formatMessage(
                  typeFilter ? messages.clearFilters : messages.clearSearch,
                )}
                onAction={
                  search || onClearTypeFilter
                    ? () => {
                        setSearch('');
                        setPage(0);
                        onClearTypeFilter?.();
                      }
                    : undefined
                }
              />
            ) : (
              <>
                {/* Keeps pagination pinned and the page stable during loading. */}
                <Box
                  sx={{
                    flexGrow: 1,
                    opacity: isPlaceholderData ? 0.6 : 1,
                    transition: 'opacity .15s ease',
                  }}
                >
                  {view === 'grid' ? (
                    <ApiGridView apis={apis} onDelete={setToDelete} onOpen={openApi} />
                  ) : (
                    <ApiListView apis={apis} onDelete={setToDelete} onOpen={openApi} />
                  )}
                </Box>
                {total > pageSizeOptions[0] && (
                  // Pulled up into the Stack's spacing, and a shorter toolbar, so the pager
                  // sits close to the last row in both views.
                  <Box sx={{ mt: -1.5 }}>
                    <TablePagination
                      component="div"
                      count={total}
                      labelRowsPerPage={intl.formatMessage(messages.rowsPerPage)}
                      onPageChange={(_event, nextPage) => setPage(nextPage)}
                      onRowsPerPageChange={(event) => {
                        setRowsPerPage(parseInt(event.target.value, 10));
                      }}
                      page={currentPage}
                      rowsPerPage={rowsPerPage}
                      rowsPerPageOptions={pageSizeOptions}
                      sx={{ '& .MuiTablePagination-toolbar': { minHeight: 40 } }}
                    />
                  </Box>
                )}
              </>
            )}
          </Stack>
          {sidebar && <Box sx={{ minWidth: 0 }}>{sidebar}</Box>}
        </Box>
      )}

      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel, {
          name: toDelete?.displayName ?? '',
        })}
        confirmLabel={intl.formatMessage(messages.deleteConfirm)}
        confirmPhrase={toDelete?.displayName ?? ''}
        destructive
        loading={deleteRestApiMutation.isPending || deleteGraphQLApiMutation.isPending}
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
