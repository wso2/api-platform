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
import {
  Link as RouterLink,
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  Grid,
  IconButton,
  PageContent,
  Stack,
  Tab,
  Tabs,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Edit, Trash2 } from '@wso2/oxygen-ui-icons-react';

import { formatRelativeTime } from './LLMProxyLayout';
import { ProxyProvider, useProxy } from '../../../../contexts/proxy';
import { useProxies } from '../../../../contexts/proxy';
import { useLLMProviders } from '../../../../contexts/llmProvider';
import LLMProxyProviderTab from './LLMProxyProviderTab';
import LLMProxyDefinitionTab from './LLMProxyDefinitionTab';
import LLMProxySecurityTab from './LLMProxySecurityTab';
import LLMProxyGuardrailsTab from './LLMProxyGuardrailsTab';
import LLMProxyOverviewTab from './LLMProxyOverviewTab';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { truncateProviderDisplayName } from '../../../../utils/providerTemplateDisplay';
import { FormattedMessage } from 'react-intl';
import type {
  Proxy as LLMProxy,
  UpdateProxyRequest,
} from '../../../../utils/types';
import {
  GatewayArtifactReadOnlyBanner,
} from '../../../../utils/readOnlyArtifacts';

type TabPanelProps = {
  value: number;
  index: number;
  children: React.ReactNode;
};

function TabPanel({ value, index, children }: TabPanelProps) {
  return (
    <Box role="tabpanel" hidden={value !== index} sx={{ pt: 2 }}>
      {value === index ? children : null}
    </Box>
  );
}

const tabs = ['Overview', 'Provider', 'Definition', 'Security', 'Guardrails & Policies'];
const UNSAVED_CHANGES_MESSAGE =
  'You have unsaved changes. Please save or cancel before leaving this page.';

function getErrorDescription(error: unknown, fallbackMessage: string): string {
  return (
    (error as any)?.response?.data?.description ||
    (error as any)?.response?.data?.message ||
    (error instanceof Error ? error.message : null) ||
    fallbackMessage
  );
}

type LLMProxyOverviewLocationState = {
  proxyAdded?: boolean;
} | null;

const normalizeProviderForComparison = (
  providerValue?: LLMProxy['provider']
) => {
  if (!providerValue) return undefined;
  if (typeof providerValue === 'string') {
    return { id: providerValue };
  }
  return {
    id: providerValue.id ?? '',
    auth: providerValue.auth,
  };
};

const normalizeProxyForComparison = (value: LLMProxy | null) => {
  if (!value) return null;
  return {
    ...value,
    vhost: value.vhost?.trim() || undefined,
    provider: normalizeProviderForComparison(value.provider),
  };
};

const buildProxyUpdatePayload = (value: LLMProxy): UpdateProxyRequest => {
  const { createdAt, updatedAt, ...payload } = value;
  const updatePayload: UpdateProxyRequest = {
    ...payload,
    vhost: payload.vhost?.trim() || undefined,
    provider: normalizeProviderForComparison(value.provider),
  };
  if (!updatePayload.vhost) {
    delete updatePayload.vhost;
  }
  delete (updatePayload as Record<string, unknown>).createdBy;
  return updatePayload;
};

/** Inner component that consumes ProxyProvider context */
function ProxyOverviewContent() {
  const {
    proxy,
    isLoading,
    error,
    setLocalProxy,
    updateProxy,
    deleteProxy: deleteProxyApi,
  } = useProxy();
  const { refreshProxies } = useProxies();
  const { providersResponse } = useLLMProviders();
  const showSnackbar = useAIWorkspaceSnackbar();
  const navigate = useNavigate();
  const location = useLocation();
  const { currentProject, currentOrganization } = useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const proxiesPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/proxies')
    : buildOrgPath(currentOrganization, '/proxies');

  const [tabIndex, setTabIndex] = useState(0);
  const [savedProxy, setSavedProxy] = useState<LLMProxy | null>(null);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const isReadOnlyProxy = Boolean(proxy?.readOnly);

  const getProviderId = (providerValue?: LLMProxy['provider']): string => {
    if (!providerValue) return '';
    if (typeof providerValue === 'string') return providerValue;
    return providerValue.id ?? '';
  };

  // Resolve provider display name
  const resolveProviderName = (
    providerValue?: LLMProxy['provider']
  ): string => {
    const providerId = getProviderId(providerValue);
    if (!providerId) return '\u2014';
    const found = providersResponse.list.find((p) => p.id === providerId);
    return found?.displayName ?? providerId;
  };

  useEffect(() => {
    const state = location.state as LLMProxyOverviewLocationState;
    if (state?.proxyAdded) {
      showSnackbar('Successfully created App LLM Proxy.', 'success');
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location.pathname, location.state, navigate, showSnackbar]);

  const hasUnsavedChanges = useMemo(() => {
    if (!proxy || !savedProxy) return false;
    return (
      JSON.stringify(normalizeProxyForComparison(proxy)) !==
      JSON.stringify(normalizeProxyForComparison(savedProxy))
    );
  }, [proxy, savedProxy]);

  useEffect(() => {
    if (!proxy) {
      setSavedProxy(null);
      return;
    }
    if (!hasUnsavedChanges) {
      setSavedProxy(proxy);
    }
  }, [proxy, hasUnsavedChanges]);

  const handleCancelChanges = () => {
    if (!savedProxy) return;
    setLocalProxy(savedProxy);
  };

  const handleTabChange = (_: React.SyntheticEvent, value: number) => {
    if (value !== tabIndex && hasUnsavedChanges) {
      showSnackbar(UNSAVED_CHANGES_MESSAGE, 'error');
      return;
    }
    setTabIndex(value);
  };

  const handleSaveChanges = async () => {
    // Runtime tabs (provider/security/guardrails & policies) are locked, so a save
    // from a gateway-created proxy only carries non-runtime edits (definition), which
    // the control plane accepts without altering the gateway runtime artifact.
    if (!proxy || !hasUnsavedChanges || isSavingChanges) {
      return;
    }
    try {
      setIsSavingChanges(true);
      setUpdateError(null);
      const updatedProxy = await updateProxy(buildProxyUpdatePayload(proxy));
      setSavedProxy(updatedProxy);
      showSnackbar('Proxy updated successfully.', 'success');
    } catch (err) {
      setUpdateError(
        err instanceof Error ? err.message : 'Failed to update proxy'
      );
    } finally {
      setIsSavingChanges(false);
    }
  };

  const handleDelete = async () => {
    if (!proxy) return;
    try {
      setIsDeleting(true);
      await deleteProxyApi();
      await refreshProxies();
      navigate(proxiesPath, {
        state: { proxyDeleted: true },
      });
    } catch (err) {
      showSnackbar(
        getErrorDescription(err, 'Failed to delete App LLM Proxy.'),
        'error'
      );
    } finally {
      setIsDeleting(false);
      setDeleteDialogOpen(false);
    }
  };

  // --- Loading / Error / Not Found states ---
  if (isLoading) {
    return (
      <PageContent>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent>
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load proxy. {error.message}
        </Alert>
        <Button component={RouterLink} to={proxiesPath}>
          Back to list
        </Button>
      </PageContent>
    );
  }

  if (!proxy) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.proxy.not.found"
              defaultMessage={'Proxy not found'}
            />
          </Typography>
          <Button component={RouterLink} to={proxiesPath}>
            Back to list
          </Button>
        </Stack>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={proxiesPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        Back to list
      </Button>

      <Stack spacing={3} sx={{ mt: 2 }}>
        {updateError && (
          <Alert severity="error" onClose={() => setUpdateError(null)}>
            {updateError}
          </Alert>
        )}

        {/* Header card with editable fields */}
        <Card>
          <Box sx={{ p: 2 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'stretch',
                justifyContent: 'space-between',
                gap: 2,
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: 2,
                  minWidth: 0,
                }}
              >
                <Avatar
                  color="secondary"
                  sx={{
                    width: 70,
                    height: 70,
                    backgroundColor: 'primary.light',
                    color: 'primary.contrastText',
                    fontSize: 32,
                  }}
                >
                  {(proxy.displayName || '\u2014').trim().slice(0, 2).toUpperCase()}
                </Avatar>

                <Box sx={{ minWidth: 0 }}>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    flexWrap="wrap"
                  >
                    <Typography variant="h3">
                      {truncateProviderDisplayName(proxy.displayName || '\u2014')}
                    </Typography>
                    <Chip
                      label={`${proxy.version || '1.0'}`}
                      size="small"
                      variant="outlined"
                      color="primary"
                    />
                    {/* Edit page (name/version/context/description). Enabled even
                        for gateway-created proxies — the page keeps the runtime
                        fields read-only and allows only the description. */}
                    <Tooltip title="Edit Proxy">
                      <IconButton
                        component={RouterLink}
                        to={`${proxiesPath}/${proxy.id}/edit`}
                        size="small"
                      >
                        <Edit size={16} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                  <Stack spacing={0.1} sx={{ mt: 1 }}>
                    <Stack direction="row" alignItems="center" gap={2}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.provider"
                          defaultMessage="Provider :"
                        />
                      </Typography>
                      <Typography variant="body2">
                        {resolveProviderName(proxy.provider)}
                      </Typography>
                    </Stack>
                    <Stack direction="row" alignItems="center" gap={2}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.context.label"
                          defaultMessage="Context :"
                        />
                      </Typography>
                      <Typography variant="body2">
                        {proxy.context || '/'}
                      </Typography>
                    </Stack>
                    <Stack direction="row" alignItems="center" gap={2}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.last.updated"
                          defaultMessage={'Last updated :'}
                        />
                      </Typography>
                      <Typography variant="body2">
                        {formatRelativeTime(proxy.updatedAt)}
                      </Typography>
                    </Stack>
                  </Stack>
                </Box>
              </Box>

              <Stack
                direction="column"
                justifyContent="space-between"
                alignItems="flex-end"
                sx={{ alignSelf: 'stretch' }}
              >
                {/* Deployments remain viewable for gateway-created proxies (deploy/
                    redeploy/restore/undeploy are disabled on the page itself), so the
                    button navigates but is relabelled "View Deployments". */}
                <Button
                  variant="contained"
                  component={RouterLink}
                  to={`${proxiesPath}/${proxy.id}/deploy`}
                >
                  {isReadOnlyProxy ? 'View Deployments' : 'Deploy to Gateway'}
                </Button>
                <IconButton
                  color="error"
                  onClick={() => setDeleteDialogOpen(true)}
                  aria-label="Delete proxy"
                >
                  <Trash2 size={16} />
                </IconButton>
              </Stack>
            </Box>
          </Box>
        </Card>

        {/* Two-column layout for Provider tab and Deployments */}
        <Grid container spacing={2} size={{ xs: 12 }}>
          <Grid size={12}>
            <Card sx={{ height: '100%' }}>
              <Tabs
                value={tabIndex}
                onChange={handleTabChange}
                variant="scrollable"
                allowScrollButtonsMobile
              >
                {tabs.map((label) => (
                  <Tab key={label} label={label} />
                ))}
              </Tabs>

              <Divider />

              <Box padding={2}>
                <TabPanel value={tabIndex} index={0}>
                  <LLMProxyOverviewTab />
                </TabPanel>

                <TabPanel value={tabIndex} index={1}>
                  {isReadOnlyProxy && (
                    <GatewayArtifactReadOnlyBanner message="The provider connection is managed by the gateway that created this proxy and is read-only here." />
                  )}
                  <LLMProxyProviderTab />
                </TabPanel>

                <TabPanel value={tabIndex} index={2}>
                  <LLMProxyDefinitionTab />
                </TabPanel>

                <TabPanel value={tabIndex} index={3}>
                  {isReadOnlyProxy && (
                    <GatewayArtifactReadOnlyBanner message="Security settings are managed by the gateway that created this proxy and are read-only here." />
                  )}
                  <LLMProxySecurityTab />
                </TabPanel>

                <TabPanel value={tabIndex} index={4}>
                  {isReadOnlyProxy && (
                    <GatewayArtifactReadOnlyBanner message="Guardrails & policies are managed by the gateway that created this proxy and are read-only here." />
                  )}
                  <LLMProxyGuardrailsTab />
                </TabPanel>
              </Box>
            </Card>
          </Grid>
        </Grid>

        <Box
          sx={{
            position: 'sticky',
            bottom: 0,
            zIndex: 10,
          }}
        >
          <Card>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              spacing={1}
              alignItems={{ xs: 'flex-start', sm: 'center' }}
              justifyContent="space-between"
              sx={{ p: 2 }}
            >
              <Typography
                variant="body2"
                color={hasUnsavedChanges ? 'warning.main' : 'text.secondary'}
              >
                {hasUnsavedChanges ? 'You have unsaved changes.' : ''}
              </Typography>
              <Stack direction="row" spacing={1}>
                <Button
                  variant="outlined"
                  color="secondary"
                  disabled={!hasUnsavedChanges || isSavingChanges}
                  onClick={handleCancelChanges}
                >
                  Cancel
                </Button>
                <Button
                  variant="contained"
                  disabled={!hasUnsavedChanges || isSavingChanges}
                  onClick={() => void handleSaveChanges()}
                >
                  {isSavingChanges ? <CircularProgress size={20} /> : 'Save'}
                </Button>
              </Stack>
            </Stack>
          </Card>
        </Box>
      </Stack>

      {/* Delete confirmation dialog */}
      <Dialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
      >
        <DialogTitle>Delete App LLM Proxy</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete <strong>{proxy.displayName}</strong>? This
            action cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setDeleteDialogOpen(false)}
          >
            Cancel
          </Button>
          <Button color="error" onClick={handleDelete} disabled={isDeleting}>
            {isDeleting ? <CircularProgress size={20} /> : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}

/** Wrapper that provides ProxyProvider based on route param */
export default function LLMProxyOverview() {
  const { proxyId } = useParams<{ proxyId: string }>();

  if (!proxyId) {
    return (
      <PageContent>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverview.no.proxy.id.specified"
            defaultMessage={'No proxy ID specified'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <ProxyProvider proxyId={proxyId}>
      <ProxyOverviewContent />
    </ProxyProvider>
  );
}
