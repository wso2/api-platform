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
  Card,
  Divider,
  InputAdornment,
  PageTitle,
  Stack,
  TablePagination,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import type { Project } from '@/api/resources/projects';
import { useDeleteProject, useProjects } from '@/api/resources/projects';
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

const messages = defineMessages({
  createProject: {
    id: 'project.list.createProjectButton',
    defaultMessage: 'New project',
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
  rowsPerPage: {
    id: 'project.list.rowsPerPage',
    defaultMessage: 'Projects per page',
  },
  searchPlaceholder: {
    id: 'project.list.searchPlaceholder',
    defaultMessage: 'Search projects',
  },
});

export function ProjectListPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const { organization } = useConsoleScope();
  const { notify } = useNotifications();

  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);
  const [createOpen, setCreateOpen] = useState(false);
  const [toDelete, setToDelete] = useState<Project | null>(null);

  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  // A new filter starts from the first page.
  useEffect(() => setPage(0), [debouncedSearch]);

  const projectsQuery = useProjects({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
    sortBy: 'createdAt',
    sortOrder: 'desc',
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
        <Card sx={{ overflow: 'hidden' }}>
          <Stack
            alignItems="center"
            direction={{ sm: 'row', xs: 'column' }}
            justifyContent="space-between"
            spacing={2}
            sx={{ p: 2.5, width: '100%' }}
          >
            <Stack alignItems="center" direction="row" spacing={2}>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                <FormattedMessage id="project.list.title" defaultMessage="Projects" />
              </Typography>
              <Divider flexItem orientation="vertical" />
              <Typography color="text.secondary" variant="body1">
                {total}
              </Typography>
            </Stack>
            <Stack
              alignItems="center"
              direction={{ sm: 'row', xs: 'column' }}
              spacing={2}
              sx={{ width: { sm: 'auto', xs: '100%' } }}
            >
              <TextField
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
                sx={{ width: { sm: 320, xs: '100%' } }}
                value={search}
              />
              <Button
                onClick={() => setCreateOpen(true)}
                startIcon={<Plus />}
                sx={{ borderRadius: 5, whiteSpace: 'nowrap' }}
                variant="outlined"
              >
                <FormattedMessage {...messages.createProject} />
              </Button>
            </Stack>
          </Stack>
          <Divider />
          {projects.length === 0 ? (
            <Box sx={{ py: 6 }}>
              <EmptyState
                title={intl.formatMessage(messages.noMatchesTitle)}
                description={intl.formatMessage(messages.noMatchesDescription)}
              />
            </Box>
          ) : (
            <>
              <Box
                sx={{
                  flexGrow: 1,
                  opacity: projectsQuery.isPlaceholderData ? 0.6 : 1,
                  transition: 'opacity .15s ease',
                }}
              >
                <ProjectsGrid onDelete={setToDelete} onOpen={openProject} projects={projects} />
              </Box>
              {total > PAGE_SIZE_OPTIONS[0] && (
                <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
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
        </Card>
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
