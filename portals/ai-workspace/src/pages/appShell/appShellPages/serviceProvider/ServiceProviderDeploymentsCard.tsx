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
  Card,
  CardContent,
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
  ListingTable,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Copy, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { createLLMProviderAPIKey } from '../../../../apis/llmProviderApis';
import type { Gateway } from '../../../../apis/gatewayTypes';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useGatewayDeploy } from '../../../../contexts/GatewayDeployContext';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type { UserAPIKey } from '../../../../utils/types';
import { logger } from '../../../../utils/logger';
import {
  DisabledActionTooltip,
  GATEWAY_MANAGED_ARTIFACT_TOOLTIP,
} from '../../../../utils/readOnlyArtifacts';

type ServiceProviderDeploymentsCardProps = {
  isGatewaysLoading: boolean;
  gateways: Gateway[];
  selectedGatewayId: string;
  onGatewayChange: (gatewayId: string) => void;
  generatedInvokeUrl: string;
  onCopyInvokeUrl: () => Promise<void>;
  onLatestGeneratedApiKeyChange?: (apiKey: string | null) => void;
  onApiKeyCreated?: () => void;
};

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

export default function ServiceProviderDeploymentsCard({
  isGatewaysLoading,
  gateways,
  selectedGatewayId,
  onGatewayChange,
  generatedInvokeUrl,
  onCopyInvokeUrl,
  onLatestGeneratedApiKeyChange,
  onApiKeyCreated,
}: ServiceProviderDeploymentsCardProps) {
  const { currentOrganization } = useAppShell();
  const { provider, getProviderAPIKeys, deleteProviderAPIKey } =
    useLLMProvider();
  const {
    gateways: deploymentGateways,
    isLoading,
    deployments,
  } = useGatewayDeploy();
  const showSnackbar = useAIWorkspaceSnackbar();

  const fetchedApiKeysProviderIdRef = useRef<string | null>(null);
  const fetchingApiKeysProviderIdRef = useRef<string | null>(null);

  const [generatingKey, setGeneratingKey] = useState(false);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [isApiKeyModalOpen, setIsApiKeyModalOpen] = useState(false);
  const [apiKeyDisplayName, setApiKeyDisplayName] = useState('');
  const [keyError, setKeyError] = useState<string | null>(null);
  const [apiKeys, setApiKeys] = useState<UserAPIKey[]>([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const [deleteTargetKeyName, setDeleteTargetKeyName] = useState<string | null>(
    null
  );
  const [isDeletingKey, setIsDeletingKey] = useState(false);
  const [latestGeneratedKeyName, setLatestGeneratedKeyName] = useState<
    string | null
  >(null);

  const apiKeyLocation = provider?.security?.apiKey?.in ?? 'header';
  const apiKeyName = provider?.security?.apiKey?.key ?? 'X-API-Key';
  const isReadOnlyProvider = Boolean(provider?.readOnly);

  const deployedGateways = useMemo(() => {
    if (!deployments?.list || deployments.list.length === 0) return [];
    const gatewayIdsWithDeployments = new Set(
      deployments.list.map((deployment) => deployment.gatewayId)
    );
    return deploymentGateways.filter((gateway) =>
      gatewayIdsWithDeployments.has(gateway.id)
    );
  }, [deploymentGateways, deployments]);

  useEffect(() => {
    onLatestGeneratedApiKeyChange?.(null);
    setLatestGeneratedKeyName(null);
  }, [onLatestGeneratedApiKeyChange, provider?.id]);

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
        setApiKeys(response.items || []);
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

  const handleGenerateAPIKey = async () => {
    if (!currentOrganization?.uuid || !provider?.id) {
      return;
    }

    const trimmedDisplayName = apiKeyDisplayName.trim();
    if (!trimmedDisplayName) {
      setKeyError('Display name is required.');
      return;
    }

    const keyName = buildApiKeyResourceName(trimmedDisplayName);

    try {
      setGeneratingKey(true);
      setKeyError(null);

      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const response = await createLLMProviderAPIKey(
        provider.id,
        currentOrganization.uuid,
        {
          id: keyName,
          displayName: apiKeyDisplayName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        PLATFORM_API_BASE_URL
      );

      setGeneratedKey(response.apiKey);
      setLatestGeneratedKeyName(keyName);
      onLatestGeneratedApiKeyChange?.(response.apiKey);

      try {
        const refreshedApiKeys = await getProviderAPIKeys();
        setApiKeys(refreshedApiKeys.items || []);
      } catch (fetchError) {
        logger.error(
          `Failed to refresh API keys for provider ${provider.id}:`,
          fetchError
        );
        showSnackbar('Failed to refresh API keys.', 'error');
      }
      onApiKeyCreated?.();
    } catch (error) {
      logger.error('Failed to generate API key:', error);
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

  const handleCloseApiKeyModal = () => {
    if (generatingKey) return;
    setIsApiKeyModalOpen(false);
    setApiKeyDisplayName('');
    setGeneratedKey(null);
    setKeyError(null);
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

  const handleCloseDeleteApiKeyDialog = () => {
    if (!isDeletingKey) {
      setDeleteTargetKeyName(null);
    }
  };

  const handleDeleteApiKey = async () => {
    if (!deleteTargetKeyName) {
      return;
    }

    try {
      setIsDeletingKey(true);
      await deleteProviderAPIKey(deleteTargetKeyName);
      setApiKeys((prev) =>
        prev.filter((key) => (key.id || '').trim() !== deleteTargetKeyName)
      );

      if (latestGeneratedKeyName === deleteTargetKeyName) {
        setLatestGeneratedKeyName(null);
        onLatestGeneratedApiKeyChange?.(null);
      }

      setDeleteTargetKeyName(null);
      onApiKeyCreated?.();
      showSnackbar('API key deleted successfully.', 'success');
    } catch (deleteError) {
      logger.error(
        `Failed to delete API key ${deleteTargetKeyName} for provider ${provider?.id}:`,
        deleteError
      );
      showSnackbar('Failed to delete API key. Please try again.', 'error');
    } finally {
      setIsDeletingKey(false);
    }
  };

  return (
    <Card>
      <CardContent>
        <Stack spacing={3} minHeight={400}>
          {/* <Box>
            <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.get.started"
                defaultMessage={'Get Started'}
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.start.using.this.llm.provider"
                defaultMessage={'Start using this LLM provider'}
              />
            </Typography>
          </Box> */}

          <Box>
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

            {gateways.length > 0 ? (
              <Stack spacing={1.5} sx={{ mb: 1.5 }}>
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
                  <Grid size={{ xs: 12, md: 3 }}>
                    <FormControl fullWidth>
                      <FormLabel>Gateways</FormLabel>
                      <Select
                        size="small"
                        value={selectedGatewayId}
                        onChange={(event) =>
                          onGatewayChange(String(event.target.value))
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
                                        void onCopyInvokeUrl();
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
          </Box>

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
                  borderColor: 'divider',
                  borderRadius: 1,
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
                    disabled={gateways.length === 0}
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
                          key={`${key.id || key.maskedApiKey || 'api-key'}-${
                            key.expiresAt || ''
                          }`}
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
                                  : 'Unable to delete key without an identifier'
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

          {/* <Box>
            <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.deployed.gateways"
                defaultMessage={'Deployed Gateways'}
              />
            </Typography>

            {isLoading ? (
              <Box display="flex" justifyContent="center" py={4}>
                <CircularProgress />
              </Box>
            ) : deployedGateways.length === 0 ? (
              <Box
                sx={{
                  textAlign: 'center',
                  py: 4,
                  px: 2,
                  bgcolor: 'background.paper',
                  border: '1px dashed',
                  borderColor: 'divider',
                  borderRadius: 1,
                }}
              >
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploymentsCard.no.deployed.gateways"
                    defaultMessage={'No deployed gateways'}
                  />
                </Typography>
              </Box>
            ) : (
              <Stack spacing={2}>
                {deployedGateways.map((gateway) => (
                  <Card
                    key={gateway.id}
                    variant="outlined"
                    sx={{
                      transition: 'all 0.2s',
                      '&:hover': {
                        boxShadow: 1,
                      },
                    }}
                  >
                    <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
                      <Stack
                        direction="row"
                        justifyContent="space-between"
                        alignItems="center"
                        spacing={2}
                      >
                        <Typography variant="body1" sx={{ fontWeight: 600 }}>
                          {gateway.displayName || gateway.name}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          {gateway.vhost}
                        </Typography>
                        <Chip
                          label={gateway.isActive ? 'Active' : 'Inactive'}
                          size="small"
                          color={gateway.isActive ? 'success' : 'default'}
                          variant="outlined"
                        />
                      </Stack>
                    </CardContent>
                  </Card>
                ))}
              </Stack>
            )}
          </Box> */}
        </Stack>
      </CardContent>

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
    </Card>
  );
}
