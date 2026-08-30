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

import {
  Box,
  Button,
  InputAdornment,
  MenuItem,
  PageTitle,
  Stack,
  TablePagination,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import type { Project } from '@/api/resources/projects';
import { useDeleteProject, useProjects, type ProjectListFilters } from '@/api/resources/projects';
import { ProjectsGrid } from './ProjectsGrid';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { NewProjectDialog } from './components/NewProjectDialog';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { ProjectFolderIllustration } from '@/components/illustrations/ProjectFolderIllustration';

const PAGE_SIZE_OPTIONS = [12, 24, 48];
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Server-side sort is limited to `name | createdAt` and `asc | desc`.
 */
type SortBy = NonNullable<ProjectListFilters['sortBy']>;
type SortOrder = NonNullable<ProjectListFilters['sortOrder']>;

const messages = defineMessages({
  createProject: {
    id: 'project.list.createProjectButton',
    defaultMessage: 'Create Project',
  },
  deleteConfirmInputLabel: {
    id: 'project.list.delete.confirmInputLabel',
    defaultMessage: 'Type "{name}" to confirm',
    description: 'Label for the type-to-confirm field guarding an irreversible delete.',
  },
  deleteConfirm: {
    id: 'project.list.delete.confirmLabel',
    defaultMessage: 'Delete',
  },
  deleteFailed: {
    id: 'project.list.delete.failed',
    defaultMessage: 'Delete failed',
    description: 'Fallback toast when the server gives no reason for a failure.',
  },
  deleteMessage: {
    id: 'project.list.delete.message',
    defaultMessage:
      'This permanently deletes the project "{name}" and its configuration. A project that still has APIs cannot be deleted. This action is irreversible.',
  },
  deleteSucceeded: {
    id: 'project.list.delete.succeeded',
    defaultMessage: 'Deleted "{name}".',
  },
  deleteTitle: {
    id: 'project.list.delete.title',
    defaultMessage: 'Delete project',
  },
  emptyAction: {
    id: 'project.list.empty.action',
    defaultMessage: 'Create project',
  },
  emptyDescription: {
    id: 'project.list.empty.description',
    defaultMessage: 'Create a project to organize and manage your APIs.',
  },
  emptyTitle: {
    id: 'project.list.empty.title',
    defaultMessage: 'Create your first Project',
  },
  errorMessage: {
    id: 'project.list.error.message',
    defaultMessage: 'Unable to load projects. {reason}',
  },
  loading: {
    id: 'project.list.loading',
    defaultMessage: 'Loading projects',
  },
  noMatchesDescription: {
    id: 'project.list.noMatches.description',
    defaultMessage: 'Try a different project name or handle.',
  },
  noMatchesTitle: {
    id: 'project.list.noMatches.title',
    defaultMessage: 'No matching projects',
  },
  projectCount: {
    id: 'project.list.count',
    defaultMessage: '{count, plural, one {# project} other {# projects}}',
  },
  rowsPerPage: {
    id: 'project.list.rowsPerPage',
    defaultMessage: 'Projects per page',
  },
  searchPlaceholder: {
    id: 'project.list.searchPlaceholder',
    defaultMessage: 'Search projects',
  },
  sortLabel: {
    id: 'project.list.sortLabel',
    defaultMessage: 'Sort by',
    description: 'Label for the control choosing the project list order.',
  },
  sortNameAscending: {
    id: 'project.list.sort.nameAscending',
    defaultMessage: 'Name (A–Z)',
    description: 'Sort option: alphabetical by project name, ascending.',
  },
  sortNameDescending: {
    id: 'project.list.sort.nameDescending',
    defaultMessage: 'Name (Z–A)',
    description: 'Sort option: alphabetical by project name, descending.',
  },
  sortNewest: {
    id: 'project.list.sort.newest',
    defaultMessage: 'Newest first',
    description: 'Sort option: by creation date, most recent project first.',
  },
  sortOldest: {
    id: 'project.list.sort.oldest',
    defaultMessage: 'Oldest first',
    description: 'Sort option: by creation date, earliest project first.',
  },
});

/**
 * Sort options with both field and direction.
 */
const SORT_OPTIONS = [
  {
    label: messages.sortNewest,
    sortBy: 'createdAt',
    sortOrder: 'desc',
    value: 'createdAt:desc',
  },
  {
    label: messages.sortOldest,
    sortBy: 'createdAt',
    sortOrder: 'asc',
    value: 'createdAt:asc',
  },
  {
    label: messages.sortNameAscending,
    sortBy: 'name',
    sortOrder: 'asc',
    value: 'name:asc',
  },
  {
    label: messages.sortNameDescending,
    sortBy: 'name',
    sortOrder: 'desc',
    value: 'name:desc',
  },
] as const satisfies readonly {
  label: MessageDescriptor;
  sortBy: SortBy;
  sortOrder: SortOrder;
  value: string;
}[];

type SortOption = (typeof SORT_OPTIONS)[number];

export function ProjectListPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const { organization } = useConsoleScope();
  const { notify } = useNotifications();

  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);
  const [sort, setSort] = useState<SortOption>(SORT_OPTIONS[0]);
  const [createOpen, setCreateOpen] = useState(false);
  const [toDelete, setToDelete] = useState<Project | null>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  // Reset to page 1 when filter or sort changes.
  useEffect(() => setPage(0), [debouncedSearch, sort.value]);

  const projectsQuery = useProjects({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
    sortBy: sort.sortBy,
    sortOrder: sort.sortOrder,
  });
  const deleteProjectMutation = useDeleteProject();

  const projects = projectsQuery.data?.list ?? [];
  const total = projectsQuery.data?.pagination?.total ?? projects.length;
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  // Clamp `page` after deleting the last row of the last page.
  const currentPage = Math.min(page, lastPage);
  const isSearching = debouncedSearch.length > 0;
  // Show the create prompt only for an empty project, not an empty search.
  const isFirstRun = total === 0 && !isSearching;

  useEffect(() => {
    if (page > lastPage) setPage(lastPage);
  }, [page, lastPage]);

  const confirmDelete = () => {
    if (!toDelete) return;
    const { displayName } = toDelete;
    deleteProjectMutation.mutate(
      { projectId: toDelete.id },
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

  const openProject = (project: Project) => navigate(routes.projectHome(orgHandle, project.id));

  if (projectsQuery.isLoading) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (projectsQuery.error) {
    return (
      <ErrorState
        message={intl.formatMessage(messages.errorMessage, {
          reason: projectsQuery.error.message,
        })}
      />
    );
  }

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage id="project.list.title" defaultMessage="Projects" />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          {organization?.displayName ? (
            <FormattedMessage
              defaultMessage="Project workspaces in {organizationName}."
              id="project.list.subHeader.withOrganization"
              values={{ organizationName: organization.displayName }}
            />
          ) : (
            <FormattedMessage
              defaultMessage="Select a project to manage APIs."
              id="project.list.subHeader.default"
            />
          )}
        </PageTitle.SubHeader>
        {!isFirstRun && (
          <PageTitle.Actions>
            <Button
              onClick={() => setCreateOpen(true)}
              startIcon={<Plus />}
              sx={{ borderRadius: 5 }}
              variant="contained"
            >
              <FormattedMessage {...messages.createProject} />
            </Button>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isFirstRun ? (
        <EmptyState
          actionLabel={intl.formatMessage(messages.emptyAction)}
          onAction={() => setCreateOpen(true)}
          title={intl.formatMessage(messages.emptyTitle)}
          description={intl.formatMessage(messages.emptyDescription)}
          actionIcon={<Plus />}
          illustration={<ProjectFolderIllustration />}
        />
      ) : (
        <Stack spacing={2} sx={{ flexGrow: 1 }}>
          {/* Full-bleed search: the field owns its own row across the page. */}
          <TextField
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            size="small"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={18} />
                  </InputAdornment>
                ),
              },
            }}
            value={search}
          />
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 2,
              justifyContent: 'space-between',
            }}
          >
            <Typography variant="h6">
              <FormattedMessage {...messages.projectCount} values={{ count: total }} />
            </Typography>
            <TextField
              label={intl.formatMessage(messages.sortLabel)}
              onChange={(event) => {
                const next = SORT_OPTIONS.find((option) => option.value === event.target.value);
                if (next) setSort(next);
              }}
              select
              size="small"
              sx={{ minWidth: 200 }}
              value={sort.value}
            >
              {SORT_OPTIONS.map((option) => (
                <MenuItem key={option.value} value={option.value}>
                  {intl.formatMessage(option.label)}
                </MenuItem>
              ))}
            </TextField>
          </Box>
          {projects.length === 0 ? (
            <EmptyState
              title={intl.formatMessage(messages.noMatchesTitle)}
              description={intl.formatMessage(messages.noMatchesDescription)}
            />
          ) : (
            <>
              {/* Takes the column's leftover height, which is what keeps the
                  pagination bar at the bottom on a half-empty page. Dimmed
                  rather than unmounted while the next page is in flight, so the
                  grid keeps its height and the page does not jump. */}
              <Box
                sx={{
                  flexGrow: 1,
                  opacity: projectsQuery.isPlaceholderData ? 0.6 : 1,
                  transition: 'opacity .15s ease',
                }}
              >
                <ProjectsGrid
                  onDelete={setToDelete}
                  onOpen={openProject}
                  orgHandle={orgHandle}
                  projects={projects}
                />
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

      <NewProjectDialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        orgHandle={orgHandle}
      />

      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel, {
          name: toDelete?.displayName ?? '',
        })}
        confirmLabel={intl.formatMessage(messages.deleteConfirm)}
        confirmPhrase={toDelete?.displayName ?? ''}
        destructive
        loading={deleteProjectMutation.isPending}
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
