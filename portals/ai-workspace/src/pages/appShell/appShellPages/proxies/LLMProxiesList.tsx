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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  TablePagination,
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
import NoSearchResults from '../../../../Components/common/NoSearchResults';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { getErrorMessage } from '../../../../utils/apiError';
import { GatewayArtifactDeleteWarning } from '../../../../utils/readOnlyArtifacts';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { NO_PERMISSION_TOOLTIP, SCOPES } from '../../../../auth/permissions';
import * as proxyApis from '../../../../apis/proxyApis';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { logger } from '../../../../utils/logger';
import type { Proxy } from '../../../../utils/types';

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

// The backend hard-caps `limit` at 100, so there is no true "All" option once
// the list is paginated server-side.
const ROWS_PER_PAGE_OPTIONS = [20, 50, 100];

type LLMProxyListLocationState = {
  proxyDeleted?: boolean;
} | null;

export default function LLMProxiesList() {
  const navigate = useNavigate();
  const location = useLocation();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const { deleteProxy } = useProxies();
  const { providersResponse } = useLLMProviders();
  const emptyMessage = 'No Available App LLM Proxies';
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
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(20);
  const [selectedProjectId, setSelectedProjectId] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
    readOnly: boolean;
  } | null>(null);

  // Server-paginated/searched page of proxies — fetched directly by this page
  // rather than through ProxiesContext's `fetchAllPages`-backed cache, which
  // other consumers (proxy pickers, overview cards) still need in full.
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [pagination, setPagination] = useState({
    total: 0,
    offset: 0,
    limit: 20,
  });
  const [isProxiesLoading, setIsProxiesLoading] = useState(true);
  // True once the first fetch (success or failure) has completed. Prevents a
  // background refetch (e.g. from typing in search) from unmounting the
  // search box and table in favor of the full-page spinner, which would drop
  // input focus mid-keystroke.
  const [hasLoadedOnce, setHasLoadedOnce] = useState(false);
  const [proxiesError, setProxiesError] = useState<Error | null>(null);
  const [hasAnyProxies, setHasAnyProxies] = useState(false);
  const fetchProxiesRequestRef = useRef(0);
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  useEffect(() => {
    const handle = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery.trim());
    }, 350);
    return () => clearTimeout(handle);
  }, [searchQuery]);

  const fetchProxies = useCallback(async () => {
    const requestId = (fetchProxiesRequestRef.current += 1);
    const organizationId = currentOrganization?.uuid ?? '';
    const projectId = currentProject?.id ?? '';
    if (!organizationId || !projectId) {
      setProxies([]);
      setPagination({ total: 0, offset: 0, limit: rowsPerPage });
      setIsProxiesLoading(false);
      return;
    }

    try {
      setIsProxiesLoading(true);
      setProxiesError(null);
      const response = await proxyApis.getProxies(
        organizationId,
        projectId,
        apimBaseUrl,
        rowsPerPage,
        page * rowsPerPage,
        debouncedSearchQuery ? { search: debouncedSearchQuery } : undefined
      );
      if (fetchProxiesRequestRef.current !== requestId) return;
      setProxies(response.list ?? []);
      setPagination(
        response.pagination ?? { total: 0, offset: 0, limit: rowsPerPage }
      );
      if (!debouncedSearchQuery) {
        setHasAnyProxies((response.pagination?.total ?? 0) > 0);
      }
    } catch (err) {
      if (fetchProxiesRequestRef.current !== requestId) return;
      logger.error('Failed to fetch App LLM Proxies:', err);
      setProxiesError(
        err instanceof Error ? err : new Error('Failed to fetch proxies')
      );
    } finally {
      if (fetchProxiesRequestRef.current === requestId) {
        setIsProxiesLoading(false);
        setHasLoadedOnce(true);
      }
    }
  }, [
    currentOrganization?.uuid,
    currentProject?.id,
    apimBaseUrl,
    page,
    rowsPerPage,
    debouncedSearchQuery,
  ]);

  useEffect(() => {
    fetchProxies();
  }, [fetchProxies]);

  useEffect(() => {
    setSelectedProjectId('');
  }, [currentOrganization?.id]);

  // A stale search term carried across a project switch would otherwise keep
  // `hasAnyProxies` pinned to the previous project's value (it's only
  // recomputed from an unfiltered fetch) — reset the search box so the new
  // project's own "has any" state is captured on its first fetch.
  useEffect(() => {
    setSearchQuery('');
  }, [currentProject?.id]);

  useEffect(() => {
    const state = location.state as LLMProxyListLocationState;
    if (state?.proxyDeleted) {
      showSnackbar('Successfully deleted App LLM Proxy.', 'success');
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

  useEffect(() => {
    setPage(0);
  }, [debouncedSearchQuery, rowsPerPage]);

  // The list can shrink out from under the current page — a delete, or a
  // search/rowsPerPage change reducing the total — which must clamp `page`
  // back into range too, or the table requests a page past the end.
  useEffect(() => {
    const maxPage = Math.max(0, Math.ceil(pagination.total / rowsPerPage) - 1);
    if (page > maxPage) setPage(maxPage);
  }, [pagination.total, rowsPerPage, page]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await deleteProxy(deleteTarget.id);
      showSnackbar('Successfully deleted App LLM Proxy.', 'success');
      setDeleteTarget(null);
      await fetchProxies();
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to delete App LLM Proxy.'),
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
    'You cannot create more App LLM Proxies because your organization has reached the maximum limit of 5 proxies.';
  const canCreateProxy = hasPermission(SCOPES.LLM_PROXY_CREATE);
  const canDeleteProxy = hasPermission(SCOPES.LLM_PROXY_DELETE);
  const createProxyDisabled = !canCreateProxy || isProxyQuotaReached;
  const createProxyTooltip = !canCreateProxy
    ? NO_PERMISSION_TOOLTIP
    : isProxyQuotaReached
      ? proxyQuotaTooltip
      : '';
  const createProxyButtonSx = {
    opacity: createProxyDisabled ? 0.55 : 1,
    '&.Mui-disabled': {
      opacity: 0.55,
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
                <PageTitle.Header>App LLM Proxies</PageTitle.Header>
                <PageTitle.SubHeader>
                  Manage and monitor your App LLM Proxy deployments.
                </PageTitle.SubHeader>
              </PageTitle>

              <Stack
                direction="row"
                spacing={1.5}
                sx={{ ml: 'auto', flexShrink: 0 }}
              >
                {hasAnyProxies ? (
                  <Tooltip title={createProxyTooltip}>
                    <Box component="span">
                      <Button
                        variant="contained"
                        component={RouterLink}
                        to={newProxyPath}
                        startIcon={<Plus size={20} />}
                        disabled={createProxyDisabled}
                        sx={createProxyButtonSx}
                      >
                        Create App LLM Proxy
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
                        'App LLM Proxies are created and managed at the project level.'
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
        ) : isProxiesLoading && !hasLoadedOnce ? (
          <Grid size={{ xs: 12 }}>
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
              <CircularProgress />
            </Box>
          </Grid>
        ) : proxiesError && !isProxiesLoading && !isProxyNotFoundError ? (
          <Grid size={{ xs: 12 }}>
            <ErrorAlert error={proxiesError} onRetry={fetchProxies} />
          </Grid>
        ) : hasLoadedOnce && (!hasAnyProxies || isProxyNotFoundError) && !isProxiesLoading ? (
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
                    defaultMessage={'Create your first App LLM Proxy'}
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
                      'Set up an App LLM Proxy to route model traffic and manage AI access across your applications.'
                    }
                  />
                </Typography>
                <Tooltip title={createProxyTooltip}>
                  <Box component="span">
                    <Button
                      variant="contained"
                      component={RouterLink}
                      to={newProxyPath}
                      startIcon={<Plus size={20} />}
                      disabled={createProxyDisabled}
                      sx={createProxyButtonSx}
                    >
                      Create App LLM Proxy
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
                placeholder="Search App LLM Proxies..."
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

            <Grid
              size={{ xs: 12 }}
              sx={{
                opacity: isProxiesLoading ? 0.5 : 1,
                transition: 'opacity 0.15s ease',
                pointerEvents: isProxiesLoading ? 'none' : 'auto',
              }}
            >
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
                      {proxies.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6}>
                            <NoSearchResults
                              message={
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxiesList.no.llm.proxies.found"
                                  defaultMessage={'No App LLM Proxies match your search.'}
                                />
                              }
                            />
                          </TableCell>
                        </TableRow>
                      ) : (
                        proxies.map((proxy) => (
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
                              <Tooltip
                                title={
                                  canDeleteProxy ? '' : NO_PERMISSION_TOOLTIP
                                }
                              >
                                <Box component="span">
                                  <IconButton
                                    size="small"
                                    color="error"
                                    disabled={!canDeleteProxy}
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      setDeleteTarget({
                                        id: proxy.id,
                                        name: proxy.displayName,
                                        readOnly: Boolean(proxy.readOnly),
                                      });
                                    }}
                                    aria-label={`Delete ${proxy.displayName}`}
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

                {pagination.total >= ROWS_PER_PAGE_OPTIONS[0] && (
                  <TablePagination
                    component="div"
                    count={pagination.total}
                    page={page}
                    onPageChange={(_event, nextPage) => setPage(nextPage)}
                    rowsPerPage={rowsPerPage}
                    onRowsPerPageChange={(event) =>
                      setRowsPerPage(parseInt(event.target.value, 10))
                    }
                    rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
                  />
                )}
              </Card>
            </Grid>
          </>
        )}
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      >
        <DialogTitle>Delete App LLM Proxy</DialogTitle>
        <DialogContent>
          {deleteTarget?.readOnly ? (
            <GatewayArtifactDeleteWarning
              artifactType="App LLM Proxy"
              artifactName={deleteTarget.name}
            />
          ) : null}
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
