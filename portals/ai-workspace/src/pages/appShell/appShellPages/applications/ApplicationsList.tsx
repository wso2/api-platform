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

import React, { useEffect, useMemo, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  formatRelativeTime,
  useApplications,
} from '../../../../contexts/ApplicationsContext';
import {
  buildOrgPath,
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import type { Application } from '../../../../utils/types';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import NoApplications from '../../../../assets/images/NoApplications.svg';
import { getErrorCode } from '../../../../utils/apiError';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trim()}…`;
}

export default function ApplicationsList() {
  const navigate = useNavigate();
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
    isProjectsLoading,
  } = useAppShell();
  const {
    applications,
    isLoading,
    error,
    deleteApplication,
    refreshApplications,
  } = useApplications();
  const showSnackbar = useAIWorkspaceSnackbar();

  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const isProjectLevel = Boolean(effectiveProject?.id);

  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Application | null>(null);

  useEffect(() => {
    setSelectedProjectId('');
  }, [currentOrganization?.id]);

  const selectedProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => project.id === selectedProjectId
      ) ?? null,
    [projectsForCurrentOrganization, selectedProjectId]
  );

  const handleGoToProjectLevel = () => {
    if (!selectedProject || !currentOrganization?.id) return;
    setCurrentProject?.(selectedProject);
    navigate(
      buildProjectPath(currentOrganization, selectedProject, '/applications')
    );
  };

  const genAIApplications = useMemo(
    () => applications.filter((app) => app.type === 'genai'),
    [applications]
  );

  const filteredApplications = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return genAIApplications;

    return applications.filter((app) =>
      [app.displayName, app.description]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query)
    );
  }, [searchQuery, genAIApplications]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;

    try {
      await deleteApplication(deleteTarget.id);
      showSnackbar('Application deleted successfully.', 'success');
    } catch {
      showSnackbar('Failed to delete application. Please try again.', 'error');
    } finally {
      setDeleteTarget(null);
    }
  };

  const handleApplicationClick = (app: Application) => {
    navigate(
      isProjectLevel
        ? buildProjectPath(
            currentOrganization,
            effectiveProject,
            `/applications/${app.id}`
          )
        : buildOrgPath(currentOrganization, `/applications/${app.id}`)
    );
  };

  const newApplicationPath = isProjectLevel
    ? buildProjectPath(
        currentOrganization,
        effectiveProject,
        '/applications/new'
      )
    : buildOrgPath(currentOrganization, '/applications/new');

  const renderOrgLevelContent = () => (
    <Grid size={{ xs: 12, sm: 12, md: 7 }}>
      <Card sx={{ p: { xs: 2, sm: 3 } }}>
        <Stack spacing={2}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.project.level.message"
                defaultMessage="GenAI Applications are created and managed at the project level."
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.select.project"
                defaultMessage="Select a project to switch to project level and continue."
              />
            </Typography>
          </Box>

          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={1.5}
            alignItems={{ xs: 'stretch', sm: 'flex-end' }}
          >
            <FormControl fullWidth sx={{ maxWidth: 500 }}>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.project"
                  defaultMessage="Project"
                />
              </FormLabel>
              <Select
                value={isProjectsLoading ? '__loading__' : selectedProjectId}
                onChange={(event) =>
                  setSelectedProjectId(event.target.value as string)
                }
                displayEmpty
                disabled={
                  isProjectsLoading ||
                  !currentOrganization?.id ||
                  projectsForCurrentOrganization.length === 0
                }
                MenuProps={{ PaperProps: { sx: { maxHeight: 300 } } }}
              >
                {isProjectsLoading ? (
                  <MenuItem value="__loading__" disabled>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.loading.projects"
                      defaultMessage="Loading projects..."
                    />
                  </MenuItem>
                ) : projectsForCurrentOrganization.length === 0 ? (
                  <MenuItem value="" disabled>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.no.projects.available"
                      defaultMessage="No projects available"
                    />
                  </MenuItem>
                ) : (
                  projectsForCurrentOrganization.map((project) => (
                    <MenuItem key={project.id} value={project.id}>
                      {project.displayName}
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>

            <Button
              variant="contained"
              onClick={handleGoToProjectLevel}
              disabled={!selectedProject || isProjectsLoading}
              sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.go.to.project.level"
                defaultMessage="Go to Project Level"
              />
            </Button>
          </Stack>
        </Stack>
      </Card>
    </Grid>
  );

  const renderProjectHeader = () => (
    <Grid size={{ xs: 12 }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: { xs: 'flex-start', sm: 'center' },
          justifyContent: 'space-between',
          flexWrap: { xs: 'wrap', sm: 'nowrap' },
          gap: 2,
        }}
      >
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.genai.applications"
              defaultMessage="GenAI Applications"
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.manage.genai.applications"
              defaultMessage="Manage and monitor your GenAI applications."
            />
          </PageTitle.SubHeader>
        </PageTitle>

        {filteredApplications.length > 0 ? (
          <Button
            variant="contained"
            component={RouterLink}
            to={newApplicationPath}
            startIcon={<Plus size={20} />}
            sx={{ ml: 'auto', flexShrink: 0 }}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.add.new.application"
              defaultMessage="Add New Application"
            />
          </Button>
        ) : null}
      </Box>
    </Grid>
  );

  const renderProjectList = () => {
    if (isLoading) {
      return (
        <Grid size={{ xs: 12 }}>
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
            <CircularProgress />
          </Box>
        </Grid>
      );
    }

    if (error) {
      const is404ProjectNotFound = getErrorCode(error) === 'PROJECT_NOT_FOUND';

      if (is404ProjectNotFound) {
        return (
          <Grid size={{ xs: 12 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
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
                  src={NoApplications}
                  alt="No applications"
                  sx={{ width: 140, maxWidth: '80%' }}
                />
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.your.first.genai.application"
                    defaultMessage="Create your first GenAI Application"
                  />
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ maxWidth: 420 }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.setup.genai.application.description"
                    defaultMessage="Set up a GenAI application to securely consume AI services through your workspace."
                  />
                </Typography>
                <Button
                  variant="contained"
                  component={RouterLink}
                  to={newApplicationPath}
                  startIcon={<Plus size={20} />}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.application"
                    defaultMessage="Create Application"
                  />
                </Button>
              </Stack>
            </Box>
          </Grid>
        );
      }

      return (
        <Grid size={{ xs: 12 }}>
          <ErrorAlert error={error} onRetry={refreshApplications} />
        </Grid>
      );
    }

    return (
      <>
        {filteredApplications.length > 0 ? (
          <Grid size={{ xs: 12 }}>
            <TextField
              fullWidth
              placeholder={searchQuery ? undefined : 'Search Applications...'}
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
          <Grid container spacing={2}>
            {filteredApplications.length === 0 ? (
              <Grid size={{ xs: 12 }}>
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
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
                      src={NoApplications}
                      alt="No applications"
                      sx={{ width: 140, maxWidth: '80%' }}
                    />
                    <Typography variant="h6" sx={{ fontWeight: 700 }}>
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.your.first.genai.application"
                        defaultMessage="Create your first GenAI Application"
                      />
                    </Typography>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ maxWidth: 420 }}
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.setup.genai.application.description"
                        defaultMessage="Set up a GenAI application to securely consume AI services through your workspace."
                      />
                    </Typography>
                    <Button
                      variant="contained"
                      component={RouterLink}
                      to={newApplicationPath}
                      startIcon={<Plus size={20} />}
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.application"
                        defaultMessage="Create Application"
                      />
                    </Button>
                  </Stack>
                </Box>
              </Grid>
            ) : (
              filteredApplications.map((app) => {
                const descriptionText = app.description?.trim() || '';
                const lastUpdated =
                  app.lastUpdated ?? app.updatedAt ?? app.createdAt;

                return (
                  <Grid key={app.id} size={{ xs: 12, md: 4, lg: 3 }}>
                    <Card
                      sx={{
                        height: '100%',
                        width: '100%',
                        cursor: 'pointer',
                        transition: 'box-shadow 0.2s ease',
                        '&.MuiCard-root:hover': { boxShadow: 3 },
                        '&:focus-visible': {
                          outline: '2px solid',
                          outlineColor: 'primary.main',
                          outlineOffset: '2px',
                        },
                      }}
                      tabIndex={0}
                      role="button"
                      onClick={() => handleApplicationClick(app)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          handleApplicationClick(app);
                        }
                      }}
                    >
                      <Box
                        sx={{
                          height: '100%',
                          width: '100%',
                          display: 'flex',
                          flexDirection: 'column',
                          padding: 2,
                        }}
                      >
                        <Stack spacing={1.5} sx={{ flex: 1 }}>
                          <Box
                            sx={{
                              display: 'flex',
                              alignItems: 'flex-start',
                              gap: 1.5,
                            }}
                          >
                            <Avatar
                              sx={{
                                width: 44,
                                height: 44,
                                fontWeight: 600,
                                bgcolor: 'primary.light',
                                color: 'primary.contrastText',
                              }}
                            >
                              {getInitials(app.displayName)}
                            </Avatar>

                            <Box sx={{ flex: 1, minWidth: 0 }}>
                              <Typography variant="h5" sx={{ fontWeight: 600 }}>
                                {app.displayName}
                              </Typography>

                              <Typography
                                variant="body2"
                                color="text.secondary"
                                fontSize="0.75rem"
                                sx={{ mb: 0.25, mt: 0.8 }}
                              >
                                {truncateText(descriptionText, 70)}
                              </Typography>
                            </Box>
                          </Box>
                        </Stack>
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            gap: 2,
                          }}
                          mt="auto"
                          pt={2}
                        >
                          <Stack
                            direction="row"
                            spacing={0.5}
                            alignItems="center"
                          >
                            <Clock size={14} />
                            <Typography variant="body2" color="text.secondary">
                              {formatRelativeTime(lastUpdated)}
                            </Typography>
                          </Stack>

                          <IconButton
                            size="small"
                            color="error"
                            onClick={(event) => {
                              event.stopPropagation();
                              setDeleteTarget(app);
                            }}
                            aria-label={`Delete ${app.displayName}`}
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </Box>
                      </Box>
                    </Card>
                  </Grid>
                );
              })
            )}
          </Grid>
        </Grid>
      </>
    );
  };

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        {!isProjectLevel ? (
          renderOrgLevelContent()
        ) : (
          <>
            {renderProjectHeader()}
            {renderProjectList()}
          </>
        )}
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>Delete application</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete {deleteTarget?.displayName}?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setDeleteTarget(null)}
          >
            Cancel
          </Button>
          <Button color="error" onClick={handleDeleteConfirm}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
