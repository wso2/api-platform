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

import React, { useMemo, useState, useEffect } from 'react';
import { Link as RouterLink, useLocation, useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Form,
  Grid,
  InputAdornment,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
  IconButton,
} from '@wso2/oxygen-ui';
import {
  ChevronLeft,
  ChevronRight,
  Clock,
  Layers,
  Pencil,
  Plus,
  Search,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import { ProjectsProvider, useProjects } from '../../../../contexts/ProjectsContext';
import AILoader from '../../../../Components/AILoader';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { FormattedMessage } from 'react-intl';
import NoProjects from '../../../../assets/images/NoProjects.svg';


function ProjectListViewInner() {
  const navigate = useNavigate();
  const location = useLocation();
  const {
    projectsForCurrentOrganization,
    isProjectsLoading,
    currentOrganization,
    setCurrentProject,
    error,
  } = useAppShell();
  const { deleteProject } = useProjects();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const filteredProjects = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return projectsForCurrentOrganization;

    return projectsForCurrentOrganization.filter((project) => {
      const haystack = [project.displayName, project.description, project.id]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [projectsForCurrentOrganization, searchQuery]);

  const totalProjects = filteredProjects.length;
  const pageCount = Math.max(1, Math.ceil(totalProjects / rowsPerPage));
  const showPagination = totalProjects > 10;
  const safePage = Math.min(page, pageCount - 1);
  const startIndex = safePage * rowsPerPage;
  const endIndex = Math.min(totalProjects, startIndex + rowsPerPage);
  const pagedProjects = filteredProjects.slice(startIndex, endIndex);

  useEffect(() => {
    setPage(0);
  }, [rowsPerPage, searchQuery, projectsForCurrentOrganization.length]);

  const orgProjectsPath = buildOrgPath(currentOrganization, '/projects/list');
  const newProjectPath = buildOrgPath(currentOrganization, '/projects/create');
  const hasProjects = projectsForCurrentOrganization.length > 0;

  // Must be before any early returns — Rules of Hooks
  useEffect(() => {
    if (!orgProjectsPath) return;
    if (location.pathname !== orgProjectsPath) {
      navigate(orgProjectsPath, { replace: true });
    }
  }, [orgProjectsPath, location.pathname, navigate]);

  const deleteTarget = projectsForCurrentOrganization.find(
    (p) => p.id === deleteTargetId
  );

  const handleDeleteConfirm = async () => {
    if (!deleteTargetId) return;
    setIsDeleting(true);
    try {
      await deleteProject(deleteTargetId);
      showSnackbar('Project deleted successfully.', 'success');
    } catch (err: unknown) {
      const msg =
        (err as any)?.response?.data?.description ||
        (err as any)?.response?.data?.message ||
        (err instanceof Error ? err.message : null) ||
        'Failed to delete project.';
      showSnackbar(msg, 'error');
    } finally {
      setIsDeleting(false);
      setDeleteTargetId(null);
    }
  };

  // Only show full-page loader on initial load (list is still empty)
  if (isProjectsLoading && projectsForCurrentOrganization.length === 0) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="center" sx={{ py: 6 }}>
          <AILoader />
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.loading.projects"
              defaultMessage={'Loading projects...'}
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth>
        <Typography color="error">{error}</Typography>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <PageTitle sx={{ mb: 2 }}>
          <PageTitle.Header>Projects</PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.subtitle"
              defaultMessage={
                'View all projects available in your organization.'
              }
            />
          </PageTitle.SubHeader>
          {hasProjects && newProjectPath ? (
            <PageTitle.Actions>
              <Button
                variant="contained"
                size="small"
                startIcon={<Plus size={16} />}
                component={RouterLink}
                to={newProjectPath}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.new.project"
                  defaultMessage="Add New Project"
                />
              </Button>
            </PageTitle.Actions>
          ) : null}
        </PageTitle>

        {hasProjects ? (
          <Grid size={{ xs: 12 }}>
            <TextField
              fullWidth
              placeholder="Search projects..."
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={20} />
                    </InputAdornment>
                  ),
                },
              }}
            />
          </Grid>
        ) : null}

        <Grid size={{ xs: 12 }}>
          <Box
            sx={{
              width: '100%',
              display: 'grid',
              gridTemplateColumns: {
                xs: '1fr',
                sm: 'repeat(2, minmax(0, 1fr))',
                md: 'repeat(4, minmax(0, 1fr))',
              },
              gap: 2,
            }}
          >
            {!hasProjects ? (
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gridColumn: '1 / -1',
                  py: 6,
                }}
              >
                <Stack
                  spacing={1.5}
                  alignItems="center"
                  justifyContent="center"
                  sx={{ textAlign: 'center', py: 2, width: '100%' }}
                >
                  <Box
                    component="img"
                    src={NoProjects}
                    alt="No projects"
                    sx={{ width: 140, maxWidth: '80%' }}
                  />
                  <Typography variant="h6" sx={{ fontWeight: 700 }}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.create.your.first.project"
                      defaultMessage="Create your first project"
                    />
                  </Typography>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ maxWidth: 420 }}
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.create.your.first.project.description"
                      defaultMessage="Set up a project to organize your APIs, gateways, and AI workspace resources in one place."
                    />
                  </Typography>
                  {newProjectPath ? (
                    <Button
                      variant="contained"
                      component={RouterLink}
                      to={newProjectPath}
                      startIcon={<Plus size={20} />}
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.create.project"
                        defaultMessage="Create Project"
                      />
                    </Button>
                  ) : null}
                </Stack>
              </Box>
            ) : pagedProjects.length === 0 ? (
              <Box sx={{ gridColumn: '1 / -1' }}>
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.no.projects.match.search"
                    defaultMessage={'No projects match your search.'}
                  />
                </Typography>
              </Box>
            ) : (
              pagedProjects.map((project) => (
                <Box key={project.id}>
                  <Card
                    sx={{
                      height: '100%',
                      width: '100%',
                      cursor: 'pointer',
                      paddingY: 1,
                      transition: 'transform 0.15s ease, box-shadow 0.15s ease',
                      '&.MuiCard-root:hover': {
                        boxShadow: 4,
                        transform: 'translateY(-2px)',
                      },
                      '&:focus-visible': {
                        outline: '2px solid',
                        outlineColor: 'primary.main',
                        outlineOffset: '2px',
                      },
                    }}
                    tabIndex={0}
                    role="button"
                    onClick={() => {
                      setCurrentProject(project);
                      navigate(
                        buildProjectPath(currentOrganization, project, '/home')
                      );
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setCurrentProject(project);
                        navigate(
                          buildProjectPath(currentOrganization, project, '/home')
                        );
                      }
                    }}
                  >
                    <Form.CardContent
                      sx={{
                        height: '100%',
                        width: '100%',
                        display: 'flex',
                        flexDirection: 'column',
                        p: 2,
                        '&:last-child': { pb: 2 },
                      }}
                    >
                      {/* Header: avatar + name + description */}
                      <Stack direction="row" spacing={1.5} alignItems="flex-start" minHeight={80} sx={{ minWidth: 0 }}>
                        <Avatar
                          sx={{
                            width: 44,
                            height: 44,
                            flexShrink: 0,
                            bgcolor: 'primary.light',
                            color: 'primary.contrastText',
                          }}
                        >
                          <Layers size={20} />
                        </Avatar>
                        <Box sx={{ minWidth: 0, overflow: 'hidden' }}>
                          <Typography
                            variant="h5" sx={{ fontWeight: 600 }}
                            noWrap
                          >
                            {project.displayName}
                          </Typography>
                          <Typography
                            variant="body2"
                            color="text.secondary"
                            fontSize="0.75rem"
                            sx={{ mb: 0.25, mt: 0.8 }}
                            noWrap
                          >
                            {project.description?.trim() || ''}
                          </Typography>
                        </Box>
                      </Stack>

                      {/* Footer: timestamp left, edit/delete right */}
                      <Stack
                        direction="row"
                        alignItems="center"
                        justifyContent="space-between"
                        sx={{ mt: 'auto' }}
                      >
                        <Stack direction="row" spacing={0.75} alignItems="center">
                          <Clock size={16} />
                          <Typography variant="body2" color="text.secondary">
                            {formatRelativeTime(
                              project.updatedAt || project.createdAt
                            )}
                          </Typography>
                        </Stack>

                        <Stack direction="row" spacing={0.5}>
                          <Tooltip title="Edit project">
                            <IconButton
                              size="small"
                              onClick={(e) => {
                                e.stopPropagation();
                                const path = buildOrgPath(
                                  currentOrganization,
                                  `/projects/${project.id}/edit`
                                );
                                if (path) navigate(path);
                              }}
                              aria-label="Edit project"
                            >
                              <Pencil size={15} />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Delete project">
                            <IconButton
                              size="small"
                              color="error"
                              onClick={(e) => {
                                e.stopPropagation();
                                setDeleteTargetId(project.id);
                              }}
                              aria-label="Delete project"
                            >
                              <Trash2 size={15} />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </Stack>
                    </Form.CardContent>
                  </Card>
                </Box>
              ))
            )}
          </Box>
        </Grid>

        {showPagination && (
          <Grid size={{ xs: 12 }}>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              spacing={1.5}
              alignItems="center"
              justifyContent="flex-end"
            >
              <Typography variant="body2" color="text.secondary">
                {startIndex + 1}-{endIndex} of {totalProjects}
              </Typography>

              <Stack direction="row" spacing={1} alignItems="center">
                <Button
                  size="small"
                  onClick={() => setPage(0)}
                  disabled={safePage === 0}
                >
                  First
                </Button>
                <IconButton
                  size="small"
                  onClick={() => setPage((prev) => Math.max(0, prev - 1))}
                  disabled={safePage === 0}
                  aria-label="Previous page"
                >
                  <ChevronLeft size={18} />
                </IconButton>
                <IconButton
                  size="small"
                  onClick={() =>
                    setPage((prev) => Math.min(pageCount - 1, prev + 1))
                  }
                  disabled={safePage >= pageCount - 1}
                  aria-label="Next page"
                >
                  <ChevronRight size={18} />
                </IconButton>
                <Button
                  size="small"
                  onClick={() => setPage(pageCount - 1)}
                  disabled={safePage >= pageCount - 1}
                >
                  Last
                </Button>
              </Stack>

              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectListView.items.per.page"
                    defaultMessage={'Items per page'}
                  />
                </Typography>
                <Select
                  size="small"
                  value={rowsPerPage}
                  onChange={(event) =>
                    setRowsPerPage(Number(event.target.value))
                  }
                >
                  {[10, 20, 50].map((value) => (
                    <MenuItem key={value} value={value}>
                      {value}
                    </MenuItem>
                  ))}
                </Select>
              </Stack>
            </Stack>
          </Grid>
        )}
      </Grid>

      {/* Delete confirmation dialog */}
      <Dialog
        open={Boolean(deleteTargetId)}
        onClose={() => !isDeleting && setDeleteTargetId(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete Project</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete{' '}
            <strong>{deleteTarget?.displayName ?? 'this project'}</strong>? This action
            cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            onClick={() => setDeleteTargetId(null)}
            disabled={isDeleting}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={handleDeleteConfirm}
            disabled={isDeleting}
            startIcon={isDeleting ? <span style={{ width: 16, height: 16, display: 'inline-block' }} /> : undefined}
          >
            {isDeleting ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}

export default function ProjectListView() {
  return (
    <ProjectsProvider>
      <ProjectListViewInner />
    </ProjectsProvider>
  );
}
