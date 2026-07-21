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
import { Link as RouterLink, useLocation, useNavigate } from 'react-router-dom';
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
import { formatRelativeTime } from './LLMProxyLayout';
import { useProxies } from '../../../../contexts/proxy';
import { useLLMProviders } from '../../../../contexts/llmProvider';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { truncateProviderDisplayName } from '../../../../utils/providerTemplateDisplay';
import { FormattedMessage } from 'react-intl';
import NoProxies from '../../../../assets/images/NoProxies.svg';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage } from '../../../../utils/apiError';

function getHttpStatusCode(error?: Error | null): number | null {
  if (!error) return null;

  const axiosStatus = (error as any)?.response?.status;
  if (typeof axiosStatus === 'number') return axiosStatus;

  const match = error.message?.match(/status:\s*(\d{3})/i);
  if (match) return parseInt(match[1], 10);

  return null;
}

function getErrorDescription(error: unknown, fallbackMessage: string): string {
  return getErrorMessage(error, fallbackMessage);
}

type LLMProxyListLocationState = {
  proxyDeleted?: boolean;
} | null;

export default function LLMProxiesList() {
  const navigate = useNavigate();
  const location = useLocation();
  const showSnackbar = useAIWorkspaceSnackbar();
  const {
    proxiesResponse,
    isLoading: isProxiesLoading,
    error: proxiesError,
    deleteProxy,
    refreshProxies,
  } = useProxies();
  const { providersResponse } = useLLMProviders();
  const proxies = proxiesResponse.list;
  const emptyMessage = 'No Available App AI Proxies';
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
    isProjectsLoading,
  } = useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const newProxyPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/proxies/create')
    : buildOrgPath(currentOrganization, '/proxies/create');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);

  useEffect(() => {
    setSelectedProjectId('');
  }, [currentOrganization?.id]);

  useEffect(() => {
    const state = location.state as LLMProxyListLocationState;
    if (state?.proxyDeleted) {
      showSnackbar('Successfully deleted App AI Proxy.', 'success');
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location.pathname, location.state, navigate, showSnackbar]);

  const selectedProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => project.id === selectedProjectId
      ) ?? null,
    [projectsForCurrentOrganization, selectedProjectId]
  );

  const getProviderId = (provider: unknown): string => {
    if (!provider) return '';
    if (typeof provider === 'string') return provider;
    if (typeof provider === 'object' && 'id' in provider) {
      const providerId = (provider as { id?: unknown }).id;
      return typeof providerId === 'string' ? providerId : '';
    }
    return '';
  };

  // Resolve provider display name from real providers list
  const resolveProviderName = (providerRef?: unknown): string => {
    const providerId = getProviderId(providerRef);
    if (!providerId) return '—';
    const found = providersResponse.list.find((p) => p.id === providerId);
    return found?.displayName ?? providerId;
  };

  const truncateProxyDescription = (
    description?: string | null,
    maxLength = 60
  ): string => {
    const normalizedDescription = description?.trim() ?? '';
    if (!normalizedDescription) return '—';
    if (normalizedDescription.length <= maxLength) {
      return normalizedDescription;
    }
    return `${normalizedDescription.slice(0, maxLength).trim()}…`;
  };

  const filteredProxies = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return proxies;

    return proxies.filter((proxy) => {
      const haystack = [
        proxy.displayName,
        proxy.description,
        proxy.context,
        getProviderId(proxy.provider),
        proxy.version,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [proxies, searchQuery]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await deleteProxy(deleteTarget.id);
      showSnackbar('Successfully deleted App AI Proxy.', 'success');
      setDeleteTarget(null);
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to delete App AI Proxy.'),
        'error'
      );
      setDeleteTarget(null);
    }
  };

  const handleGoToProjectLevel = () => {
    if (!selectedProject || !currentOrganization?.id) return;
    setCurrentProject?.(selectedProject);
    navigate(
      buildProjectPath(currentOrganization, selectedProject, '/proxies')
    );
  };

  const isProxyQuotaReached = false;
  const proxyQuotaTooltip =
    'You cannot create more App AI Proxies because your organization has reached the maximum limit of 5 proxies.';
  const createProxyButtonSx = {
    opacity: isProxyQuotaReached ? 0.55 : 1,
    '&.Mui-disabled': {
      opacity: isProxyQuotaReached ? 0.55 : 1,
    },
  };
  const proxyErrorStatusCode = getHttpStatusCode(proxiesError);
  const isProxyNotFoundError = proxyErrorStatusCode === 404;

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          {isProjectLevel ? (
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
                <PageTitle.Header>App AI Proxies</PageTitle.Header>
                <PageTitle.SubHeader>
                  Manage and monitor your App AI Proxy deployments.
                </PageTitle.SubHeader>
              </PageTitle>

              <Stack
                direction="row"
                spacing={1.5}
                sx={{ ml: 'auto', flexShrink: 0 }}
              >
                {proxies.length > 0 ? (
                  <Tooltip title={isProxyQuotaReached ? proxyQuotaTooltip : ''}>
                    <Box component="span">
                      <Button
                        variant="contained"
                        component={RouterLink}
                        to={newProxyPath}
                        startIcon={<Plus size={20} />}
                        disabled={isProxyQuotaReached}
                        sx={createProxyButtonSx}
                      >
                        Create App AI Proxy
                      </Button>
                    </Box>
                  </Tooltip>
                ) : null}
              </Stack>
            </Box>
          ) : null}
        </Grid>

        {!isProjectLevel ? (
          <Grid size={{ xs: 12, sm: 7 }}>
            <Card sx={{ p: { xs: 2, sm: 3 } }}>
              <Stack spacing={2}>
                <Box>
                  <Typography variant="h6" sx={{ fontWeight: 600 }}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.llm.proxies.are.created.and.managed.at.the.project.level"
                      defaultMessage={
                        'App AI Proxies are created and managed at the project level.'
                      }
                    />
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.select.a.project.to.switch.to.project.level.and.continue"
                      defaultMessage={
                        'Select a project to switch to project level and continue.'
                      }
                    />
                  </Typography>
                </Box>

                <Stack
                  direction={{ xs: 'column', sm: 'row' }}
                  spacing={1.5}
                  alignItems={{ xs: 'stretch', sm: 'flex-end' }}
                >
                  <FormControl fullWidth sx={{ maxWidth: 500 }}>
                    <FormLabel>Project</FormLabel>
                    <Select
                      value={
                        isProjectsLoading ? '__loading__' : selectedProjectId
                      }
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
                          Loading projects...
                        </MenuItem>
                      ) : projectsForCurrentOrganization.length === 0 ? (
                        <MenuItem value="" disabled>
                          No projects available
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
                    Go to Project Level
                  </Button>
                </Stack>
              </Stack>
            </Card>
          </Grid>
        ) : isProxiesLoading ? (
          <Grid size={{ xs: 12 }}>
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
              <CircularProgress />
            </Box>
          </Grid>
        ) : proxiesError && !isProxyNotFoundError ? (
          <Grid size={{ xs: 12 }}>
            <ErrorAlert error={proxiesError} onRetry={refreshProxies} />
          </Grid>
        ) : proxies.length === 0 || isProxyNotFoundError ? (
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
                  src={NoProxies}
                  alt="No proxies"
                  sx={{ width: 140, maxWidth: '80%' }}
                />
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.create.your.first.llm.proxy"
                    defaultMessage={'Create your first App AI Proxy'}
                  />
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ maxWidth: 420 }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.setup.an.llm.proxy.description"
                    defaultMessage={
                      'Set up an App AI Proxy to route model traffic and manage AI access across your applications.'
                    }
                  />
                </Typography>
                <Tooltip title={isProxyQuotaReached ? proxyQuotaTooltip : ''}>
                  <Box component="span">
                    <Button
                      variant="contained"
                      component={RouterLink}
                      to={newProxyPath}
                      startIcon={<Plus size={20} />}
                      disabled={isProxyQuotaReached}
                      sx={createProxyButtonSx}
                    >
                      Create App AI Proxy
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
                placeholder="Search App AI Proxies..."
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
                        <TableCell>Service Provider</TableCell>
                        <TableCell>Version</TableCell>
                        <TableCell>Last Updated</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredProxies.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6}>
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.no.llm.proxies.found"
                                defaultMessage={'No App AI Proxies found.'}
                              />
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        filteredProxies.map((proxy) => (
                          <TableRow
                            key={proxy.id}
                            hover
                            sx={{ cursor: 'pointer' }}
                            onClick={() =>
                              navigate(
                                buildProjectPath(
                                  currentOrganization,
                                  currentProject,
                                  `/proxies/${proxy.id}`
                                )
                              )
                            }
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
                                  {(proxy.displayName || '—')
                                    .trim()
                                    .slice(0, 2)
                                    .toUpperCase()}
                                </Avatar>
                                <Box>
                                  <Typography
                                    variant="h6"
                                    sx={{ fontWeight: 600 }}
                                  >
                                    {truncateProviderDisplayName(proxy.displayName)}
                                  </Typography>
                                </Box>
                              </Box>
                            </TableCell>
                            <TableCell>
                              {truncateProxyDescription(proxy.description)}
                            </TableCell>
                            <TableCell>
                              {resolveProviderName(proxy.provider)}
                            </TableCell>
                            <TableCell>{proxy.version || '—'}</TableCell>
                            <TableCell>
                              {formatRelativeTime(proxy.updatedAt)}
                            </TableCell>
                            <TableCell align="right">
                              <IconButton
                                size="small"
                                color="error"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  setDeleteTarget({
                                    id: proxy.id,
                                    name: proxy.displayName,
                                  });
                                }}
                                aria-label={`Delete ${proxy.displayName}`}
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
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>Delete App AI Proxy</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete {deleteTarget?.name}?
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
