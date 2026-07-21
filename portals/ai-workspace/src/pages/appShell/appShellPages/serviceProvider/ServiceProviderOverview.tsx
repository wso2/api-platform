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
import YAML from 'yaml';
import {
  Link as RouterLink,
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  PageContent,
  Select,
  Stack,
  Tab,
  Tabs,
  TextField,
  IconButton,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import serviceProviderOverviewMock from '../../../../data/serviceProviderOverviewMock';
import {
  LLMProviderContext,
  LLMProviderProvider,
  useLLMProvider,
  useLLMProviders,
  formatRelativeTime,
} from '../../../../contexts/llmProvider';
import { GatewayDeployProvider } from '../../../../contexts/GatewayDeployContext';
import ServiceProviderConnectionTab from './ServiceProviderConnectionTab';
import ServiceProviderOverviewTab from './ServiceProviderOverviewTab';
import ServiceProviderSecurityTab from './ServiceProviderSecurityTab';
import ServiceProviderRateLimitingTab from './ServiceProviderRateLimitingTab';
import ServiceProviderGuardrailsTab from './ServiceProviderGuardrailsTab';
import ServiceProviderModelsTab from './ServiceProviderModelsTab';
import ServiceProviderDeploymentsCard from './ServiceProviderDeploymentsCard';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { getGateways } from '../../../../apis/gatewayApis';
import {
  getLLMProviderDeployments,
  getLLMProviderProxies,
} from '../../../../apis/llmProviderApis';
import * as proxyApis from '../../../../apis/proxyApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { API_BASE_URLS, PLATFORM_API_BASE_URL } from '../../../../config.env';
import {
  getProviderTemplateDisplayName,
  truncateProviderDisplayName,
} from '../../../../utils/providerTemplateDisplay';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import {
  AIEntityProvider,
  useAIEntity,
} from '../../../../contexts/AIEntitiesContext';
import {
  DisabledActionTooltip,
  GatewayArtifactReadOnlyBanner,
} from '../../../../utils/readOnlyArtifacts';

import AnthropicLogo from '../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../assets/brands/openAI.png';
import {
  ChevronLeft,
  Clock,
  Edit,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import type {
  LLMProvider,
  ProjectBase,
  UpdateLLMProviderRequest,
} from '../../../../utils/types';
import logger from '../../../../utils/logger';
import LLLMStepBanner, {
  type LLLMStepBannerStepId,
} from '../quickStart/lllmStepBanner';
import ServiceProviderResourcesTab from './ServiceProviderResourcesTab';

const PROVIDER_LOGO_MAP: Record<string, string> = {
  openai: OpenAILogo,
  anthropic: AnthropicLogo,
  'azure-openai': AzureLogo,
  'azureai-foundry': AzureLogo,
  'aws-bedrock': AWSBedrockLogo,
  awsbedrock: AWSBedrockLogo,
  'google-vertex': GoogleVertexLogo,
  gemini: GoogleGeminiLogo,
  mistralai: MistralAILogo,
  mistral: MistralAILogo,
};
const TEMPLATE_LOGO_MAP: Record<string, string> = {
  openai: OpenAILogo,
  anthropic: AnthropicLogo,
  'azure-openai': AzureLogo,
  'azureai-foundry': AzureLogo,
  'aws-bedrock': AWSBedrockLogo,
  awsbedrock: AWSBedrockLogo,
  'google-vertex': GoogleVertexLogo,
  gemini: GoogleGeminiLogo,
  mistralai: MistralAILogo,
  mistral: MistralAILogo,
};

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

type OpenApiSpec = Record<string, unknown>;

function parseOpenApiSpec(text: string): OpenApiSpec | null {
  if (!text.trim()) return null;
  try {
    const jsonSpec = JSON.parse(text);
    return jsonSpec && typeof jsonSpec === 'object'
      ? (jsonSpec as OpenApiSpec)
      : null;
  } catch {
    try {
      const yamlSpec = YAML.parse(text);
      return yamlSpec && typeof yamlSpec === 'object'
        ? (yamlSpec as OpenApiSpec)
        : null;
    } catch (parseError) {
      logger.error('Failed to parse provider OpenAPI spec:', parseError);
      return null;
    }
  }
}

type LocationState = {
  providerAdded?: boolean;
};

type ProxyCreationNavigationState = {
  preselectedProviderId: string;
  preselectedProvider: LLMProvider;
  selectedProjectId?: string;
};

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

const tabs = [
  'Overview',
  'Connection',
  'Access Control',
  'Security',
  'Rate Limiting',
  'Guardrails & Policies',
  'Models',
];

type RateLimitingDraftActions = {
  saveDraftChanges: () => Promise<boolean>;
  discardDraftChanges: () => void;
};

const UNSAVED_CHANGES_MESSAGE =
  'You have unsaved changes. Please save or cancel before leaving this page.';

const stripReadOnlyProviderFields = (
  value: LLMProvider
): UpdateLLMProviderRequest => {
  const { status, createdAt, createdBy, updatedAt, updatedBy, lastUpdated, ...payload } =
    value;
  return payload;
};

function ServiceProviderOverviewContent() {
  const { refreshProviders } = useLLMProviders();
  const { refetch: refetchSelectedEntity } = useAIEntity();
  const {
    provider,
    isLoading,
    error,
    getProviderProxies,
    updateProvider,
    deleteProvider,
    getProviderAPIKeys,
    deleteProviderAPIKey,
    refetch,
  } = useLLMProvider();
  const navigate = useNavigate();
  const location = useLocation();
  const { hasPermission } = useAppAuth();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
    isProjectsLoading,
  } = useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const canDelete = !isProjectLevel && hasPermission(SCOPES.LLM_PROVIDER_DELETE);
  const providersPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/service-provider')
    : buildOrgPath(currentOrganization, '/service-provider');
  const createProxyPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/proxies/create')
    : buildOrgPath(currentOrganization, '/proxies/create');
  const [tabIndex, setTabIndex] = useState(0);
  const [draftProvider, setDraftProvider] = useState<LLMProvider | null>(null);
  const draftProviderRef = useRef<LLMProvider | null>(null);
  const [hasDraftChanges, setHasDraftChanges] = useState(false);
  const [isRateLimitingDirty, setIsRateLimitingDirty] = useState(false);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [rateLimitingActions, setRateLimitingActions] =
    useState<RateLimitingDraftActions | null>(null);
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [isGatewaysLoading, setIsGatewaysLoading] = useState(false);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [isProjectPickerDialogOpen, setIsProjectPickerDialogOpen] =
    useState(false);
  const [selectedProjectIdForProxy, setSelectedProjectIdForProxy] =
    useState('');
  const [orgProxyCount, setOrgProxyCount] = useState(0);
  const [latestGeneratedApiKey, setLatestGeneratedApiKey] = useState<
    string | null
  >(null);
  const [stepBannerRefreshTrigger, setStepBannerRefreshTrigger] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [deleteConfirmationInput, setDeleteConfirmationInput] = useState('');
  const [isDeletingProvider, setIsDeletingProvider] = useState(false);
  const [checkingProviderId, setCheckingProviderId] = useState<string | null>(
    null
  );
  const showSnackbar = useAIWorkspaceSnackbar();
  const hasUnsavedChanges = hasDraftChanges || isRateLimitingDirty;
  const selectedGateway = useMemo(
    () => gateways.find((gateway) => gateway.id === selectedGatewayId) ?? null,
    [gateways, selectedGatewayId]
  );
  const generatedInvokeUrl = useMemo(() => {
    const vhost = (selectedGateway?.endpoints?.[0] || selectedGateway?.vhost)?.trim();
    if (!vhost) return '';

    const normalizedBase = /^https?:\/\//i.test(vhost)
      ? vhost.replace(/\/+$/, '')
      : `https://${vhost.replace(/\/+$/, '')}`;
    const context = (provider?.context || '/').trim();
    const normalizedContext = context
      ? context.startsWith('/')
        ? context
        : `/${context}`
      : '/';
    return `${normalizedBase}${normalizedContext}`;
  }, [provider?.context, selectedGateway?.endpoints, selectedGateway?.vhost]);
  const selectedProjectForProxy = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => project.id === selectedProjectIdForProxy
      ) ?? null,
    [projectsForCurrentOrganization, selectedProjectIdForProxy]
  );
  const createProxyNavigationState = useMemo(
    () =>
      provider?.id
        ? ({
            preselectedProviderId: provider.id,
            preselectedProvider: provider,
          } as ProxyCreationNavigationState)
        : undefined,
    [provider]
  );
  const openApiSpecText = useMemo(
    () => (draftProvider ?? provider)?.openapi || '',
    [draftProvider, provider]
  );
  const parsedOpenApiSpec = useMemo(
    () => parseOpenApiSpec(openApiSpecText),
    [openApiSpecText]
  );
  const activeAccessControl = (draftProvider ?? provider)?.accessControl;
  const hasOpenApiSpecText = openApiSpecText.trim().length > 0;
  const apiKeyHeaderName = provider?.security?.apiKey?.key?.trim()
    ? provider.security.apiKey.key.trim()
    : 'X-API-Key';
  const swaggerDefaultHeaders = useMemo<Record<string, string> | undefined>(
    () =>
      latestGeneratedApiKey
        ? {
            [apiKeyHeaderName]: latestGeneratedApiKey,
          }
        : undefined,
    [apiKeyHeaderName, latestGeneratedApiKey]
  );

  const stageProviderUpdate = useCallback(
    async (updates: UpdateLLMProviderRequest): Promise<LLMProvider> => {
      let nextDraftProvider: LLMProvider | null = null;
      setDraftProvider((prev) => {
        const base = prev ?? draftProviderRef.current ?? provider;
        if (!base) return prev;
        nextDraftProvider = {
          ...base,
          ...updates,
        };
        draftProviderRef.current = nextDraftProvider;
        return nextDraftProvider;
      });

      if (!nextDraftProvider) {
        throw new Error('Provider is not loaded');
      }

      setHasDraftChanges(true);
      return nextDraftProvider;
    },
    [provider]
  );

  const handleCancelChanges = () => {
    if (isRateLimitingDirty) {
      rateLimitingActions?.discardDraftChanges();
    }
    setDraftProvider(provider);
    draftProviderRef.current = provider;
    setHasDraftChanges(false);
    setIsRateLimitingDirty(false);
  };

  const handleSaveChanges = async () => {
    if (!provider || isSavingChanges) return;

    let hasChangesToPersist = hasDraftChanges;
    if (isRateLimitingDirty) {
      const isRateLimitingStaged =
        (await rateLimitingActions?.saveDraftChanges()) ?? false;
      if (!isRateLimitingStaged) return;
      hasChangesToPersist = true;
    }

    if (!hasChangesToPersist) return;

    const stagedProvider = draftProviderRef.current ?? provider;
    setIsSavingChanges(true);
    try {
      await updateProvider(stripReadOnlyProviderFields(stagedProvider));
      await refetchSelectedEntity();
      setHasDraftChanges(false);
      setIsRateLimitingDirty(false);
      showSnackbar('Service provider changes saved successfully.', 'success');
    } catch {
      showSnackbar('Failed to save provider changes.', 'error');
    } finally {
      setIsSavingChanges(false);
    }
  };

  const handleBlockedNavigation = (event: React.MouseEvent<HTMLElement>) => {
    if (!hasUnsavedChanges) return;
    event.preventDefault();
    showSnackbar(UNSAVED_CHANGES_MESSAGE, 'error');
  };

  const handleDeployNavigation = () => {
    // DP-originated artifacts: the deployments page is viewable (actions are disabled
    // there), so navigate rather than blocking with a warning.
    if (hasUnsavedChanges) {
      showSnackbar(UNSAVED_CHANGES_MESSAGE, 'error');
      return;
    }
    navigate('deploy');
  };

  const handleTabChange = (_: React.SyntheticEvent, value: number) => {
    if (value !== tabIndex && hasUnsavedChanges) {
      showSnackbar(UNSAVED_CHANGES_MESSAGE, 'error');
      return;
    }
    setTabIndex(value);
  };

  const refreshOrgProxyCount = useCallback(async () => {
    if (!currentOrganization?.uuid || isProjectsLoading) {
      return;
    }

    if (!projectsForCurrentOrganization.length) {
      setOrgProxyCount(0);
      return;
    }

    try {
      const proxiesByProject = await Promise.all(
        projectsForCurrentOrganization.map((project) =>
          proxyApis.getProxies(
            currentOrganization.uuid,
            project.id,
            PLATFORM_API_BASE_URL
          )
        )
      );

      const totalProxyCount = proxiesByProject.reduce(
        (count, proxiesResponse) =>
          count + (proxiesResponse.count ?? proxiesResponse.list.length),
        0
      );

      setOrgProxyCount(totalProxyCount);
    } catch {
      setOrgProxyCount(0);
    }
  }, [
    currentOrganization?.uuid,
    isProjectsLoading,
    projectsForCurrentOrganization,
  ]);

  useEffect(() => {
    void refreshOrgProxyCount();
  }, [refreshOrgProxyCount]);

  const isProxyQuotaReached = false;
  const proxyQuotaTooltip =
    'You cannot create more App AI Proxies because your organization has reached the maximum limit of 5 proxies.';
  const isReadOnlyProvider = Boolean(provider?.readOnly);
  const createProxyTooltip = isProxyQuotaReached ? proxyQuotaTooltip : '';

  const handleCreateProxyClick = () => {
    if (!provider?.id || isProxyQuotaReached) {
      return;
    }

    if (currentProject?.id) {
      navigate(createProxyPath, {
        state: {
          ...(createProxyNavigationState || {}),
          selectedProjectId: currentProject.id,
        } as ProxyCreationNavigationState,
      });
      return;
    }

    setSelectedProjectIdForProxy(
      (prev) => prev || projectsForCurrentOrganization[0]?.id || ''
    );
    setIsProjectPickerDialogOpen(true);
  };

  const handleConfirmProjectForProxy = () => {
    if (
      !currentOrganization ||
      !selectedProjectForProxy ||
      !createProxyNavigationState
    ) {
      return;
    }

    setCurrentProject?.(selectedProjectForProxy);
    navigate(
      buildProjectPath(
        currentOrganization,
        selectedProjectForProxy,
        '/proxies/create'
      ),
      {
        state: {
          ...createProxyNavigationState,
          selectedProjectId: selectedProjectForProxy.id,
        } as ProxyCreationNavigationState,
      }
    );
    setIsProjectPickerDialogOpen(false);
  };

  const [highlightApiKeySection, setHighlightApiKeySection] = useState(false);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || isDeletingProvider) return;

    try {
      setIsDeletingProvider(true);
      await deleteProvider();
      await refreshProviders();
      showSnackbar('Provider deleted successfully.', 'success');
      setDeleteTarget(null);
      setDeleteConfirmationInput('');
      navigate(providersPath, { replace: true });
    } catch {
      showSnackbar('Failed to delete provider. Please try again.', 'error');
    } finally {
      setIsDeletingProvider(false);
    }
  };

  const checkProviderUsageAndConfirmDelete = async (
    providerId: string,
    providerName: string
  ) => {
    if (!currentOrganization?.uuid) {
      showSnackbar(
        'Unable to verify App AI Proxy usage because organization details are unavailable.',
        'error'
      );
      return;
    }

    setCheckingProviderId(providerId);
    try {
      const linkedProxiesResponse = await getLLMProviderProxies(
        providerId,
        currentOrganization.uuid,
        apimBaseUrl
      );
      const linkedProxyCount = linkedProxiesResponse.count ?? 0;
      if (linkedProxyCount > 0) {
        const proxyLabel = linkedProxyCount === 1 ? 'Proxy' : 'Proxies';
        const usageVerb = linkedProxyCount === 1 ? 'is' : 'are';
        showSnackbar(
          `Cannot delete "${providerName}" because ${linkedProxyCount} App LLM ${proxyLabel} ${usageVerb} using this provider. Remove or update those proxies first.`,
          'error'
        );
        return;
      }

      setDeleteTarget({ id: providerId, name: providerName });
      setDeleteConfirmationInput('');
    } catch {
      showSnackbar(
        'Failed to verify App AI Proxy usage for this provider. Deletion has been blocked. Please try again.',
        'error'
      );
    } finally {
      setCheckingProviderId(null);
    }
  };

  const handleLLLMStepBannerClick = (stepId: LLLMStepBannerStepId) => {
    if (stepId === 'add-guardrails') {
      setTabIndex(5);
    } else if (stepId === 'deploy-to-gateway') {
      // DP artifacts can still open the (read-only) deployments page.
      if (!provider?.id) return;
      const deployPath = isProjectLevel
        ? buildProjectPath(
            currentOrganization,
            currentProject,
            `/service-provider/${provider.id}/deploy`
          )
        : buildOrgPath(
            currentOrganization,
            `/service-provider/${provider.id}/deploy`
          );
      navigate(deployPath);
    } else if (stepId === 'consume') {
      setTabIndex(0);
      setHighlightApiKeySection(true);
      setTimeout(() => setHighlightApiKeySection(false), 3000);
    }
  };

  const draftContextValue = useMemo(
    () => ({
      provider: draftProvider ?? provider,
      isLoading,
      error,
      getProviderProxies,
      updateProvider: stageProviderUpdate,
      deleteProvider,
      getProviderAPIKeys,
      deleteProviderAPIKey,
      refetch,
      isDraftMode: true,
    }),
    [
      deleteProvider,
      deleteProviderAPIKey,
      draftProvider,
      error,
      getProviderAPIKeys,
      getProviderProxies,
      isLoading,
      provider,
      refetch,
      stageProviderUpdate,
    ]
  );

  useEffect(() => {
    const state = location.state as LocationState | null;
    if (state?.providerAdded) {
      showSnackbar('Successfully added new service provider.', 'success');
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location.pathname, location.state, navigate]);

  useEffect(() => {
    if (hasUnsavedChanges) return;
    setDraftProvider(provider);
    draftProviderRef.current = provider;
  }, [hasUnsavedChanges, provider]);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    const providerId = provider?.id;

    if (!organizationId || !providerId) {
      setGateways([]);
      setSelectedGatewayId('');
      setIsGatewaysLoading(false);
      return;
    }

    let isMounted = true;
    void (async () => {
      setIsGatewaysLoading(true);
      try {
        const [gatewaysResponse, deploymentsResponse] = await Promise.all([
          getGateways(organizationId),
          getLLMProviderDeployments(
            providerId,
            organizationId,
            PLATFORM_API_BASE_URL
          ),
        ]);
        if (!isMounted) return;

        const availableGateways = gatewaysResponse.list || [];
        const deployedEntries = (deploymentsResponse.list || []).filter(
          (deployment) => deployment.status === 'DEPLOYED'
        );

        if (availableGateways.length === 0 || deployedEntries.length === 0) {
          setGateways([]);
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

        const deployedGateways = availableGateways
          .filter((gateway) => latestDeploymentTimeByGateway.has(gateway.id))
          .sort((a, b) => {
            const timeA = latestDeploymentTimeByGateway.get(a.id) || 0;
            const timeB = latestDeploymentTimeByGateway.get(b.id) || 0;
            return timeB - timeA;
          });

        setGateways(deployedGateways);
        setSelectedGatewayId((currentSelectedId) => {
          if (
            currentSelectedId &&
            deployedGateways.some((gateway) => gateway.id === currentSelectedId)
          ) {
            return currentSelectedId;
          }
          return deployedGateways[0]?.id || '';
        });
      } catch (gatewayError) {
        if (!isMounted) return;
        logger.error(
          'Failed to fetch deployed gateways for invoke URL generation:',
          gatewayError
        );
        setGateways([]);
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
  }, [currentOrganization?.uuid, provider?.id]);

  useEffect(() => {
    setLatestGeneratedApiKey(null);
  }, [provider?.id]);

  useEffect(() => {
    if (!hasUnsavedChanges) return;
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [hasUnsavedChanges]);

  // Show loading state
  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 300,
          }}
        >
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  // Show error state
  if (error) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="flex-start" sx={{ minHeight: 300 }}>
          <Typography variant="h6" color="error">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.error.loading.provider"
              defaultMessage={'Error loading provider'}
            />
          </Typography>
          <Typography>{error.message}</Typography>
          <Button
            component={RouterLink}
            to={providersPath}
            onClick={handleBlockedNavigation}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.back.to.list"
              defaultMessage={'Back to list'}
            />
          </Button>
        </Stack>
      </PageContent>
    );
  }

  if (!provider) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="flex-start" sx={{ minHeight: 300 }}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.provider.not.found"
              defaultMessage={'Provider not found'}
            />
          </Typography>
          <Button
            component={RouterLink}
            to={providersPath}
            onClick={handleBlockedNavigation}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.back.to.list"
              defaultMessage={'Back to list'}
            />
          </Button>
        </Stack>
      </PageContent>
    );
  }

  const isAdminOrgLevel = hasPermission(SCOPES.LLM_PROVIDER_MANAGE) && !isProjectLevel;
  const { resources, rateLimiting, models } = serviceProviderOverviewMock;
  const tokenLimit = rateLimiting.backend.criteria.find(
    (item) => item.label === 'Token Count'
  );
  const tokenLimitText = tokenLimit
    ? `${tokenLimit.quota} tokens per ${tokenLimit.resetUnit}`
    : 'Not configured';
  const handleCopyInvokeUrl = async () => {
    if (!generatedInvokeUrl) return;
    try {
      await navigator.clipboard.writeText(generatedInvokeUrl);
      showSnackbar('URL copied to clipboard.', 'success');
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = generatedInvokeUrl;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };
  const providerKey = provider.id ?? provider.displayName;
  const providerDisplayName = truncateProviderDisplayName(provider.displayName);
  const logoUrl = PROVIDER_LOGO_MAP[providerKey];
  const hasLogo = Boolean(logoUrl);
  const templateDisplayName = getProviderTemplateDisplayName(provider.template);
  const templateKey = (provider.template ?? '').toLowerCase();
  const templateLogo = TEMPLATE_LOGO_MAP[templateKey];
  const hasTemplateLogo = Boolean(templateLogo);
  const descriptionText = provider.description?.trim() || '';
  const truncatedDescription =
    descriptionText.length > 200
      ? `${descriptionText.slice(0, 200).trim()}…`
      : descriptionText;
  const lastUpdated =
    provider.updatedAt ?? provider.lastUpdated ?? provider.createdAt;
  const modelProviders = provider.modelProviders ?? [];
  const providerModels = modelProviders.flatMap((modelProvider) =>
    (modelProvider.models ?? []).map((model) => ({
      model,
      providerId: modelProvider.id,
    }))
  );
  const providerDeleteAction = isAdminOrgLevel && canDelete ? (
    <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%' }}>
      <IconButton
        size="small"
        color="error"
        disabled={checkingProviderId === providerKey}
        onClick={() => {
          void checkProviderUsageAndConfirmDelete(
            providerKey,
            provider.displayName
          );
        }}
        aria-label={`Delete ${providerDisplayName}`}
        data-cyid="delete-provider-button"
      >
        <Trash2 size={16} />
      </IconButton>
    </Box>
  ) : null;
  const renderResourcesSpecViewer = () => {
    if (!hasOpenApiSpecText) {
      return (
        <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
          No available resources.
        </Typography>
      );
    }

    if (!parsedOpenApiSpec) {
      return (
        <Typography variant="body2" color="error" sx={{ py: 2 }}>
          Failed to parse the OpenAPI specification from provider response.
        </Typography>
      );
    }

    return (
      <SwaggerSpecViewer
        spec={parsedOpenApiSpec ?? {}}
        accessControl={activeAccessControl}
        requestBaseUrl={generatedInvokeUrl}
        defaultHeaders={swaggerDefaultHeaders}
        disableTryOutBtn={gateways.length === 0}
        hideInfoSection
        hideServers
        hideAuthorizeButton
        hideTagHeaders
        docExpansion="list"
        defaultModelsExpandDepth={-1}
        displayRequestDuration
        enableResourceSearch
      />
    );
  };
  const projectSelectionDialog = (
    <Dialog
      open={isProjectPickerDialogOpen}
      onClose={() => setIsProjectPickerDialogOpen(false)}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.title"
          defaultMessage="Create App AI Proxy"
        />
        <Typography variant="body2" color="text.secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.description"
            defaultMessage="Select a project to continue creating the App AI Proxy."
          />
        </Typography>
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.projects"
                defaultMessage="Projects"
              />
            </FormLabel>
            <Select
              value={
                isProjectsLoading ? '__loading__' : selectedProjectIdForProxy
              }
              onChange={(event) =>
                setSelectedProjectIdForProxy(event.target.value as string)
              }
              disabled={isProjectsLoading}
              data-cyid="proxy-project-select"
            >
              {isProjectsLoading ? (
                <MenuItem value="__loading__" disabled>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.loadingProjects"
                    defaultMessage="Loading projects..."
                  />
                </MenuItem>
              ) : projectsForCurrentOrganization.length === 0 ? (
                <MenuItem value="" disabled>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.noProjects"
                    defaultMessage="No projects available"
                  />
                </MenuItem>
              ) : (
                projectsForCurrentOrganization.map((project: ProjectBase) => (
                  <MenuItem key={project.id} value={project.id}>
                    {project.displayName}
                  </MenuItem>
                ))
              )}
            </Select>
          </FormControl>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => setIsProjectPickerDialogOpen(false)} variant="outlined"
            color="secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.cancel"
            defaultMessage="Cancel"
          />
        </Button>
        <Button
          variant="contained"
          onClick={handleConfirmProjectForProxy}
          disabled={!selectedProjectForProxy || isProjectsLoading}
          data-cyid="proxy-project-continue-button"
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.projectPicker.continue"
            defaultMessage="Continue"
          />
        </Button>
      </DialogActions>
    </Dialog>
  );
  const isDeleteConfirmationValid =
    deleteConfirmationInput.trim() === (deleteTarget?.name ?? '').trim();
  const deleteDialog = (
    <Dialog
      open={Boolean(deleteTarget)}
      onClose={() => {
        if (isDeletingProvider) return;
        setDeleteTarget(null);
        setDeleteConfirmationInput('');
      }}
    >
      <DialogTitle>
        Are you sure you want to remove the AI Provider{' '}
        <strong>'{deleteTarget?.name ?? ''}'</strong>?
      </DialogTitle>
      <DialogContent>
        <Typography sx={{ mt: 1 }} variant="body2" color="text.secondary">
          This action will be irreversible and all related details will be
          lost. Please type in the component name below to confirm.
        </Typography>
        <TextField
          fullWidth
          size="small"
          sx={{ mt: 2 }}
          value={deleteConfirmationInput}
          onChange={(event) => setDeleteConfirmationInput(event.target.value)}
          placeholder={deleteTarget?.name ?? 'Enter provider name'}
          data-cyid="delete-provider-confirm-input"
        />
      </DialogContent>
      <DialogActions>
        <Button
          variant="outlined"
          color="secondary"
          disabled={isDeletingProvider}
          onClick={() => {
            setDeleteTarget(null);
            setDeleteConfirmationInput('');
          }}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.cancel"
            defaultMessage={'Cancel'}
          />
        </Button>
        <Button
          color="error"
          onClick={handleDeleteConfirm}
          disabled={!isDeleteConfirmationValid || isDeletingProvider}
          data-cyid="delete-provider-confirm-button"
        >
          {isDeletingProvider ? (
            <CircularProgress size={20} />
          ) : (
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ProvidersList.delete"
              defaultMessage={'Delete'}
            />
          )}
        </Button>
      </DialogActions>
    </Dialog>
  );

  if (!isAdminOrgLevel) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={providersPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          onClick={handleBlockedNavigation}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.back.to.list"
            defaultMessage={'Back to list'}
          />
        </Button>
        <Card sx={{ mb: 2, mt: 1 }}>
          <Box>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'flex-start',
                justifyContent: 'space-between',
                flexWrap: 'wrap',
                gap: 2,
              }}
            >
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
                    src={hasLogo ? logoUrl : undefined}
                    sx={{
                      width: 72,
                      height: 72,
                      fontWeight: 600,
                      fontSize: 28,
                      bgcolor: hasLogo ? 'common.white' : 'primary.light',
                      color: hasLogo ? 'text.primary' : 'primary.contrastText',
                      border: hasLogo ? '1px solid' : 'none',
                      borderColor: 'divider',
                      p: hasLogo ? 0.5 : 0,
                      '& img': {
                        objectFit: 'contain',
                      },
                    }}
                  >
                    {!hasLogo ? getInitials(provider.displayName) : null}
                  </Avatar>
                  <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                    <Stack
                      direction="row"
                      spacing={1}
                      alignItems="center"
                      flexWrap="wrap"
                    >
                      {templateDisplayName ? (
                        <Chip
                          label={` ${templateDisplayName}`}
                          size="small"
                          variant="outlined"
                          color="primary"
                          sx={{ borderRadius: 0.5 }}
                          icon={
                            hasTemplateLogo ? (
                              <Avatar
                                src={templateLogo}
                                variant="circular"
                                sx={{
                                  width: 16,
                                  height: 16,
                                  '& img': { objectFit: 'contain' },
                                }}
                              />
                            ) : undefined
                          }
                        />
                      ) : null}
                    </Stack>
                    <Stack
                      direction="row"
                      spacing={1}
                      alignItems="center"
                      flexWrap="wrap"
                    >
                      <Typography variant="h3">
                        {providerDisplayName}
                      </Typography>
                      <Chip
                        label={`${provider.version || '1.0'}`}
                        size="small"
                        variant="outlined"
                        color="primary"
                      />
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {truncatedDescription}
                    </Typography>
                    <Stack spacing={0.25}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.context.label"
                          defaultMessage={'Context: {context}'}
                          values={{ context: provider.context || '/' }}
                        />
                      </Typography>
                      <Stack direction="row" spacing={0.75} alignItems="center">
                        <Typography variant="caption" color="text.secondary">
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.created"
                            defaultMessage="Last updated :"
                          />
                        </Typography>
                        <Clock size={14} />
                        <Typography variant="caption" color="text.secondary">
                          {lastUpdated ? formatRelativeTime(lastUpdated) : '—'}
                        </Typography>
                      </Stack>
                      {provider.createdBy && (
                        <Typography variant="caption" color="text.secondary">
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.createdBy"
                            defaultMessage="Created by: {createdBy}"
                            values={{ createdBy: provider.createdBy }}
                          />
                        </Typography>
                      )}
                    </Stack>
                  </Stack>
                </Box>
              </Box>

              <Stack
                spacing={2}
                p={2}
                sx={{ width: { xs: '100%', sm: 200 }, alignItems: 'stretch' }}
              >
                <Tooltip title={isProxyQuotaReached ? proxyQuotaTooltip : ''}>
                  <Box component="span">
                    <Button
                      variant="outlined"
                      onClick={handleCreateProxyClick}
                      disabled={!provider.id || isProxyQuotaReached}
                      data-cyid="create-app-llm-proxy-button"
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.create.llm.proxy"
                        defaultMessage="Create App AI Proxy"
                      />
                    </Button>
                  </Box>
                </Tooltip>
              </Stack>
            </Box>
          </Box>
        </Card>

        {/* Two-column layout for Resources and Deployments */}
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: gateways.length > 0 ? 6 : 12 }}>
            <Card sx={{ height: '100%' }}>
              <CardContent>
                <Stack spacing={3}>
                  <Box>
                    <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.resources"
                        defaultMessage={'Resources'}
                      />
                    </Typography>
                    <Box
                      sx={{
                        maxHeight: 350,
                        overflowY: 'auto',
                        border: '1px solid',
                        borderColor: 'divider',
                        borderRadius: 1,
                        bgcolor: 'background.paper',
                        pl: 2,
                        pr: 2,
                        pt: 2,
                      }}
                    >
                      {renderResourcesSpecViewer()}
                    </Box>
                  </Box>

                  {providerModels.length > 0 ? (
                    <>
                      <Divider />
                      <Box>
                        <Typography
                          variant="h6"
                          sx={{ mb: 1.5, fontWeight: 600 }}
                        >
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.models"
                            defaultMessage={'Models'}
                          />
                        </Typography>
                        <Box
                          sx={{
                            maxHeight: 200,
                            overflowY: 'auto',
                            p: 1,
                            pl: 2,
                            border: '1px solid',
                            borderColor: 'divider',
                            borderRadius: 1,
                            bgcolor: 'background.paper',
                          }}
                        >
                          <Stack
                            direction="row"
                            spacing={1}
                            sx={{ flexWrap: 'wrap', gap: 1 }}
                          >
                            {providerModels.map(({ model, providerId }) => (
                              <Box
                                key={`${providerId}:${model.id}`}
                                sx={{
                                  border: '1px solid',
                                  borderColor: '#000000',
                                  borderRadius: 0.5,
                                  px: 1.25,
                                  py: 0.75,
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  backgroundColor: '#fff',
                                  boxShadow: '0 1px 2px rgba(0,0,0,0.06)',
                                }}
                              >
                                <Typography
                                  variant="body2"
                                  color="primary.main"
                                >
                                  {model.displayName || model.id}
                                </Typography>
                              </Box>
                            ))}
                          </Stack>
                        </Box>
                      </Box>
                    </>
                  ) : null}
                </Stack>
              </CardContent>
            </Card>
          </Grid>

          {gateways.length > 0 ? (
            <Grid size={{ xs: 12, md: 6 }}>
              <ServiceProviderDeploymentsCard
                isGatewaysLoading={isGatewaysLoading}
                gateways={gateways}
                selectedGatewayId={selectedGatewayId}
                onGatewayChange={setSelectedGatewayId}
                generatedInvokeUrl={generatedInvokeUrl}
                onCopyInvokeUrl={handleCopyInvokeUrl}
                onLatestGeneratedApiKeyChange={setLatestGeneratedApiKey}
                onApiKeyCreated={() =>
                  setStepBannerRefreshTrigger((prev) => prev + 1)
                }
              />
            </Grid>
          ) : null}
        </Grid>
        {projectSelectionDialog}
        {deleteDialog}
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={providersPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
        onClick={handleBlockedNavigation}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.back.to.list"
          defaultMessage={'Back to list'}
        />
      </Button>
      <Box mt={1}>
        <LLLMStepBanner
          providerName={provider.displayName}
          onStepClick={handleLLLMStepBannerClick}
          refreshTrigger={stepBannerRefreshTrigger}
        />
      </Box>
      <Stack spacing={3} sx={{ mt: 2 }}>
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
                src={hasLogo ? logoUrl : undefined}
                sx={{
                  width: 72,
                  height: 72,
                  fontWeight: 600,
                  fontSize: 28,
                  bgcolor: hasLogo ? 'common.white' : 'primary.light',
                  color: hasLogo ? 'text.primary' : 'primary.contrastText',
                  border: hasLogo ? '1px solid' : 'none',
                  borderColor: 'divider',
                  p: hasLogo ? 0.5 : 0,
                  '& img': {
                    objectFit: 'contain',
                  },
                }}
              >
                {!hasLogo ? getInitials(provider.displayName) : null}
              </Avatar>
              <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  {templateDisplayName ? (
                    <Chip
                      label={` ${templateDisplayName}`}
                      size="small"
                      variant="outlined"
                      color="primary"
                      sx={{ borderRadius: 0.5 }}
                      icon={
                        hasTemplateLogo ? (
                          <Avatar
                            src={templateLogo}
                            variant="circular"
                            sx={{
                              width: 16,
                              height: 16,
                              '& img': { objectFit: 'contain' },
                            }}
                          />
                        ) : undefined
                      }
                    />
                  ) : null}
                </Stack>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  <Typography variant="h3">{providerDisplayName}</Typography>
                  <Chip
                    label={`${provider.version || '1.0'}`}
                    size="small"
                    variant="outlined"
                    color="primary"
                  />
                  {/* Edit page (name/version/context/description). Enabled even for
                      gateway-created providers — the page itself keeps the runtime
                      fields (name/version/context) read-only and allows only the
                      description, which is not part of the gateway runtime artifact. */}
                  <IconButton
                    component={RouterLink}
                    to="edit"
                    size="small"
                  >
                    <Edit size={16} />
                  </IconButton>
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {truncatedDescription}
                </Typography>
                <Stack spacing={0.25}>
                  <Typography variant="caption" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.context.label"
                      defaultMessage={'Context: {context}'}
                      values={{ context: provider.context || '/' }}
                    />
                  </Typography>
                  <Stack direction="row" spacing={0.75} alignItems="center">
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.created"
                        defaultMessage="Last updated :"
                      />
                    </Typography>
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary">
                      {lastUpdated ? formatRelativeTime(lastUpdated) : '—'}
                    </Typography>
                  </Stack>
                  {provider.createdBy && (
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.createdBy.2"
                        defaultMessage="Created by: {createdBy}"
                        values={{ createdBy: provider.createdBy }}
                      />
                    </Typography>
                  )}
                </Stack>
              </Stack>
            </Box>
            <Stack
              sx={{
                alignSelf: 'stretch',
                ml: 'auto',
                justifyContent: 'space-between',
                alignItems: 'stretch',
                width: { xs: '100%', sm: 200 },
              }}
            >
              <Stack spacing={1} sx={{ alignItems: 'stretch' }}>
                {/* For gateway-created (read-only) providers the deployments remain
                    viewable (deploy/redeploy/restore/undeploy are disabled on the page
                    itself), so the button navigates but is relabelled "View Deployments". */}
                <Button
                  variant="contained"
                  component={RouterLink}
                  to="deploy"
                  onClick={isReadOnlyProvider ? undefined : handleBlockedNavigation}
                  fullWidth
                >
                  {isReadOnlyProvider ? (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.view.deployments"
                      defaultMessage={'View Deployments'}
                    />
                  ) : (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.deploy.to.gateway"
                      defaultMessage={'Deploy to Gateway'}
                    />
                  )}
                </Button>
                <DisabledActionTooltip
                  disabled={isProxyQuotaReached}
                  title={createProxyTooltip}
                  fullWidth
                >
                  <Button
                    variant="outlined"
                    onClick={handleCreateProxyClick}
                    disabled={!provider.id || isProxyQuotaReached}
                    fullWidth
                  >
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.create.llm.proxy"
                      defaultMessage="Create App AI Proxy"
                    />
                  </Button>
                </DisabledActionTooltip>
              </Stack>
              {providerDeleteAction}
            </Stack>
          </Box>
        </Card>

        <Card>
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
          <LLMProviderContext.Provider value={draftContextValue}>
            <Box padding={2}>
              <TabPanel value={tabIndex} index={0}>
                <ServiceProviderOverviewTab
                  onApiKeyCreated={() =>
                    setStepBannerRefreshTrigger((prev) => prev + 1)
                  }
                  highlightApiKeySection={highlightApiKeySection}
                  onCreateProxy={handleCreateProxyClick}
                  onBlockedNavigation={handleDeployNavigation}
                />
              </TabPanel>

              <TabPanel value={tabIndex} index={1}>
                {isReadOnlyProvider && (
                  <GatewayArtifactReadOnlyBanner message="Connection settings are managed by the gateway that created this provider and are read-only here." />
                )}
                <ServiceProviderConnectionTab />
              </TabPanel>

              <TabPanel value={tabIndex} index={2}>
                {isReadOnlyProvider && (
                  <GatewayArtifactReadOnlyBanner message="Access control is managed by the gateway that created this provider and is read-only here." />
                )}
                <ServiceProviderResourcesTab />
              </TabPanel>

              <TabPanel value={tabIndex} index={3}>
                {isReadOnlyProvider && (
                  <GatewayArtifactReadOnlyBanner message="Security settings are managed by the gateway that created this provider and are read-only here." />
                )}
                <ServiceProviderSecurityTab />
              </TabPanel>

              <TabPanel value={tabIndex} index={4}>
                {isReadOnlyProvider && (
                  <GatewayArtifactReadOnlyBanner message="Rate limiting is managed by the gateway that created this provider and is read-only here." />
                )}
                <ServiceProviderRateLimitingTab
                  onDirtyChange={setIsRateLimitingDirty}
                  onActionsChange={setRateLimitingActions}
                />
              </TabPanel>

              <TabPanel value={tabIndex} index={5}>
                {isReadOnlyProvider && (
                  <GatewayArtifactReadOnlyBanner message="Guardrails & policies are managed by the gateway that created this provider and are read-only here." />
                )}
                <ServiceProviderGuardrailsTab />
              </TabPanel>

              <TabPanel value={tabIndex} index={6}>
                <ServiceProviderModelsTab />
              </TabPanel>
            </Box>
          </LLMProviderContext.Provider>
        </Card>

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
                  Save
                </Button>
              </Stack>
            </Stack>
          </Card>
        </Box>
      </Stack>
      {projectSelectionDialog}
      {deleteDialog}
    </PageContent>
  );
}

export default function ServiceProviderOverview() {
  const { providerId } = useParams<{ providerId: string }>();

  if (!providerId) {
    return (
      <PageContent fullWidth>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverview.provider.id.is.required"
            defaultMessage={'Provider ID is required'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <AIEntityProvider type="llm-provider" id={providerId}>
      <LLMProviderProvider providerId={providerId}>
        <GatewayDeployProvider apiId={providerId} resourceType="provider">
          <ServiceProviderOverviewContent />
        </GatewayDeployProvider>
      </LLMProviderProvider>
    </AIEntityProvider>
  );
}
