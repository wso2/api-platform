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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  Dialog,
  DialogActions,
  DialogContent,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  ListingTable,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
  DialogTitle,
} from '@wso2/oxygen-ui';
import { Copy, Trash2 } from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { getGateways } from '../../../../apis/gatewayApis';
import {
  createLLMProviderAPIKey,
  deleteLLMProviderAPIKey,
  getLLMProviderDeployments,
  getLLMProviderProxies,
} from '../../../../apis/llmProviderApis';
import { getLLMProxyDeployments } from '../../../../apis/llmProxiesApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { logger } from '../../../../utils/logger';
import type {
  DeploymentResponse,
  Proxy,
  UserAPIKey,
} from '../../../../utils/types';
import NoData from '../../../../assets/images/NoData.svg';
import { FormattedMessage } from 'react-intl';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';
import { buildProjectPath } from '../../../../utils/projectRouting';
import {
  formatPrefixedKey,
  resolveApiKeyAuthDisplay,
} from '../../../../utils/apiKeyAuthDisplay';
import {
  DisabledActionTooltip,
  GATEWAY_MANAGED_ARTIFACT_TOOLTIP,
} from '../../../../utils/readOnlyArtifacts';
import {
  getCurrentDeployment,
  getProxyIdentifier,
} from './ProviderMap/ProviderMapTab';
import ResourceDrawerCards from './ResourceDrawerCards';
import ApiTryOutCurlSnippet from '../../../../Components/common/ApiTryOutCurlSnippet';

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

function formatDate(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleDateString();
}

function buildApiKeyResourceName(displayName: string): string {
  const normalizedDisplayName = displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalizedDisplayName || 'api-key';
}

type ServiceProviderOverviewTabProps = {
  onApiKeyCreated?: () => void;
  highlightApiKeySection?: boolean;
  onCreateProxy?: () => void;
  onBlockedNavigation?: () => void;
};

export default function ServiceProviderOverviewTab({
  onApiKeyCreated,
  highlightApiKeySection,
  onCreateProxy,
  onBlockedNavigation,
}: ServiceProviderOverviewTabProps) {
  const { provider, getProviderAPIKeys } = useLLMProvider();
  const { currentOrganization, projectsForCurrentOrganization } = useAppShell();
  const navigate = useNavigate();
  const fetchedApiKeysProviderIdRef = useRef<string | null>(null);
  const fetchingApiKeysProviderIdRef = useRef<string | null>(null);
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [gatewayDeployments, setGatewayDeployments] = useState<
    Record<string, DeploymentResponse>
  >({});
  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [proxyDeployments, setProxyDeployments] = useState<
    Record<string, DeploymentResponse>
  >({});
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [generatingKey, setGeneratingKey] = useState(false);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [latestGeneratedKey, setLatestGeneratedKey] = useState<string | null>(
    null
  );
  const [isApiKeyModalOpen, setIsApiKeyModalOpen] = useState(false);
  const [apiKeyDisplayName, setApiKeyDisplayName] = useState('');
  const [keyError, setKeyError] = useState<string | null>(null);
  const [deleteTargetKeyName, setDeleteTargetKeyName] = useState<string | null>(
    null
  );
  const [isDeletingKey, setIsDeletingKey] = useState(false);
  const [apiKeys, setApiKeys] = useState<UserAPIKey[]>([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const {
    headerName: apiKeyName,
    location: apiKeyLocation,
    valuePrefix: apiKeyValuePrefix,
  } = useMemo(
    () => resolveApiKeyAuthDisplay(provider?.security, provider?.globalPolicies),
    [provider?.security, provider?.globalPolicies]
  );
  const showSnackbar = useAIWorkspaceSnackbar();
  const isReadOnlyProvider = Boolean(provider?.readOnly);

  const parsedOpenApiSpec = useMemo(
    () => parseOpenApiSpec(provider?.openapi || ''),
    [provider?.openapi]
  );
  const selectedGateway = useMemo(
    () => gateways.find((gateway) => gateway.id === selectedGatewayId) ?? null,
    [gateways, selectedGatewayId]
  );
  const generatedGatewayUrl = useMemo(() => {
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
  const swaggerSpecWithGatewayServer = useMemo<OpenApiSpec>(() => {
    const baseSpec = parsedOpenApiSpec;
    if (!baseSpec) return {};
    if (!generatedGatewayUrl) return baseSpec;

    const existingServers = Array.isArray(baseSpec.servers)
      ? baseSpec.servers.filter(
          (server): server is Record<string, unknown> =>
            typeof server === 'object' && server !== null
        )
      : [];
    const nonDuplicateServers = existingServers.filter(
      (server) => String(server.url ?? '') !== generatedGatewayUrl
    );

    return {
      ...baseSpec,
      servers: [{ url: generatedGatewayUrl }, ...nonDuplicateServers],
    };
  }, [generatedGatewayUrl, parsedOpenApiSpec]);
  const swaggerDefaultHeaders = useMemo<
    Record<string, string> | undefined
  >(() => {
    if (!latestGeneratedKey) return undefined;
    const resolvedApiKeyHeaderName = apiKeyName.trim() || 'X-API-Key';

    return {
      [resolvedApiKeyHeaderName]: formatPrefixedKey(
        apiKeyValuePrefix,
        latestGeneratedKey
      ),
    };
  }, [apiKeyName, apiKeyValuePrefix, latestGeneratedKey]);
  const swaggerViewerKey = useMemo(
    () =>
      [
        generatedGatewayUrl,
        apiKeyLocation,
        apiKeyName,
        latestGeneratedKey ?? '',
        provider?.openapi ?? '',
      ].join('::'),
    [
      apiKeyLocation,
      apiKeyName,
      generatedGatewayUrl,
      latestGeneratedKey,
      provider?.openapi,
    ]
  );
  const deployedGateways = gateways;

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    const providerId = provider?.id;

    if (!organizationId || !providerId) {
      setGateways([]);
      setGatewayDeployments({});
      setSelectedGatewayId('');
      return;
    }

    let isMounted = true;
    void (async () => {
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
          setGatewayDeployments({});
          setSelectedGatewayId('');
          return;
        }

        const latestDeploymentTimeByGateway = new Map<string, number>();
        const deploymentByGateway: Record<string, DeploymentResponse> = {};
        deployedEntries.forEach((deployment) => {
          const nextTime = new Date(deployment.createdAt || 0).getTime();
          const currentTime = latestDeploymentTimeByGateway.get(
            deployment.gatewayId
          );
          if (currentTime === undefined || nextTime > currentTime) {
            latestDeploymentTimeByGateway.set(deployment.gatewayId, nextTime);
            deploymentByGateway[deployment.gatewayId] = deployment;
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
        setGatewayDeployments(deploymentByGateway);
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
        setGatewayDeployments({});
        setSelectedGatewayId('');
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [currentOrganization?.uuid, provider?.id]);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    const providerId = provider?.id;

    if (!organizationId || !providerId) {
      setProxies([]);
      setProxyDeployments({});
      return;
    }

    let isMounted = true;
    setProxyDeployments({});

    void (async () => {
      try {
        const [proxiesResponse, gatewaysResponse] = await Promise.all([
          getLLMProviderProxies(
            providerId,
            organizationId,
            PLATFORM_API_BASE_URL
          ),
          getGateways(organizationId),
        ]);
        if (!isMounted) return;

        const proxyList = proxiesResponse.list ?? [];
        const gatewayList = gatewaysResponse.list ?? [];
        setProxies(proxyList);

        const proxyDeploymentEntries = await Promise.all(
          proxyList.map(async (proxy) => {
            const proxyId = getProxyIdentifier(proxy);

            if (!proxyId) {
              logger.error(
                'Skipping proxy deployment lookup because proxy id is unavailable:',
                proxy
              );
              return undefined;
            }

            const deploymentResponses = await Promise.all(
              gatewayList.map((gateway) =>
                getLLMProxyDeployments(
                  proxyId,
                  organizationId,
                  PLATFORM_API_BASE_URL,
                  gateway.id
                ).catch((err) => {
                  logger.error(
                    `Failed to load deployments for proxy ${proxyId} on gateway ${gateway.id}:`,
                    err
                  );
                  return { list: [] as DeploymentResponse[], count: 0 };
                })
              )
            );

            return [
              proxyId,
              getCurrentDeployment(
                deploymentResponses.flatMap((response) => response.list ?? [])
              ),
            ] as const;
          })
        );
        if (!isMounted) return;

        setProxyDeployments(
          proxyDeploymentEntries.reduce<Record<string, DeploymentResponse>>(
            (acc, entry) => {
              if (!entry) return acc;

              const [proxyId, deployment] = entry;
              if (deployment) {
                acc[proxyId] = deployment;
              }
              return acc;
            },
            {}
          )
        );
      } catch (proxyError) {
        if (!isMounted) return;
        logger.error('Failed to fetch provider proxies:', proxyError);
        setProxies([]);
        setProxyDeployments({});
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [currentOrganization?.uuid, provider?.id]);

  useEffect(() => {
    setLatestGeneratedKey(null);
  }, [provider?.id]);

  useEffect(() => {
    const providerId = provider?.id;
    if (!providerId) {
      fetchedApiKeysProviderIdRef.current = null;
      fetchingApiKeysProviderIdRef.current = null;
      setApiKeys([]);
      setKeysLoading(false);
      return;
    }
    if (fetchedApiKeysProviderIdRef.current === providerId) {
      return;
    }
    if (fetchingApiKeysProviderIdRef.current === providerId) {
      return;
    }
    fetchingApiKeysProviderIdRef.current = providerId;

    let isMounted = true;
    setKeysLoading(true);

    void (async () => {
      try {
        const response = await getProviderAPIKeys();
        if (!isMounted) return;
        setApiKeys(response.list || []);
        fetchedApiKeysProviderIdRef.current = providerId;
      } catch (fetchError) {
        if (!isMounted) return;
        logger.error(
          `Failed to fetch API keys for provider ${providerId}:`,
          fetchError
        );
        showSnackbar('Failed to load API keys.', 'error');
      } finally {
        if (fetchingApiKeysProviderIdRef.current === providerId) {
          fetchingApiKeysProviderIdRef.current = null;
        }
        if (isMounted) {
          setKeysLoading(false);
        }
      }
    })();

    return () => {
      isMounted = false;
      if (fetchingApiKeysProviderIdRef.current === providerId) {
        fetchingApiKeysProviderIdRef.current = null;
      }
      if (fetchedApiKeysProviderIdRef.current !== providerId) {
        setKeysLoading(false);
      }
    };
  }, [getProviderAPIKeys, provider?.id, showSnackbar]);

  const handleCopyGatewayUrl = async () => {
    if (!generatedGatewayUrl) return;
    try {
      await navigator.clipboard.writeText(generatedGatewayUrl);
      showSnackbar('URL copied to clipboard.', 'success');
    } catch (copyError) {
      logger.error('Failed to copy generated URL:', copyError);
      showSnackbar('Failed to copy URL.', 'error');
    }
  };

  const handleGenerateAPIKey = async () => {
    if (!currentOrganization?.uuid || !provider?.id) {
      return;
    }
    const trimmedDisplayName = apiKeyDisplayName.trim();
    if (!trimmedDisplayName) {
      setKeyError('Display name is required.');
      return;
    }

    try {
      setGeneratingKey(true);
      setKeyError(null);

      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const response = await createLLMProviderAPIKey(
        provider.id,
        currentOrganization.uuid,
        {
          id: buildApiKeyResourceName(trimmedDisplayName),
          displayName: apiKeyDisplayName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        PLATFORM_API_BASE_URL
      );

      setGeneratedKey(response.apiKey);
      setLatestGeneratedKey(response.apiKey);
      setIsApiKeyModalOpen(true);
      try {
        const refreshedApiKeys = await getProviderAPIKeys();
        setApiKeys(refreshedApiKeys.list || []);
      } catch (fetchError) {
        logger.error(
          `Failed to refresh API keys for provider ${provider.id}:`,
          fetchError
        );
        showSnackbar('Failed to refresh API keys.', 'error');
      }
      onApiKeyCreated?.();
    } catch (apiKeyError) {
      logger.error('Failed to generate API key:', apiKeyError);
      setKeyError('Failed to generate API key. Please try again.');
    } finally {
      setGeneratingKey(false);
    }
  };

  const handleOpenApiKeyModal = () => {
    setKeyError(null);
    setGeneratedKey(null);
    setApiKeyDisplayName('');
    setIsApiKeyModalOpen(true);
  };

  const handleCopyAPIKey = async () => {
    if (!generatedKey) return;

    try {
      await navigator.clipboard.writeText(generatedKey);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = generatedKey;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  const handleCloseApiKeyModal = () => {
    if (generatingKey) return;
    setIsApiKeyModalOpen(false);
    setApiKeyDisplayName('');
    setGeneratedKey(null);
    setKeyError(null);
  };

  const handleCloseDeleteApiKeyDialog = () => {
    if (!isDeletingKey) {
      setDeleteTargetKeyName(null);
    }
  };

  const handleProxyClick = useCallback(
    (proxyId: string, proxyProjectId?: string) => {
      // removed: ProjectBase no longer carries a `handler` alias field, so
      // match on `id` only.
      const proxyProject = projectsForCurrentOrganization.find(
        (project) => project.id === proxyProjectId
      );

      if (!currentOrganization || !proxyProject) {
        logger.error(
          `Unable to navigate to proxy ${proxyId} because project ${
            proxyProjectId ?? ''
          } is unavailable.`
        );
        return;
      }

      const proxyPath = `/proxies/${encodeURIComponent(proxyId)}`;
      navigate(buildProjectPath(currentOrganization, proxyProject, proxyPath));
    },
    [currentOrganization, navigate, projectsForCurrentOrganization]
  );

  const handleDeleteApiKey = async () => {
    if (
      !deleteTargetKeyName ||
      !provider?.id ||
      !currentOrganization?.uuid
    ) {
      return;
    }

    try {
      setIsDeletingKey(true);
      await deleteLLMProviderAPIKey(
        provider.id,
        deleteTargetKeyName,
        currentOrganization.uuid,
        PLATFORM_API_BASE_URL
      );
      setApiKeys((prev) =>
        prev.filter((key) => (key.id || '').trim() !== deleteTargetKeyName)
      );
      setDeleteTargetKeyName(null);
      onApiKeyCreated?.();
      showSnackbar('API key deleted successfully.', 'success');
    } catch (deleteError) {
      logger.error(
        `Failed to delete API key ${deleteTargetKeyName} for provider ${provider.id}:`,
        deleteError
      );
      showSnackbar('Failed to delete API key. Please try again.', 'error');
    } finally {
      setIsDeletingKey(false);
    }
  };

  return (
    <Stack spacing={2}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <ResourceDrawerCards
            proxies={proxies}
            gateways={gateways}
            gatewayDeployments={gatewayDeployments}
            proxyDeployments={proxyDeployments}
            onProxyClick={handleProxyClick}
            onCreateProxy={onCreateProxy}
            onBlockedNavigation={onBlockedNavigation}
          />
          <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
            OpenAPI Resources
          </Typography>
          <Box
            sx={{
              maxHeight: { xs: 320, md: 520 },
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
            {!provider?.openapi?.trim() ? (
              <Stack
                spacing={1}
                alignItems="center"
                justifyContent="center"
                sx={{ py: 2, textAlign: 'center' }}
              >
                <Box
                  component="img"
                  src={NoData}
                  alt="No available resources"
                  sx={{ width: 80, maxWidth: '80%' }}
                />
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.no.available.resources"
                    defaultMessage={'No available resources.'}
                  />
                </Typography>
              </Stack>
            ) : !parsedOpenApiSpec ? (
              <Alert severity="error">
                Failed to parse the OpenAPI specification. Please provide a
                valid JSON or YAML OpenAPI document.
              </Alert>
            ) : (
              <SwaggerSpecViewer
                key={swaggerViewerKey}
                spec={swaggerSpecWithGatewayServer}
                requestBaseUrl={generatedGatewayUrl}
                defaultHeaders={swaggerDefaultHeaders}
                disableNetworkExecution
                disableResponseSection
                hideInfoSection
                hideServers
                {...(gateways.length === 0 ? { disableTryOutBtn: true } : {})}
                hideAuthorizeButton
                hideTagHeaders
                docExpansion="list"
                defaultModelsExpandDepth={-1}
                displayRequestDuration
                enableResourceSearch
                accessControl={provider?.accessControl}
              />
            )}
          </Box>
        </Box>
        {gateways.length > 0 ? (
          <>
            <Divider
              orientation="vertical"
              flexItem
              sx={{ display: { xs: 'none', md: 'block' } }}
            />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Stack spacing={2}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                      Invoke URL
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Change the Gateway to generate the gateway specific invoke
                      URL.
                    </Typography>
                  </Box>
                  <Grid container spacing={1} alignItems="flex-end">
                    <Grid size={{ xs: 12, md: 4 }}>
                      <FormControl fullWidth>
                        <FormLabel>Gateways</FormLabel>
                        <Select
                          size="small"
                          value={selectedGatewayId}
                          onChange={(event) =>
                            setSelectedGatewayId(String(event.target.value))
                          }
                          displayEmpty
                          disabled={gateways.length === 0}
                        >
                          {gateways.map((gateway) => (
                            <MenuItem key={gateway.id} value={gateway.id}>
                              {gateway.displayName || gateway.name}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    </Grid>
                    <Grid size={{ xs: 12, md: 8 }}>
                      <FormControl fullWidth>
                        <FormLabel>URL</FormLabel>
                        <TextField
                          size="small"
                          fullWidth
                          value={generatedGatewayUrl}
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
                                          void handleCopyGatewayUrl();
                                        }}
                                        disabled={!generatedGatewayUrl}
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
                <Divider />
                <Box>
                  <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.api.keys"
                      defaultMessage={'API Keys'}
                    />
                  </Typography>
                  <Stack spacing={2}>
                    <Stack
                      direction={{ xs: 'column', sm: 'row' }}
                      spacing={2}
                      alignItems={{ xs: 'flex-start', sm: 'center' }}
                      sx={{
                        p: 2,
                        bgcolor: 'background.paper',
                        border: '1px solid',
                        borderColor: highlightApiKeySection
                          ? '#ff6701'
                          : 'divider',
                        borderRadius: 1,
                        transition:
                          'border-color 0.3s ease, box-shadow 0.3s ease',
                        ...(highlightApiKeySection && {
                          boxShadow: '0 0 0 3px rgba(255, 103, 1, 0.2)',
                        }),
                      }}
                    >
                      <Box sx={{ flex: 1 }}>
                        <Typography variant="body2" color="text.secondary">
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.generate.an.api.key.to.authenticate.requests.to.deployed.gateways"
                            defaultMessage={
                              'Generate an API key to authenticate requests to deployed gateways'
                            }
                          />
                        </Typography>
                      </Box>
                      <DisabledActionTooltip disabled={false}>
                        <Button
                          variant="contained"
                          size="medium"
                          onClick={handleOpenApiKeyModal}
                          disabled={deployedGateways.length === 0}
                        >
                          Generate API Key
                        </Button>
                      </DisabledActionTooltip>
                    </Stack>

                    {keyError && (
                      <Alert severity="error" onClose={() => setKeyError(null)}>
                        {keyError}
                      </Alert>
                    )}

                    {keysLoading ? (
                      <Box
                        sx={{
                          display: 'flex',
                          justifyContent: 'center',
                          py: 4,
                        }}
                      >
                        <CircularProgress />
                      </Box>
                    ) : apiKeys.length > 0 ? (
                      <ListingTable.Container>
                        <ListingTable>
                          <ListingTable.Head>
                            <ListingTable.Row>
                              <ListingTable.Cell>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.keyName"
                                  defaultMessage={'Name'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderSecurityTab.maskedKey"
                                  defaultMessage={'API Key'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverviewTab.expiresAt"
                                  defaultMessage={'Expires At'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell align="right">
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderOverviewTab.actions"
                                  defaultMessage={'Actions'}
                                />
                              </ListingTable.Cell>
                            </ListingTable.Row>
                          </ListingTable.Head>
                          <ListingTable.Body>
                            {apiKeys.map((key) => (
                              <ListingTable.Row
                                key={`${
                                  key.id || key.maskedApiKey || 'api-key'
                                }-${key.expiresAt || ''}`}
                              >
                                <ListingTable.Cell>
                                  {key.displayName || key.id || '-'}
                                </ListingTable.Cell>
                                <ListingTable.Cell>
                                  {key.maskedApiKey || '-'}
                                </ListingTable.Cell>
                                <ListingTable.Cell>
                                  <Tooltip
                                    title={
                                      key.expiresAt
                                        ? new Date(key.expiresAt).toUTCString()
                                        : ''
                                    }
                                  >
                                    <span>{formatDate(key.expiresAt)}</span>
                                  </Tooltip>
                                </ListingTable.Cell>
                                <ListingTable.Cell align="right">
                                  <Tooltip
                                    title={
                                      key.id
                                        ? 'Delete API key'
                                        : 'Unable to delete key without a name'
                                    }
                                  >
                                    <span>
                                      <IconButton
                                        size="small"
                                        color="error"
                                        onClick={() =>
                                          setDeleteTargetKeyName(
                                            key.id?.trim() || null
                                          )
                                        }
                                        disabled={!key.id || isDeletingKey}
                                      >
                                        <Trash2 size={16} />
                                      </IconButton>
                                    </span>
                                  </Tooltip>
                                </ListingTable.Cell>
                              </ListingTable.Row>
                            ))}
                          </ListingTable.Body>
                        </ListingTable>
                      </ListingTable.Container>
                    ) : null}
                  </Stack>
                </Box>
              </Stack>
            </Box>
          </>
        ) : null}
      </Stack>

      <Dialog
        open={isApiKeyModalOpen}
        onClose={handleCloseApiKeyModal}
        maxWidth={generatedKey ? 'md' : 'sm'}
        fullWidth
      >
        <DialogTitle>
          {generatedKey ? 'API Key Generated Successfully' : 'Generate API Key'}
        </DialogTitle>
        <DialogContent>
          {keyError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {keyError}
            </Alert>
          )}
          {generatedKey ? (
            <>
              <Alert
                severity="warning"
                sx={{
                  '& .MuiAlert-message': {
                    width: '100%',
                  },
                }}
              >
                <Stack spacing={1}>
                  <Typography variant="caption" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.please.copy.and.save.this.api.key.for.security.reasons.you.won.t.be.able.to.see."
                      defaultMessage={
                        "Please copy and save this API key. For security reasons, you won't be able to see it again."
                      }
                    />
                  </Typography>
                  <Box
                    display="flex"
                    flexDirection="row"
                    alignItems="center"
                    gap={0.5}
                  >
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ flexShrink: 0 }}
                    >
                      {apiKeyLocation}
                    </Typography>
                    <TextField
                      size="small"
                      fullWidth
                      value={apiKeyName}
                      slotProps={{
                        input: {
                          readOnly: true,
                        },
                      }}
                    />
                  </Box>
                  <Box
                    display="flex"
                    flexDirection="row"
                    alignItems="center"
                    gap={0.5}
                  >
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ flexShrink: 0 }}
                    >
                      API Key
                    </Typography>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        p: 1.5,
                        width: '100%',
                        bgcolor: 'background.paper',
                        border: '1px solid',
                        borderColor: 'divider',
                        borderRadius: 1,
                        fontFamily: 'monospace',
                        fontSize: '0.875rem',
                      }}
                    >
                      <Box sx={{ flex: 1, wordBreak: 'break-all' }}>
                        {generatedKey}
                      </Box>
                      <IconButton
                        size="small"
                        onClick={() => {
                          void handleCopyAPIKey();
                        }}
                        sx={{ flexShrink: 0 }}
                      >
                        <Copy size={16} />
                      </IconButton>
                    </Box>
                  </Box>
                </Stack>
              </Alert>
              <Divider sx={{ my: 2 }} />
              <ApiTryOutCurlSnippet
                apiKey={generatedKey}
                gatewayUrl={generatedGatewayUrl}
                apiKeyHeaderName={apiKeyName}
                apiKeyLocation={apiKeyLocation}
                apiKeyValuePrefix={apiKeyValuePrefix}
                providerTemplate={provider?.template}
              />
            </>
          ) : (
            <Stack spacing={1}>
              {/* <Typography variant="body2" color="text.secondary">
                Enter a display name for the new API key.
              </Typography> */}
              <FormControl fullWidth>
                <FormLabel>Key Name</FormLabel>
                <TextField
                  autoFocus
                  size="small"
                  fullWidth
                  value={apiKeyDisplayName}
                  onChange={(event) => setApiKeyDisplayName(event.target.value)}
                  placeholder="Ex: Production Key"
                />
              </FormControl>
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          {generatedKey ? (
            <Button
              onClick={handleCloseApiKeyModal}
              variant="outlined"
              size="small"
            >
              Done
            </Button>
          ) : (
            <>
              <Button
                onClick={handleCloseApiKeyModal}
                variant="outlined"
                color="secondary"
                size="small"
                disabled={generatingKey}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                size="small"
                onClick={() => {
                  void handleGenerateAPIKey();
                }}
                disabled={generatingKey || !apiKeyDisplayName.trim()}
              >
                {generatingKey ? (
                  <>
                    <CircularProgress size={16} sx={{ mr: 1 }} />
                    Generating...
                  </>
                ) : (
                  'Generate'
                )}
              </Button>
            </>
          )}
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(deleteTargetKeyName)}
        onClose={handleCloseDeleteApiKeyDialog}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete API Key</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            Are you sure you want to delete this API key?
          </Typography>
          <Typography variant="body2" sx={{ mt: 1, fontWeight: 600 }}>
            {deleteTargetKeyName}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            size="small"
            onClick={handleCloseDeleteApiKeyDialog}
            disabled={isDeletingKey}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            size="small"
            onClick={() => {
              void handleDeleteApiKey();
            }}
            disabled={isDeletingKey}
          >
            {isDeletingKey ? (
              <>
                <CircularProgress size={16} sx={{ mr: 1 }} />
                Deleting...
              </>
            ) : (
              'Delete'
            )}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}
