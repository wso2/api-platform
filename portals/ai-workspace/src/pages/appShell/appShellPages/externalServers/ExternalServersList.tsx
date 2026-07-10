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
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { formatRelativeTime } from '../proxies/LLMProxyLayout';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import type { MCPServer } from '../../../../utils/types';
import NoMCPServers from '../../../../assets/images/NoMCPServers.svg';
import { getErrorMessage } from '../../../../utils/apiError';

function getErrorDescription(error: unknown, fallbackMessage: string): string {
  return getErrorMessage(error, fallbackMessage);
}

export default function ExternalServersList(): React.JSX.Element {
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
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [isServersLoading, setIsServersLoading] = useState(false);
  const [hasFetchedServers, setHasFetchedServers] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<MCPServer | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';
  const projectId = effectiveProject?.id ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  useEffect(() => {
    if (!organizationId || !projectId) return;
    let cancelled = false;
    const fetchServers = async () => {
      try {
        setIsServersLoading(true);
        setHasFetchedServers(false);
        const response = await mcpProxiesApis.getMCPServers(
          projectId,
          apimBaseUrl
        );
        if (!cancelled) {
          setServers(response.list ?? []);
        }
      } catch {
        // silently fail on load
      } finally {
        if (!cancelled) {
          setIsServersLoading(false);
          setHasFetchedServers(true);
        }
      }
    };
    fetchServers();
    return () => {
      cancelled = true;
    };
  }, [organizationId, projectId, apimBaseUrl]);

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
      buildProjectPath(currentOrganization, selectedProject, '/mcp-proxy')
    );
  };

  const filteredServers = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return servers;

    return servers.filter((server) =>
      [server.displayName, server.description, server.context, server.version]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query)
    );
  }, [searchQuery, servers]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || !organizationId) return;
    const serverId = deleteTarget.id;
    try {
      await mcpProxiesApis.deleteMCPServer(serverId, apimBaseUrl);
      setServers((prev) => prev.filter((s) => s.id !== serverId));
      showSnackbar('MCP Proxy deleted successfully.', 'success');
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to delete MCP Proxy.'),
        'error'
      );
    }
    setDeleteTarget(null);
  };

  const handleServerRowClick = (server: MCPServer) => {
    navigate(
      buildProjectPath(
        currentOrganization,
        effectiveProject,
        `/mcp-proxy/${server.id}`
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
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.external.servers.are.created.and.managed.at.the.project.level"
                defaultMessage="MCP proxies are created and managed at the project level."
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.select.a.project.to.switch.to.project.level.and.continue"
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
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.project"
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
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.loading.projects"
                      defaultMessage="Loading projects..."
                    />
                  </MenuItem>
                ) : projectsForCurrentOrganization.length === 0 ? (
                  <MenuItem value="" disabled>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.no.projects.available"
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
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.go.to.project.level"
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
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.external.servers"
                defaultMessage="MCP Proxy"
              />
            </PageTitle.Header>
            <PageTitle.SubHeader>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.and.manage.mcp.servers.for.this.project"
                defaultMessage="Create and manage MCP proxies for this project."
              />
            </PageTitle.SubHeader>
          </PageTitle>

          {servers.length > 0 ? (
            <Button
              variant="contained"
              component={RouterLink}
              to={buildProjectPath(
                currentOrganization,
                effectiveProject,
                '/mcp-proxy/new'
              )}
              startIcon={<Plus size={20} />}
              sx={{ ml: 'auto', flexShrink: 0 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.external.server"
                defaultMessage="Create MCP Proxy"
              />
            </Button>
          ) : null}
        </Box>
      </Grid>

      {isServersLoading || !hasFetchedServers ? (
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
      ) : hasFetchedServers && servers.length === 0 ? (
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
                src={NoMCPServers}
                alt="No MCP proxies"
                sx={{ width: 140, maxWidth: '80%' }}
              />
              <Typography variant="h6" sx={{ fontWeight: 700 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.your.first.mcp.server"
                  defaultMessage="Create your first MCP Proxy"
                />
              </Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ maxWidth: 420 }}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.setup.an.mcp.server.description"
                  defaultMessage="Set up an MCP Proxy to expose tools, prompts, and resources through your AI gateway workflows."
                />
              </Typography>
              <Button
                variant="contained"
                component={RouterLink}
                to={buildProjectPath(
                  currentOrganization,
                  effectiveProject,
                  '/mcp-proxy/new'
                )}
                startIcon={<Plus size={20} />}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.external.server"
                  defaultMessage="Create MCP Proxy"
                />
              </Button>
            </Stack>
          </Box>
        </Grid>
      ) : (
        <>
          <Grid size={{ xs: 12 }}>
            <TextField
              fullWidth
              placeholder={searchQuery ? undefined : 'Search MCP Proxies...'}
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
                    {filteredServers.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6}>
                          <Typography variant="body2" color="text.secondary">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.no.mcp.servers.found"
                              defaultMessage="No MCP proxies found."
                            />
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      filteredServers.map((server) => (
                        <TableRow
                          key={server.id}
                          hover
                          onClick={() => handleServerRowClick(server)}
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
                                {(server.displayName || '—')
                                  .trim()
                                  .slice(0, 2)
                                  .toUpperCase()}
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
                                {server.displayName}
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
                            {server.description || '—'}
                          </TableCell>
                          <TableCell>{server.context || '—'}</TableCell>
                          <TableCell>{server.version || '—'}</TableCell>
                          <TableCell>
                            {formatRelativeTime(server.updatedAt)}
                          </TableCell>
                          <TableCell align="right">
                            <IconButton
                              size="small"
                              color="error"
                              onClick={(event) => {
                                event.stopPropagation();
                                setDeleteTarget(server);
                              }}
                              aria-label={`Delete ${server.displayName}`}
                            >
                              <Trash2 size={16} />
                            </IconButton>
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
        <DialogTitle>Delete external server</DialogTitle>
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
          <Button color="error" onClick={() => void handleDeleteConfirm()}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
