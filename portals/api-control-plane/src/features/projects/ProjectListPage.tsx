import {
  Box,
  Button,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useDeleteProject, useProjects } from '../../api/hooks/useMvpQueries';
import { ProjectsGrid } from '../../components/cards/ProjectsGrid';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useNotifications } from '../../components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import type { Project } from '../../types/domain';
import { NewProjectDialog } from './NewProjectDialog';

export function ProjectListPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { organization } = useConsoleScope();
  const { notify } = useNotifications();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [toDelete, setToDelete] = useState<Project | null>(null);
  const projectsQuery = useProjects();
  const deleteProjectMutation = useDeleteProject();
  const projects = projectsQuery.data || [];

  const confirmDelete = () => {
    if (!toDelete) return;
    deleteProjectMutation.mutate(toDelete, {
      onSuccess: () => {
        notify(`Deleted "${toDelete.name}".`, 'success');
        setToDelete(null);
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Delete failed',
          'error'
        ),
    });
  };

  const filteredProjects = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return projects;
    return projects.filter((project) =>
      [project.name, project.handler, project.region, project.description]
        .filter(Boolean)
        .some((field) => field!.toLowerCase().includes(term))
    );
  }, [projects, search]);

  const openProject = (project: Project) =>
    navigate(routes.projectHome(orgHandle, project.handler));

  if (projectsQuery.isLoading) return <LoadingState label="Loading projects" />;
  if (projectsQuery.error) {
    return (
      <ErrorState
        message={`Unable to load projects. ${projectsQuery.error.message}`}
      />
    );
  }

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>Projects</PageTitle.Header>
        <PageTitle.SubHeader>
          {organization?.name
            ? `Project workspaces in ${organization.name}.`
            : 'Select a project to manage APIs.'}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            onClick={() => setCreateOpen(true)}
            startIcon={<Plus />}
            sx={{ borderRadius: 5 }}
            variant="contained"
          >
            New project
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {projects.length === 0 ? (
        <EmptyState
          actionLabel="Create project"
          onAction={() => setCreateOpen(true)}
          title="No projects found"
          description="Create a project to organize and manage your APIs."
        />
      ) : (
        <Stack spacing={2}>
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
              {filteredProjects.length} project
              {filteredProjects.length === 1 ? '' : 's'}
            </Typography>
            <TextField
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search projects"
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
              sx={{ minWidth: 260 }}
              value={search}
            />
          </Box>
          {filteredProjects.length === 0 ? (
            <EmptyState
              title="No matching projects"
              description="Try a different project name, handler, region, or description."
            />
          ) : (
            <ProjectsGrid
              onDelete={setToDelete}
              onOpen={openProject}
              orgHandle={orgHandle}
              projects={filteredProjects}
            />
          )}
        </Stack>
      )}

      <NewProjectDialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        orgHandle={orgHandle}
      />

      <ConfirmDialog
        confirmInputLabel={`Type "${toDelete?.name ?? ''}" to confirm`}
        confirmLabel="Delete"
        confirmPhrase={toDelete?.name ?? ''}
        destructive
        loading={deleteProjectMutation.isPending}
        message={
          toDelete
            ? `This permanently deletes the project "${toDelete.name}" and its configuration. A project that still has APIs cannot be deleted. This action is irreversible.`
            : ''
        }
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
        open={toDelete !== null}
        title="Delete project"
      />
    </PageContent>
  );
}
