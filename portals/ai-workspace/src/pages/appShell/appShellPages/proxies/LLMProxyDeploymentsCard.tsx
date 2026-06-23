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
  InputAdornment,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  Alert,
  IconButton,
  Tooltip,
} from '@wso2/oxygen-ui';
import { Copy } from '@wso2/oxygen-ui-icons-react';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useProxy } from '../../../../contexts/proxy';
import { getGateways } from '../../../../apis/gatewayApis';
import {
  getLLMProxyDeployments,
  createLLMProxyAPIKey,
} from '../../../../apis/llmProxiesApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { logger } from '../../../../utils/logger';
import type { Gateway } from '../../../../apis/gatewayTypes';
import type { DeploymentResponse } from '../../../../utils/types';
import { FormattedMessage } from 'react-intl';
import {
  DisabledActionTooltip,
  GATEWAY_MANAGED_ARTIFACT_TOOLTIP,
} from '../../../../utils/readOnlyArtifacts';

interface GatewayWithDeployment extends Gateway {
  deployment?: DeploymentResponse;
}

export default function LLMProxyDeploymentsCard() {
  const { currentOrganization } = useAppShell();
  const { proxy } = useProxy();
  const [loading, setLoading] = useState(true);
  const [gateways, setGateways] = useState<GatewayWithDeployment[]>([]);
  const [deployments, setDeployments] = useState<DeploymentResponse[]>([]);
  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const [generatingKey, setGeneratingKey] = useState(false);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [keyError, setKeyError] = useState<string | null>(null);
  const isReadOnlyProxy = Boolean(proxy?.readOnly);
  const apiKeyLocation = proxy?.security?.apiKey?.in ?? 'header';
  const apiKeyName = proxy?.security?.apiKey?.key ?? 'X-API-Key';

  useEffect(() => {
    const fetchData = async () => {
      if (!currentOrganization?.uuid || !proxy?.id) {
        return;
      }

      try {
        setLoading(true);

        // Fetch gateways first
        const gatewaysResponse = await getGateways(currentOrganization.uuid);

        // Fetch deployments for each gateway in parallel
        const deploymentPromises = gatewaysResponse.list.map((gateway) =>
          getLLMProxyDeployments(
            proxy.id,
            currentOrganization.uuid,
            PLATFORM_API_BASE_URL,
            gateway.id
          ).catch((error) => {
            logger.error(
              `Failed to fetch deployments for gateway ${gateway.id}:`,
              error
            );
            return { list: [], count: 0 };
          })
        );

        const deploymentResponses = await Promise.all(deploymentPromises);

        // Flatten all deployments from all gateways
        const allDeployments = deploymentResponses.flatMap(
          (response) => response.list
        );

        // Map deployments to gateways
        const gatewaysWithDeployments: GatewayWithDeployment[] =
          gatewaysResponse.list.map((gateway) => {
            const deployment = allDeployments.find(
              (d) => d.gatewayId === gateway.id && d.status === 'DEPLOYED'
            );
            return {
              ...gateway,
              deployment,
            };
          });

        setGateways(gatewaysWithDeployments);
        setDeployments(allDeployments);
        const nextDeployedGateways = gatewaysWithDeployments.filter((g) =>
          Boolean(g.deployment)
        );
        setSelectedGatewayId((currentSelectedId) => {
          if (
            currentSelectedId &&
            nextDeployedGateways.some(
              (gateway) => gateway.id === currentSelectedId
            )
          ) {
            return currentSelectedId;
          }
          return nextDeployedGateways[0]?.id || '';
        });
      } catch (error) {
        logger.error('Failed to fetch deployments data:', error);
        setSelectedGatewayId('');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [currentOrganization?.uuid, proxy?.id]);

  const handleGenerateAPIKey = async () => {
    if (isReadOnlyProxy) return;
    if (!currentOrganization?.uuid || !proxy?.id) {
      return;
    }

    try {
      setGeneratingKey(true);
      setKeyError(null);

      // Set expiration to 90 days from now
      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const response = await createLLMProxyAPIKey(
        proxy.id,
        currentOrganization.uuid,
        {
          name: `key-${Date.now()}`,
          displayName: `API Key - ${new Date().toLocaleString()}`,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        }
      );

      setGeneratedKey(response.apiKey);
    } catch (error) {
      logger.error('Failed to generate API key:', error);
      setKeyError('Failed to generate API key. Please try again.');
    } finally {
      setGeneratingKey(false);
    }
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

  const deployedGateways = useMemo(
    () => gateways.filter((g) => g.deployment),
    [gateways]
  );
  const isGatewaysLoading = loading;
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
    const context = (proxy?.context || '/').trim();
    const normalizedContext = context
      ? context.startsWith('/')
        ? context
        : `/${context}`
      : '/';
    return `${normalizedBase}${normalizedContext}`;
  }, [proxy?.context, selectedGateway?.vhost]);

  const handleCopyInvokeUrl = async () => {
    if (!generatedInvokeUrl) return;

    try {
      await navigator.clipboard.writeText(generatedInvokeUrl);
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

  return (
    <Card sx={{ height: '100%' }}>
      <CardContent>
        <Stack spacing={3}>
          {/* Get Started Section */}
          <Box>
            <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.get.started"
                defaultMessage={'Get Started'}
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.start.using.this.llm.proxy"
                defaultMessage={
                  'Start using this App LLM Proxy'
                }
              />
            </Typography>
          </Box>

          {isGatewaysLoading ? (
            <Box
              sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}
            >
              <CircularProgress size={16} />
              <Typography variant="caption" color="text.secondary">
                Loading gateways...
              </Typography>
            </Box>
          ) : null}
          {deployedGateways.length > 0 ? (
            <Stack spacing={1.5} sx={{ mb: 1.5 }}>
              <Box>
                <Typography variant="h6" sx={{ fontWeight: 600 }}>Invoke URL</Typography>
                <Typography variant="body2" color="text.secondary">
                  Change the Gateway to generate the gateway specific invoke
                  URL.
                </Typography>
              </Box>
              <Grid container spacing={1} alignItems="flex-end">
                <Grid size={{ xs: 12, md: 3 }}>
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
                <Grid size={{ xs: 12, md: 9 }}>
                  <FormControl fullWidth>
                    <FormLabel>URL</FormLabel>
                    <TextField
                      size="small"
                      fullWidth
                      value={generatedInvokeUrl}
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

          {/* API Keys Section */}
          <Box>
            <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.api.keys"
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
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.generate.an.api.key"
                      defaultMessage={
                        'Generate an App LLM Proxy key to authenticate requests to deployed gateways'
                      }
                    />
                  </Typography>
                </Box>
                <DisabledActionTooltip
                  disabled={isReadOnlyProxy}
                  title={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                >
                  <Tooltip
                    title={
                      !isReadOnlyProxy && deployedGateways.length === 0
                        ? 'No deployed gateways available. Deploy to a gateway first to generate an API key.'
                        : ''
                    }
                    placement="top"
                  >
                    <span>
                      <Button
                        variant="contained"
                        size="medium"
                        onClick={handleGenerateAPIKey}
                        disabled={
                          isReadOnlyProxy ||
                          generatingKey ||
                          deployedGateways.length === 0
                        }
                      >
                        {generatingKey ? (
                          <>
                            <CircularProgress size={16} sx={{ mr: 1 }} />
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.generating"
                              defaultMessage={'Generating...'}
                            />
                          </>
                        ) : (
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.generate.api.key"
                            defaultMessage={'Generate API Key'}
                          />
                        )}
                      </Button>
                    </span>
                  </Tooltip>
                </DisabledActionTooltip>
              </Stack>

              {keyError && (
                <Alert severity="error" onClose={() => setKeyError(null)}>
                  {keyError}
                </Alert>
              )}

              {generatedKey && (
                <Alert
                  severity="warning"
                  sx={{
                    '& .MuiAlert-message': {
                      width: '100%',
                    },
                  }}
                >
                  <Stack spacing={1}>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.api.key.generated.successfully"
                        defaultMessage={'API Key Generated Successfully'}
                      />
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.please.copy.and.save.this.api.key"
                        defaultMessage={
                          "Please copy and save this API key. For security reasons, you won't be able to see it again."
                        }
                      />
                    </Typography>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 0.5,
                      }}
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
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 0.5,
                      }}
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
                          flex: 1,
                          minWidth: 0,
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
                          aria-label="Copy API key"
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
              )}
            </Stack>
          </Box>

          <Divider />

          {/* Deployed Gateways Section */}
          <Box>
            <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.deployed.gateways"
                defaultMessage={'Deployed Gateways'}
              />
            </Typography>

            {loading ? (
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'center',
                  py: 3,
                }}
              >
                <CircularProgress size={24} />
              </Box>
            ) : deployedGateways.length > 0 ? (
              <Stack spacing={1.5}>
                {deployedGateways.map((gateway) => (
                  <Box
                    key={gateway.id}
                    sx={{
                      p: 1.5,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      bgcolor: 'background.paper',
                    }}
                  >
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        mb: 0.5,
                      }}
                    >
                      <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                        {gateway.displayName || gateway.name}
                      </Typography>
                      <Chip
                        size="small"
                        label={gateway.isActive ? 'Active' : 'Inactive'}
                        color={gateway.isActive ? 'success' : 'default'}
                        variant="outlined"
                      />
                    </Box>
                    {gateway.deployment && (
                      <Typography variant="caption" color="text.secondary">
                        Deployment: {gateway.deployment.name}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Stack>
            ) : (
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  textAlign: 'center',
                  minHeight: 100,
                  border: '1px dashed',
                  borderColor: 'divider',
                  borderRadius: 1,
                  bgcolor: 'background.paper',
                  p: 3,
                }}
              >
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDeploymentsCard.no.deployed.gateways"
                    defaultMessage={'No deployed gateways'}
                  />
                </Typography>
              </Box>
            )}
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}
