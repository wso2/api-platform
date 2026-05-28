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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  PageContent,
  Select,
  Skeleton,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Clock, Copy, Edit, ExternalLink } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { DEV_PORTAL_BASE_URL, DOMAIN, PLATFORM_API_BASE_URL } from '../../../../config.env';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import {
  checkMCPServerPublished,
  publishMCPServer,
  unpublishMCPServer,
} from '../../../../apis/MCP/mcpDevPortalApis';
import { getDevPortalBaseUrl } from '../../../../utils/devPortalUtils';import { getGuardrails } from '../../../../apis/policyHubApis';
import { getMCPServerDeployments } from '../../../../apis/MCP/mcpServerDeployApis';
import { getGateways } from '../../../../apis/gatewayApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import type { DeploymentResponse, MCPServer } from '../../../../utils/types';
import type { ParameterValues } from '../../PolicyParameterEditor/types';
import PolicyMapper from './PolicyMapper';
import type { SelectedPolicy } from './PolicyMapper';
import ExternalServersValidationDetails from './ExternalServersValidationDetails';
import type { EndpointValidationResponse } from './externalServersValidationTypes';
import ExternalServerStepBanner from '../quickStart/ExternalServerStepBanner';
import type { ExternalServerStepBannerStepId } from '../quickStart/ExternalServerStepBanner';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

function isNonArrayObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function pruneEmptyPolicyParamValues(value: unknown): unknown {
  if (typeof value === 'string') {
    const trimmed = value.trim();
    return trimmed === '' ? undefined : trimmed;
  }

  if (Array.isArray(value)) {
    const cleaned = value
      .map((item) => pruneEmptyPolicyParamValues(item))
      .filter((item) => {
        if (item === undefined || item === null) return false;
        if (typeof item === 'string') return item.trim() !== '';
        if (Array.isArray(item)) return item.length > 0;
        if (isNonArrayObject(item)) return Object.keys(item).length > 0;
        return true;
      });

    return cleaned.length > 0 ? cleaned : undefined;
  }

  if (isNonArrayObject(value)) {
    const cleaned = Object.entries(value).reduce<Record<string, unknown>>(
      (acc, [key, rawValue]) => {
        const normalizedValue = pruneEmptyPolicyParamValues(rawValue);

        if (normalizedValue === undefined || normalizedValue === null) {
          return acc;
        }
        if (
          typeof normalizedValue === 'string' &&
          normalizedValue.trim() === ''
        ) {
          return acc;
        }
        if (Array.isArray(normalizedValue) && normalizedValue.length === 0) {
          return acc;
        }
        if (
          isNonArrayObject(normalizedValue) &&
          Object.keys(normalizedValue).length === 0
        ) {
          return acc;
        }

        acc[key] = normalizedValue;
        return acc;
      },
      {}
    );

    return Object.keys(cleaned).length > 0 ? cleaned : undefined;
  }

  return value;
}

type TabPanelProps = {
  children: React.ReactNode;
  value: number;
  index: number;
};

function TabPanel({ children, value, index }: TabPanelProps): JSX.Element {
  return (
    <Box role="tabpanel" hidden={value !== index}>
      {value === index ? children : null}
    </Box>
  );
}

const TAB_LABELS = ['Overview', 'Policies'];
const UNSAVED_CHANGES_MESSAGE =
  'You have unsaved changes. Please save or cancel before leaving this page.';

export default function ExternalServersOverview(): JSX.Element {
  const { serverId, projectSlug } = useParams<{
    serverId: string;
    projectSlug: string;
  }>();
  const {
    currentOrganization,
    currentProject,
    projectsForCurrentOrganization,
  } = useAppShell();
  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const organizationId = currentOrganization?.uuid ?? '';
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/mcp-proxy'
  );

  const navigate = useNavigate();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [server, setServer] = useState<MCPServer | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [deployedGateways, setDeployedGateways] = useState<Gateway[]>([]);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [isGatewaysLoading, setIsGatewaysLoading] = useState(false);
  const [tabIndex, setTabIndex] = useState(0);
  const [selectedPolicies, setSelectedPolicies] = useState<SelectedPolicy[]>(
    []
  );

  // Publish to Developer Portal state
  const [isPublished, setIsPublished] = useState(false);
  const [isPublishStatusLoading, setIsPublishStatusLoading] = useState(false);
  const [isPublishActionLoading, setIsPublishActionLoading] = useState(false);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [isUnpublishConfirmOpen, setIsUnpublishConfirmOpen] = useState(false);
  const [publishDialogGatewayId, setPublishDialogGatewayId] = useState('');

  const selectedPoliciesRef = useRef<SelectedPolicy[]>([]);
  const [initialPolicies, setInitialPolicies] = useState<SelectedPolicy[]>([]);

  const updateSelectedPolicies = useCallback(
    (updater: React.SetStateAction<SelectedPolicy[]>) => {
      setSelectedPolicies((prev) => {
        const next =
          typeof updater === 'function'
            ? (updater as (prevState: SelectedPolicy[]) => SelectedPolicy[])(
                prev
              )
            : updater;
        selectedPoliciesRef.current = next;
        return next;
      });
    },
    []
  );

  useEffect(() => {
    if (!serverId || !organizationId) return;
    let cancelled = false;
    const fetchServer = async () => {
      try {
        setIsLoading(true);
        const response = await mcpProxiesApis.getMCPServer(
          serverId,
          organizationId,
          apimBaseUrl
        );
        if (!cancelled) {
          setServer(response);
        }
      } catch {
        // silently fail
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };
    fetchServer();
    return () => {
      cancelled = true;
    };
  }, [serverId, organizationId, apimBaseUrl]);

  // Fetch gateways and MCP server deployments for invoke URL
  useEffect(() => {
    if (!organizationId || !serverId) {
      setDeployedGateways([]);
      setSelectedGatewayId('');
      setIsGatewaysLoading(false);
      return;
    }

    let isMounted = true;
    void (async () => {
      setIsGatewaysLoading(true);
      try {
        const gatewaysResponse = await getGateways(organizationId);
        const availableGateways = gatewaysResponse.list || [];

        // Fetch deployments per gateway (same pattern as LLMProxyOverviewTab)
        const deploymentPromises = availableGateways.map((gateway) =>
          getMCPServerDeployments(
            serverId,
            organizationId,
            apimBaseUrl,
            gateway.id
          ).catch((error) => {
            logger.error(
              `Failed to fetch deployments for gateway ${gateway.id}:`,
              error
            );
            return { list: [] as DeploymentResponse[], count: 0 };
          })
        );

        const deploymentResponses = await Promise.all(deploymentPromises);
        if (!isMounted) return;

        const allDeployments = deploymentResponses.flatMap(
          (response) => response.list
        );
        const deployedEntries = allDeployments.filter(
          (deployment) => deployment.status === 'DEPLOYED'
        );

        if (availableGateways.length === 0 || deployedEntries.length === 0) {
          setDeployedGateways([]);
          setSelectedGatewayId('');
          return;
        }

        const latestDeploymentTimeByGateway = new Map<string, number>();
        deployedEntries.forEach((deployment) => {
          const nextTime = new Date(deployment.createdAt || 0).getTime();
          const currentTime = latestDeploymentTimeByGateway.get(
            deployment.gatewayId
          );
          if (currentTime === undefined || nextTime > currentTime) {
            latestDeploymentTimeByGateway.set(deployment.gatewayId, nextTime);
          }
        });

        const sortedDeployedGateways = availableGateways
          .filter((gateway) => latestDeploymentTimeByGateway.has(gateway.id))
          .sort((a, b) => {
            const timeA = latestDeploymentTimeByGateway.get(a.id) || 0;
            const timeB = latestDeploymentTimeByGateway.get(b.id) || 0;
            return timeB - timeA;
          });

        setDeployedGateways(sortedDeployedGateways);
        setSelectedGatewayId((currentSelectedId) => {
          if (
            currentSelectedId &&
            sortedDeployedGateways.some(
              (gateway) => gateway.id === currentSelectedId
            )
          ) {
            return currentSelectedId;
          }
          return sortedDeployedGateways[0]?.id || '';
        });
      } catch (gatewayError) {
        if (!isMounted) return;
        logger.error(
          'Failed to fetch deployed gateways for invoke URL generation:',
          gatewayError
        );
        setDeployedGateways([]);
        setSelectedGatewayId('');
      } finally {
        if (isMounted) {
          setIsGatewaysLoading(false);
        }
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [organizationId, serverId, apimBaseUrl]);

  const selectedGateway = useMemo(
    () =>
      deployedGateways.find((gateway) => gateway.id === selectedGatewayId) ??
      null,
    [deployedGateways, selectedGatewayId]
  );

  const generatedInvokeUrl = useMemo(() => {
    const vhost = selectedGateway?.vhost?.trim();
    if (!vhost) return '';

    const normalizedBase = /^https?:\/\//i.test(vhost)
      ? vhost.replace(/\/+$/, '')
      : `https://${vhost.replace(/\/+$/, '')}`;
    const context = (server?.context || '/').trim();
    const normalizedContext = context
      ? context.startsWith('/')
        ? context
        : `/${context}`
      : '/';
    return `${normalizedBase}${normalizedContext}`;
  }, [server?.context, selectedGateway?.vhost]);

  const handleCopyInvokeUrl = async () => {
    if (!generatedInvokeUrl) return;
    const fullUrl = `${generatedInvokeUrl}${generatedInvokeUrl.endsWith('/') ? 'mcp' : '/mcp'}`;
    try {
      await navigator.clipboard.writeText(fullUrl);
      showSnackbar('URL copied to clipboard.', 'success');
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = fullUrl;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  // Convert server.policies -> SelectedPolicy[] on load
  useEffect(() => {
    if (!server) return;
    const policies = (server.policies ?? []) as Array<{
      name: string;
      version: string;
      params?: Record<string, unknown>;
    }>;

    const mapPolicies = async () => {
      let guardrailPolicies: Array<{ name: string; displayName?: string }> = [];
      try {
        const response = await getGuardrails('MCP');
        guardrailPolicies = response.data ?? [];
      } catch {
        logger.error('Failed to fetch guardrail policies for display names');
      }

      const mapped: SelectedPolicy[] = policies.map((policy, index) => {
        const guardrail = guardrailPolicies.find((g) => g.name === policy.name);
        return {
          instanceId: `${policy.name}-${policy.version}-${index}-${Date.now()}`,
          policyId: policy.name,
          policyName: policy.name,
          displayName: guardrail?.displayName || policy.name,
          version: policy.version,
          params: policy.params ?? {},
        };
      });
      updateSelectedPolicies(mapped);
      setInitialPolicies(mapped);
    };

    void mapPolicies();
  }, [server, updateSelectedPolicies]);

  const mcpHubViewUrl = useMemo(() => {
    if (!isPublished || !server || !currentOrganization?.handle) return null;
    return `${DEV_PORTAL_BASE_URL.replace(/\/?$/, '')}/${encodeURIComponent(currentOrganization.handle)}/views/default/mcp/${encodeURIComponent(server.id)}`;
  }, [isPublished, server, currentOrganization?.handle]);

  const hasUnsavedChanges = useMemo(() => {
    if (selectedPolicies.length !== initialPolicies.length) return true;
    return selectedPolicies.some(
      (p, i) =>
        p.policyId !== initialPolicies[i]?.policyId ||
        p.version !== initialPolicies[i]?.version ||
        JSON.stringify(p.params) !== JSON.stringify(initialPolicies[i]?.params)
    );
  }, [selectedPolicies, initialPolicies]);

  // Check published status once server + deployments are both resolved
  useEffect(() => {
    if (isLoading || isGatewaysLoading || !server || !organizationId) return;

    const orgHandle = currentOrganization?.handle;
    if (!orgHandle) return;

    console.debug('[publish-check] DOMAIN:', DOMAIN, '| devPortalBaseUrl:', DEV_PORTAL_BASE_URL, '| orgHandle:', orgHandle, '| server.id:', server.id);

    let cancelled = false;
    void (async () => {
      setIsPublishStatusLoading(true);
      try {
        const published = await checkMCPServerPublished(
          DEV_PORTAL_BASE_URL,
          orgHandle,
          server.id
        );
        if (!cancelled) setIsPublished(published);
      } catch {
        // Non-blocking — leave isPublished as false
        if (!cancelled) setIsPublished(false);
      } finally {
        if (!cancelled) setIsPublishStatusLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isLoading, isGatewaysLoading, server, organizationId, currentOrganization?.handle]);

  const refreshPublishStatus = async () => {
    if (!server || !organizationId || !currentOrganization?.handle) return;
    console.debug('[publish-check refresh] DOMAIN:', DOMAIN, '| devPortalBaseUrl:', DEV_PORTAL_BASE_URL, '| orgHandle:', currentOrganization.handle, '| server.id:', server.id);
    try {
      const published = await checkMCPServerPublished(
        DEV_PORTAL_BASE_URL,
        currentOrganization.handle,
        server.id
      );
      setIsPublished(published);
    } catch {
      // ignore
    }
  };

  const handleOpenPublishDialog = () => {
    // Pre-select the gateway already chosen in the overview URL picker, or first deployed
    setPublishDialogGatewayId(selectedGatewayId || deployedGateways[0]?.id || '');
    setIsPublishDialogOpen(true);
  };

  const handleConfirmPublish = async () => {
    if (!server || !organizationId || !currentOrganization?.handle) return;
    const gateway = deployedGateways.find((g) => g.id === publishDialogGatewayId);
    if (!gateway) return;

    const vhost = gateway.vhost?.trim() ?? '';
    const normalizedBase = /^https?:\/\//i.test(vhost)
      ? vhost.replace(/\/+$/, '')
      : `https://${vhost.replace(/\/+$/, '')}`;
    const context = (server.context || '').trim().replace(/\/+$/, '');
    const normalizedContext = context ? (context.startsWith('/') ? context : `/${context}`) : '';
    const endpointUrl = `${normalizedBase}${normalizedContext}/mcp`;

    setIsPublishActionLoading(true);
    setIsPublishDialogOpen(false);
    try {
      await publishMCPServer(
        apimBaseUrl,
        server.id,
        organizationId,
        { orgHandle: currentOrganization.handle, remoteUrl: endpointUrl }
      );
      showSnackbar('MCP Proxy published to MCP Hub.', 'success');
      await refreshPublishStatus();
    } catch {
      showSnackbar('Failed to publish MCP Proxy.', 'error');
    } finally {
      setIsPublishActionLoading(false);
    }
  };

  const handleConfirmUnpublish = async () => {
    if (!server || !organizationId || !currentOrganization?.handle) return;
    setIsPublishActionLoading(true);
    setIsUnpublishConfirmOpen(false);
    try {
      await unpublishMCPServer(
        apimBaseUrl,
        server.id,
        organizationId,
        currentOrganization.handle
      );
      showSnackbar('MCP Proxy unpublished from MCP Hub.', 'success');
      await refreshPublishStatus();
    } catch {
      showSnackbar('Failed to unpublish MCP Proxy.', 'error');
    } finally {
      setIsPublishActionLoading(false);
    }
  };

  const handleCancelChanges = () => {
    updateSelectedPolicies(initialPolicies);
  };

  const handleSaveChanges = async () => {
    if (!server || !organizationId) return;
    const orderedPolicies = selectedPoliciesRef.current;

    // Convert selectedPolicies -> flat policy payload (preserve current UI order)
    const policiesPayload = orderedPolicies.map((sp) => {
      const normalizedParams = pruneEmptyPolicyParamValues(sp.params);

      return {
        name: sp.policyName,
        version: sp.version,
        ...(isNonArrayObject(normalizedParams)
          ? { params: normalizedParams }
          : {}),
      };
    });

    const { createdAt, updatedAt, ...updatePayload } = server;

    try {
      setIsSavingChanges(true);
      const updated = await mcpProxiesApis.updateMCPServer(
        server.id,
        { ...updatePayload, policies: policiesPayload },
        organizationId,
        apimBaseUrl
      );
      setServer(updated);
      showSnackbar('Policies saved successfully.', 'success');
    } catch {
      showSnackbar('Failed to save policies.', 'error');
    } finally {
      setIsSavingChanges(false);
    }
  };

  const handleStepBannerClick = (stepId: ExternalServerStepBannerStepId) => {
    if (stepId === 'add-policies') {
      setTabIndex(1);
    } else if (stepId === 'deploy-to-gateway') {
      navigate('deploy');
    } else if (stepId === 'publish-to-devportal') {
      if (isPublished && mcpHubViewUrl) {
        window.open(mcpHubViewUrl, '_blank', 'noopener,noreferrer');
      } else {
        handleOpenPublishDialog();
      }
    }
  };

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setTabIndex(newValue);
  };

  const handleBlockedNavigation = (event: React.MouseEvent<HTMLElement>) => {
    if (!hasUnsavedChanges) return;
    event.preventDefault();
    showSnackbar(UNSAVED_CHANGES_MESSAGE, 'error');
  };

  const handleAddPolicy = (policy: Omit<SelectedPolicy, 'instanceId'>) => {
    const nextItem: SelectedPolicy = {
      instanceId: `${policy.policyId}-${Date.now()}`,
      ...policy,
    };

    updateSelectedPolicies((prev) => [...prev, nextItem]);
  };

  const handleUpdatePolicy = (instanceId: string, params: ParameterValues) => {
    updateSelectedPolicies((prev) =>
      prev.map((policy) =>
        policy.instanceId === instanceId ? { ...policy, params } : policy
      )
    );
  };

  const handleRemovePolicy = (instanceId: string) => {
    updateSelectedPolicies((prev) =>
      prev.filter((policy) => policy.instanceId !== instanceId)
    );
  };

  const handleReorderPolicies = (
    draggedInstanceId: string,
    targetInstanceId: string
  ) => {
    updateSelectedPolicies((prev) => {
      const draggedIndex = prev.findIndex(
        (policy) => policy.instanceId === draggedInstanceId
      );
      const targetIndex = prev.findIndex(
        (policy) => policy.instanceId === targetInstanceId
      );

      if (draggedIndex === -1 || targetIndex === -1) {
        return prev;
      }

      const next = [...prev];
      const [movedPolicy] = next.splice(draggedIndex, 1);
      next.splice(targetIndex, 0, movedPolicy);
      return next;
    });
  };

  const validationResult: EndpointValidationResponse | null = useMemo(() => {
    if (!server?.capabilities) return null;
    return {
      endpointUrl: server.upstream?.main?.url ?? '',
      serverInfo: {
        name: server.name ?? '',
        version: server.version ?? '',
      },
      tools: (server.capabilities.tools ??
        []) as unknown as EndpointValidationResponse['tools'],
      resources: (server.capabilities.resources ??
        []) as unknown as EndpointValidationResponse['resources'],
      prompts: (server.capabilities.prompts ??
        []) as unknown as EndpointValidationResponse['prompts'],
    };
  }, [server]);

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Stack spacing={3} sx={{ mt: 2 }}>
          <Card>
            <Box
              sx={{ display: 'flex', alignItems: 'center', gap: 2, padding: 2 }}
            >
              <Skeleton variant="circular" width={72} height={72} />
              <Stack spacing={1} sx={{ flex: 1 }}>
                <Skeleton variant="text" width="40%" height={32} />
                <Skeleton variant="text" width="60%" height={20} />
                <Skeleton variant="text" width="30%" height={16} />
              </Stack>
            </Box>
          </Card>
        </Stack>
      </PageContent>
    );
  }

  if (!server) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.notFound"
              defaultMessage="MCP Proxy not found"
            />
          </Typography>
          <Button component={RouterLink} to={listPath}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.backToList"
              defaultMessage="Back to external servers"
            />
          </Button>
        </Stack>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={listPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        sx={{ px: 0, minWidth: 'auto' }}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.back"
          defaultMessage="Back to list"
        />
      </Button>

      <ExternalServerStepBanner
        serverName={server?.name}
        hasPolicies={(server?.policies?.length ?? 0) > 0}
        hasDeployments={deployedGateways.length > 0}
        isPublished={isPublished}
        devPortalUrl={mcpHubViewUrl}
        onStepClick={handleStepBannerClick}
      />

      <Stack spacing={3} sx={{ mt: 2, mb: 4 }}>
        {/* Top Card - Server Info */}
        <Card>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 2,
              padding: 2,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
              <Avatar
                sx={{
                  width: 72,
                  height: 72,
                  fontWeight: 600,
                  fontSize: 28,
                  bgcolor: 'primary.light',
                  color: 'primary.contrastText',
                }}
              >
                {getInitials(server.name)}
              </Avatar>
              <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  <Typography variant="h3">{server.name}</Typography>
                  <Chip
                    label={server.version}
                    size="small"
                    variant="outlined"
                    color="primary"
                  />
                  <Tooltip title="Edit MCP Proxy">
                    <IconButton component={RouterLink} to="edit" size="small">
                      <Edit size={16} />
                    </IconButton>
                  </Tooltip>
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {server.description}
                </Typography>
                <Stack spacing={0.2}>
                  <Stack direction="row" alignItems="center" gap={2}>
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.context.label"
                        defaultMessage="Context :"
                      />
                    </Typography>
                    <Typography variant="body2">
                      {server.context || '/'}
                    </Typography>
                  </Stack>
                  <Stack direction="row" spacing={0.75} alignItems="center">
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.created"
                        defaultMessage="Last updated :"
                      />
                    </Typography>
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary">
                      {formatRelativeTime(server.updatedAt)}
                    </Typography>
                  </Stack>
                </Stack>
              </Stack>
            </Box>
            <Stack
              spacing={1}
              sx={{ alignSelf: 'flex-start', ml: 'auto', gap: 1 }}
            >
              <Button
                variant="contained"
                component={RouterLink}
                to="deploy"
                onClick={handleBlockedNavigation}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.deployToGateway"
                  defaultMessage="Deploy to Gateway"
                />
              </Button>
              <Tooltip
                title={
                  deployedGateways.length === 0
                    ? 'Deploy to a gateway before publishing'
                    : ''
                }
              >
                <span>
                  <Button
                    variant="outlined"
                    color={isPublished && deployedGateways.length > 0 ? 'error' : 'primary'}
                    disabled={
                      deployedGateways.length === 0 ||
                      isPublishStatusLoading ||
                      isPublishActionLoading ||
                      isGatewaysLoading
                    }
                    onClick={
                      isPublished && deployedGateways.length > 0
                        ? () => setIsUnpublishConfirmOpen(true)
                        : handleOpenPublishDialog
                    }
                    startIcon={
                      (isPublishStatusLoading || isPublishActionLoading) ? (
                        <CircularProgress size={14} color="inherit" />
                      ) : undefined
                    }
                  >
                    {isPublished && deployedGateways.length > 0 ? 'Unpublish from MCP Hub' : 'Publish to MCP Hub'}
                  </Button>
                </span>
              </Tooltip>
              {isPublished && deployedGateways.length > 0 && mcpHubViewUrl ? (
                <Box
                  component="a"
                  href={mcpHubViewUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 0.5,
                    fontSize: '0.75rem',
                    color: 'primary.main',
                    textDecoration: 'none',
                    justifyContent: 'center',
                    '&:hover': { textDecoration: 'underline' },
                  }}
                >
                  View in MCP Hub <ExternalLink size={12} />
                </Box>
              ) : null}
            </Stack>
          </Box>
        </Card>

        {/* Tab Card - Overview & Policies */}
        <Card>
          <Tabs
            value={tabIndex}
            onChange={handleTabChange}
            variant="scrollable"
            allowScrollButtonsMobile
          >
            {TAB_LABELS.map((label) => (
              <Tab key={label} label={label} />
            ))}
          </Tabs>
          <Divider />
          <Box padding={2}>
            <TabPanel value={tabIndex} index={0}>
              {isGatewaysLoading ? (
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    mb: 1.5,
                  }}
                >
                  <CircularProgress size={16} />
                  <Typography variant="caption" color="text.secondary">
                    Loading gateways...
                  </Typography>
                </Box>
              ) : null}
              {deployedGateways.length > 0 ? (
                <Stack spacing={1.5} sx={{ mb: 2.5 }}>
                  <Box>
                    <Typography variant="h6" sx={{ fontWeight: 600 }}>
                      MCP Proxy URL
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Change the Gateway to generate the gateway specific URL
                      and add that to your MCP client to try this out.
                    </Typography>
                  </Box>
                  <Grid container spacing={1} alignItems="flex-end">
                    <Grid size={{ xs: 12, md: 2 }}>
                      <FormControl fullWidth>
                        <FormLabel>Gateways</FormLabel>
                        <Select
                          size="small"
                          value={selectedGatewayId}
                          onChange={(event) =>
                            setSelectedGatewayId(String(event.target.value))
                          }
                          displayEmpty
                          disabled={deployedGateways.length === 0}
                        >
                          {deployedGateways.map((gateway) => (
                            <MenuItem key={gateway.id} value={gateway.id}>
                              {gateway.displayName || gateway.name}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    </Grid>
                    <Grid size={{ xs: 12, md: 4 }}>
                      <FormControl fullWidth>
                        <FormLabel>URL</FormLabel>
                        <TextField
                          size="small"
                          fullWidth
                          value={
                            generatedInvokeUrl
                              ? `${generatedInvokeUrl}${
                                  generatedInvokeUrl.endsWith('/')
                                    ? 'mcp'
                                    : '/mcp'
                                }`
                              : ''
                          }
                          slotProps={{
                            input: {
                              readOnly: true,
                              endAdornment: (
                                <InputAdornment position="end">
                                  <Tooltip title="Copy URL" arrow>
                                    <span>
                                      <IconButton
                                        size="small"
                                        aria-label="Copy URL"
                                        onClick={() => {
                                          void handleCopyInvokeUrl();
                                        }}
                                        disabled={!generatedInvokeUrl}
                                      >
                                        <Copy size={16} />
                                      </IconButton>
                                    </span>
                                  </Tooltip>
                                </InputAdornment>
                              ),
                            },
                          }}
                        />
                      </FormControl>
                    </Grid>
                  </Grid>
                </Stack>
              ) : null}
              {validationResult ? (
                <ExternalServersValidationDetails
                  validationResult={validationResult}
                  showHeader={false}
                  showInputSchema
                />
              ) : (
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.overview.noValidation"
                    defaultMessage="No validation data available."
                  />
                </Typography>
              )}
            </TabPanel>

            <TabPanel value={tabIndex} index={1}>
              <PolicyMapper
                selectedPolicies={selectedPolicies}
                onAddPolicy={handleAddPolicy}
                onUpdatePolicy={handleUpdatePolicy}
                onRemovePolicy={handleRemovePolicy}
                onReorderPolicies={handleReorderPolicies}
                validationResult={validationResult}
              />
            </TabPanel>
          </Box>
        </Card>
      </Stack>

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
                {isSavingChanges ? 'Saving...' : 'Save'}
              </Button>
            </Stack>
          </Stack>
        </Card>
      </Box>

      {/* Publish: endpoint selection Dialog */}
      <Dialog
        open={isPublishDialogOpen}
        onClose={() => setIsPublishDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Publish to MCP Hub</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <Typography variant="body2" color="text.secondary">
              Select the gateway endpoint that clients will use to connect to
              this MCP proxy in the MCP Hub.
            </Typography>
            <FormControl fullWidth>
              <FormLabel>Gateway</FormLabel>
              <Select
                size="small"
                value={publishDialogGatewayId}
                onChange={(e) =>
                  setPublishDialogGatewayId(String(e.target.value))
                }
              >
                {deployedGateways.map((gateway) => (
                  <MenuItem key={gateway.id} value={gateway.id}>
                    {gateway.displayName || gateway.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            {publishDialogGatewayId ? (() => {
              const gw = deployedGateways.find(
                (g) => g.id === publishDialogGatewayId
              );
              const vhost = gw?.vhost?.trim() ?? '';
              const base = /^https?:\/\//i.test(vhost)
                ? vhost.replace(/\/+$/, '')
                : `https://${vhost.replace(/\/+$/, '')}`;
              const ctx = (server?.context || '').trim().replace(/\/+$/, '');
              const normalizedCtx = ctx ? (ctx.startsWith('/') ? ctx : `/${ctx}`) : '';
              const endpointUrl = `${base}${normalizedCtx}/mcp`;
              return (
                <FormControl fullWidth>
                  <FormLabel>Endpoint URL</FormLabel>
                  <TextField
                    size="small"
                    value={endpointUrl}
                    slotProps={{ input: { readOnly: true } }}
                  />
                </FormControl>
              );
            })() : null}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setIsPublishDialogOpen(false)}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={!publishDialogGatewayId || isPublishActionLoading}
            onClick={() => void handleConfirmPublish()}
          >
            Publish
          </Button>
        </DialogActions>
      </Dialog>

      {/* Unpublish: confirmation Dialog */}
      <Dialog
        open={isUnpublishConfirmOpen}
        onClose={() => setIsUnpublishConfirmOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Unpublish from MCP Hub</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Are you sure you want to unpublish{' '}
            <strong>{server?.name}</strong> from the MCP Hub? Clients
            will no longer be able to discover it.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setIsUnpublishConfirmOpen(false)}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            disabled={isPublishActionLoading}
            onClick={() => void handleConfirmUnpublish()}
          >
            Unpublish
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
