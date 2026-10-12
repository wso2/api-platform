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
  Skeleton,
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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useAgentProxies } from '../../../../contexts/agentProxy';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { formatRelativeTime } from '../proxies/LLMProxyLayout';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import type { AgentProxyListItem } from '../../../../utils/types';
import NoAgents from '../../../../assets/images/NoAgents.svg';
import { getErrorMessage } from '../../../../utils/apiError';
import { GatewayArtifactDeleteWarning } from '../../../../utils/readOnlyArtifacts';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { DISABLED_ACTION_SX, NO_PERMISSION_TOOLTIP, SCOPES } from '../../../../auth/permissions';

function getErrorDescription(error: unknown, fallbackMessage: string): string {
  return getErrorMessage(error, fallbackMessage);
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

export default function AgentProxiesList(): React.JSX.Element {
  const navigate = useNavigate();
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
    isProjectsLoading,
  } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const canCreateAgentProxy = hasPermission(SCOPES.AGENT_PROXY_CREATE);
  const canDeleteAgentProxy = hasPermission(SCOPES.AGENT_PROXY_DELETE);
  const createAgentProxyTooltip = canCreateAgentProxy ? '' : NO_PERMISSION_TOOLTIP;
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
  const {
    agentProxiesResponse,
    isLoading: isAgentProxiesLoading,
    deleteAgentProxy,
  } = useAgentProxies();
  const agentProxies = agentProxiesResponse.list;
  const [deleteTarget, setDeleteTarget] = useState<AgentProxyListItem | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

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
      buildProjectPath(currentOrganization, selectedProject, '/agent-proxy')
    );
  };

  const filteredAgentProxies = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return agentProxies;

    return agentProxies.filter((agentProxy) =>
      [
        agentProxy.displayName,
        agentProxy.description,
        agentProxy.context,
        agentProxy.version,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query)
    );
  }, [searchQuery, agentProxies]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || !organizationId) return;
    const agentProxyId = deleteTarget.id;
    try {
      await deleteAgentProxy(agentProxyId);
      showSnackbar('Agent Proxy deleted successfully.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to delete Agent Proxy.'),
        'error'
      );
    }
    setDeleteTarget(null);
  };

  const handleAgentProxyRowClick = (agentProxy: AgentProxyListItem) => {
    navigate(
      buildProjectPath(
        currentOrganization,
        effectiveProject,
        `/agent-proxy/${agentProxy.id}`
      )
    );
  };

  const renderOrgLevelContent = () => (
    <Grid size={{ xs: 12, sm: 12, md: 7 }}>
      <Card sx={{ p: { xs: 2, sm: 3 } }}>
        <Stack spacing={2}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.agent.proxies.are.created.and.managed.at.the.project.level"
                defaultMessage="Agent proxies are created and managed at the project level."
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.select.a.project.to.switch.to.project.level.and.continue"
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
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.project"
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
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.loading.projects"
                      defaultMessage="Loading projects..."
                    />
                  </MenuItem>
                ) : projectsForCurrentOrganization.length === 0 ? (
                  <MenuItem value="" disabled>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.no.projects.available"
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
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.go.to.project.level"
                defaultMessage="Go to Project Level"
              />
            </Button>
          </Stack>
        </Stack>
      </Card>
    </Grid>
  );

  const renderProjectList = () => (
    <>
      <Grid size={{ xs: 12 }}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            flexWrap: 'nowrap',
            gap: 2,
          }}
        >
          <PageTitle sx={{ minWidth: 0, flex: 1 }}>
            <PageTitle.Header>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.agent.proxies"
                defaultMessage="Agent Proxies"
              />
            </PageTitle.Header>
            <PageTitle.SubHeader>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.and.manage.agent.proxies.for.this.project"
                defaultMessage="Create and manage agent proxies for this project."
              />
            </PageTitle.SubHeader>
          </PageTitle>

          {agentProxies.length > 0 ? (
            <Tooltip title={createAgentProxyTooltip}>
              <Box component="span" sx={{ ml: 'auto', flexShrink: 0 }}>
                <Button
                  variant="contained"
                  component={RouterLink}
                  to={buildProjectPath(
                    currentOrganization,
                    effectiveProject,
                    '/agent-proxy/create'
                  )}
                  startIcon={<Plus size={20} />}
                  disabled={!canCreateAgentProxy}
                  sx={DISABLED_ACTION_SX}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.agent.proxy"
                    defaultMessage="Create Agent Proxy"
                  />
                </Button>
              </Box>
            </Tooltip>
          ) : null}
        </Box>
      </Grid>

      {isAgentProxiesLoading ? (
        <Grid size={{ xs: 12 }}>
          <Card>
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Name</TableCell>
                    <TableCell>Description</TableCell>
                    <TableCell>Context</TableCell>
                    <TableCell>Version</TableCell>
                    <TableCell>Last Updated</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {[...Array(3)].map((_, index) => (
                    <TableRow key={index}>
                      <TableCell>
                        <Box
                          sx={{ display: 'flex', alignItems: 'center', gap: 1 }}
                        >
                          <Skeleton variant="circular" width={36} height={36} />
                          <Skeleton variant="text" width="60%" />
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" />
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" />
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" width="50%" />
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" width="70%" />
                      </TableCell>
                      <TableCell align="right">
                        <Skeleton variant="circular" width={24} height={24} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </Card>
        </Grid>
      ) : agentProxies.length === 0 ? (
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
                src={NoAgents}
                alt="No agent proxies"
                sx={{ width: 140, maxWidth: '80%' }}
              />
              <Typography variant="h6" sx={{ fontWeight: 700 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.your.first.agent.proxy"
                  defaultMessage="Create your first agent proxy"
                />
              </Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ maxWidth: 420 }}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.setup.an.agent.proxy.description"
                  defaultMessage="Set up an Agent Proxy to expose skills, tasks, and messages through your AI gateway workflows."
                />
              </Typography>
              <Tooltip title={createAgentProxyTooltip}>
                <Box component="span">
                  <Button
                    variant="contained"
                    component={RouterLink}
                    to={buildProjectPath(
                      currentOrganization,
                      effectiveProject,
                      '/agent-proxy/create'
                    )}
                    startIcon={<Plus size={20} />}
                    disabled={!canCreateAgentProxy}
                    sx={DISABLED_ACTION_SX}
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.agent.proxy"
                      defaultMessage="Create Agent Proxy"
                    />
                  </Button>
                </Box>
              </Tooltip>
            </Stack>
          </Box>
        </Grid>
      ) : (
        <>
          <Grid size={{ xs: 12 }}>
            <TextField
              fullWidth
              placeholder={searchQuery ? undefined : 'Search Agent Proxies...'}
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

          <Grid size={{ xs: 12 }}>
            <Card>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Description</TableCell>
                      <TableCell>Context</TableCell>
                      <TableCell>Version</TableCell>
                      <TableCell>Last Updated</TableCell>
                      <TableCell align="right">Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {filteredAgentProxies.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6}>
                          <Typography variant="body2" color="text.secondary">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.no.agent.proxies.found"
                              defaultMessage="No agent proxies found."
                            />
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredAgentProxies.map((agentProxy) => (
                        <TableRow
                          key={agentProxy.id}
                          hover
                          onClick={() => handleAgentProxyRowClick(agentProxy)}
                          sx={{ cursor: 'pointer' }}
                        >
                          <TableCell sx={{ minWidth: 220 }}>
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 1,
                              }}
                            >
                              <Avatar
                                color="secondary"
                                sx={{
                                  width: 36,
                                  height: 36,
                                  backgroundColor: 'primary.light',
                                  color: 'primary.contrastText',
                                  fontSize: 16,
                                }}
                              >
                                {getInitials(agentProxy.displayName || '')}
                              </Avatar>
                              <Typography
                                variant="h6"
                                sx={{
                                  fontWeight: 600,
                                  maxWidth: 200,
                                  overflow: 'hidden',
                                  textOverflow: 'ellipsis',
                                  whiteSpace: 'nowrap',
                                }}
                              >
                                {agentProxy.displayName}
                              </Typography>
                            </Box>
                          </TableCell>
                          <TableCell
                            sx={{
                              maxWidth: 200,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {agentProxy.description || '—'}
                          </TableCell>
                          <TableCell>{agentProxy.context || '—'}</TableCell>
                          <TableCell>{agentProxy.version || '—'}</TableCell>
                          <TableCell>
                            {formatRelativeTime(agentProxy.updatedAt)}
                          </TableCell>
                          <TableCell align="right">
                            <Tooltip
                              title={
                                canDeleteAgentProxy ? '' : NO_PERMISSION_TOOLTIP
                              }
                            >
                              <Box component="span">
                                <IconButton
                                  size="small"
                                  color="error"
                                  disabled={!canDeleteAgentProxy}
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    setDeleteTarget(agentProxy);
                                  }}
                                  aria-label={`Delete ${agentProxy.displayName}`}
                                >
                                  <Trash2 size={16} />
                                </IconButton>
                              </Box>
                            </Tooltip>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Card>
          </Grid>
        </>
      )}
    </>
  );

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        {!isProjectLevel ? renderOrgLevelContent() : renderProjectList()}
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>Delete agent proxy</DialogTitle>
        <DialogContent>
          {deleteTarget?.readOnly ? (
            <GatewayArtifactDeleteWarning
              artifactType="Agent Proxy"
              artifactName={deleteTarget.displayName}
            />
          ) : null}
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
          <Button color="error" onClick={() => void handleDeleteConfirm()}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
