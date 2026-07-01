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

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
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
  ListingTable,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Copy, Trash2 } from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useProxy } from '../../../../contexts/proxy';
import { getGateways } from '../../../../apis/gatewayApis';
import { getLLMProxyDeployments } from '../../../../apis/llmProxiesApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { logger } from '../../../../utils/logger';
import NoData from '../../../../assets/images/NoData.svg';
import { FormattedMessage } from 'react-intl';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';
import type { Gateway } from '../../../../apis/gatewayTypes';
import type { DeploymentResponse, UserAPIKey } from '../../../../utils/types';

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
      logger.error('Failed to parse proxy OpenAPI spec:', parseError);
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

export default function LLMProxyOverviewTab() {
  const { currentOrganization } = useAppShell();
  const { proxy, getProxyAPIKeys, createProxyAPIKey, deleteProxyAPIKey } =
    useProxy();
  const showSnackbar = useAIWorkspaceSnackbar();
  const fetchedApiKeysProxyIdRef = useRef<string | null>(null);
  const fetchingApiKeysProxyIdRef = useRef<string | null>(null);

  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [isGatewaysLoading, setIsGatewaysLoading] = useState(false);
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

  const apiKeyLocation = proxy?.security?.apiKey?.in ?? 'header';
  const apiKeyName = proxy?.security?.apiKey?.key ?? 'X-API-Key';

  const parsedOpenApiSpec = useMemo(
    () => parseOpenApiSpec(proxy?.openapi || ''),
    [proxy?.openapi]
  );

  const selectedGateway = useMemo(
    () => gateways.find((gateway) => gateway.id === selectedGatewayId) ?? null,
    [gateways, selectedGatewayId]
  );

  const generatedGatewayUrl = useMemo(() => {
    const vhost = selectedGateway?.vhost?.trim();
    if (!vhost) return '';

    const normalizedBase = /^https?:\/\//i.test(vhost)
      ? vhost.replace(/\/+$/, '')
      : `https://${vhost.replace(/\/+$/, '')}`;
    const context = (proxy?.context || '/').trim();
    const normalizedContext = context
      ? context.startsWith('/')
        ? context
        : `/${context}`
      : '/';
    return `${normalizedBase}${normalizedContext}`;
  }, [proxy?.context, selectedGateway?.vhost]);

  const swaggerSpecWithGatewayServer = useMemo<OpenApiSpec>(() => {
    if (!parsedOpenApiSpec) return {};
    if (!generatedGatewayUrl) return parsedOpenApiSpec;

    const existingServers = Array.isArray(parsedOpenApiSpec.servers)
      ? parsedOpenApiSpec.servers.filter(
          (server): server is Record<string, unknown> =>
            typeof server === 'object' && server !== null
        )
      : [];
    const nonDuplicateServers = existingServers.filter(
      (server) => String(server.url ?? '') !== generatedGatewayUrl
    );

    return {
      ...parsedOpenApiSpec,
      servers: [{ url: generatedGatewayUrl }, ...nonDuplicateServers],
    };
  }, [parsedOpenApiSpec, generatedGatewayUrl]);

  const swaggerDefaultHeaders = useMemo<
    Record<string, string> | undefined
  >(() => {
    if (!latestGeneratedKey) return undefined;
    const resolvedApiKeyHeaderName = apiKeyName.trim() || 'X-API-Key';

    return {
      [resolvedApiKeyHeaderName]: latestGeneratedKey,
    };
  }, [apiKeyName, latestGeneratedKey]);

  const swaggerViewerKey = useMemo(
    () =>
      [
        generatedGatewayUrl,
        apiKeyLocation,
        apiKeyName,
        latestGeneratedKey ?? '',
        proxy?.openapi ?? '',
      ].join('::'),
    [
      apiKeyLocation,
      apiKeyName,
      generatedGatewayUrl,
      latestGeneratedKey,
      proxy?.openapi,
    ]
  );

  const deployedGateways = gateways;

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    const proxyId = proxy?.id;

    if (!organizationId || !proxyId) {
      setGateways([]);
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

        const deploymentPromises = availableGateways.map((gateway) =>
          getLLMProxyDeployments(
            proxyId,
            organizationId,
            PLATFORM_API_BASE_URL,
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

        const sortedDeployedGateways = availableGateways
          .filter((gateway) => latestDeploymentTimeByGateway.has(gateway.id))
          .sort((a, b) => {
            const timeA = latestDeploymentTimeByGateway.get(a.id) || 0;
            const timeB = latestDeploymentTimeByGateway.get(b.id) || 0;
            return timeB - timeA;
          });

        setGateways(sortedDeployedGateways);
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
  }, [currentOrganization?.uuid, proxy?.id]);

  useEffect(() => {
    setLatestGeneratedKey(null);
  }, [proxy?.id]);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    const proxyId = proxy?.id;
    if (!organizationId || !proxyId) {
      fetchedApiKeysProxyIdRef.current = null;
      fetchingApiKeysProxyIdRef.current = null;
      setApiKeys([]);
      setKeysLoading(false);
      return;
    }
    if (fetchedApiKeysProxyIdRef.current === proxyId) {
      return;
    }
    if (fetchingApiKeysProxyIdRef.current === proxyId) {
      return;
    }
    fetchingApiKeysProxyIdRef.current = proxyId;

    let isMounted = true;
    setKeysLoading(true);

    void (async () => {
      try {
        const response = await getProxyAPIKeys();
        if (!isMounted) return;
        setApiKeys(response.items || []);
        fetchedApiKeysProxyIdRef.current = proxyId;
      } catch (fetchError) {
        if (!isMounted) return;
        logger.error(
          `Failed to fetch API keys for proxy ${proxyId}:`,
          fetchError
        );
        showSnackbar('Failed to load API keys.', 'error');
      } finally {
        if (fetchingApiKeysProxyIdRef.current === proxyId) {
          fetchingApiKeysProxyIdRef.current = null;
        }
        if (isMounted) {
          setKeysLoading(false);
        }
      }
    })();

    return () => {
      isMounted = false;
      if (fetchingApiKeysProxyIdRef.current === proxyId) {
        fetchingApiKeysProxyIdRef.current = null;
      }
      if (fetchedApiKeysProxyIdRef.current !== proxyId) {
        setKeysLoading(false);
      }
    };
  }, [currentOrganization?.uuid, getProxyAPIKeys, proxy?.id, showSnackbar]);

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
    if (!currentOrganization?.uuid || !proxy?.id) {
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

      const response = await createProxyAPIKey({
        id: buildApiKeyResourceName(trimmedDisplayName),
        displayName: apiKeyDisplayName,
        expiresAt: expiresAt.toISOString(),
        issuer: 'api-platform-ai-workspace',
      });

      setGeneratedKey(response.apiKey);
      setLatestGeneratedKey(response.apiKey);
      setIsApiKeyModalOpen(true);
      try {
        const refreshedApiKeys = await getProxyAPIKeys();
        setApiKeys(refreshedApiKeys.items || []);
      } catch (fetchError) {
        logger.error(
          `Failed to refresh API keys for proxy ${proxy.id}:`,
          fetchError
        );
        showSnackbar('Failed to refresh API keys.', 'error');
      }
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

  const handleDeleteApiKey = async () => {
    if (!deleteTargetKeyName || !proxy?.id) {
      return;
    }

    try {
      setIsDeletingKey(true);
      await deleteProxyAPIKey(deleteTargetKeyName);
      setApiKeys((prev) =>
        prev.filter((key) => (key.name || '').trim() !== deleteTargetKeyName)
      );
      setDeleteTargetKeyName(null);
      showSnackbar('API key deleted successfully.', 'success');
    } catch (deleteError) {
      logger.error(
        `Failed to delete API key ${deleteTargetKeyName} for proxy ${proxy.id}:`,
        deleteError
      );
      showSnackbar('Failed to delete API key. Please try again.', 'error');
    } finally {
      setIsDeletingKey(false);
    }
  };

  return (
    <Stack spacing={2}>
      {isGatewaysLoading ? (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <CircularProgress size={16} />
          <Typography variant="caption" color="text.secondary">
            Loading gateways...
          </Typography>
        </Box>
      ) : null}

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
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
            {!proxy?.openapi?.trim() ? (
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
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.no.available.resources"
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
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.api.keys"
                      defaultMessage={'App LLM Proxy Keys'}
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
                        borderColor: 'divider',
                        borderRadius: 1,
                      }}
                    >
                      <Box sx={{ flex: 1 }}>
                        <Typography variant="body2" color="text.secondary">
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.generate.an.api.key"
                            defaultMessage={
                              'Generate an API key to authenticate requests to deployed gateways'
                            }
                          />
                        </Typography>
                      </Box>
                      <Tooltip
                        title={
                          deployedGateways.length === 0
                            ? 'No deployed gateways available. Deploy to a gateway first to generate an API key.'
                            : ''
                        }
                        placement="top"
                      >
                        <span>
                          <Button
                            variant="contained"
                            size="medium"
                            onClick={handleOpenApiKeyModal}
                            disabled={deployedGateways.length === 0}
                          >
                            Generate API Key
                          </Button>
                        </span>
                      </Tooltip>
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
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.keyName"
                                  defaultMessage={'Name'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.maskedKey"
                                  defaultMessage={'API Key'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell>
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.expiresAt"
                                  defaultMessage={'Expires At'}
                                />
                              </ListingTable.Cell>
                              <ListingTable.Cell align="right">
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.actions"
                                  defaultMessage={'Actions'}
                                />
                              </ListingTable.Cell>
                            </ListingTable.Row>
                          </ListingTable.Head>
                          <ListingTable.Body>
                            {apiKeys.map((key) => (
                              <ListingTable.Row
                                key={`${
                                  key.name || key.maskedApiKey || 'api-key'
                                }-${key.expiresAt || ''}`}
                              >
                                <ListingTable.Cell>
                                  {key.name || '-'}
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
                                      key.name
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
                                            key.name?.trim() || null
                                          )
                                        }
                                        disabled={!key.name || isDeletingKey}
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
        maxWidth="sm"
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
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyOverviewTab.please.copy.and.save.this.api.key"
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
          ) : (
            <Stack spacing={1}>
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
