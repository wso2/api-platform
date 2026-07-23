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
  Alert,
  Box,
  Button,
  Card,
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
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Copy } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import { createLLMProviderAPIKey } from '../../../../apis/llmProviderApis';
import {
  createSecret,
  deleteSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
} from '../../../../apis/secretApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { useProxies } from '../../../../contexts/proxy';
import {
  LLMProviderProvider,
  useLLMProvider,
  useLLMProviders,
} from '../../../../contexts/llmProvider';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { truncateProviderDisplayName } from '../../../../utils/providerTemplateDisplay';
import { resolveApiKeyAuthDisplay } from '../../../../utils/apiKeyAuthDisplay';
import type { CreateProxyRequest, LLMProvider } from '../../../../utils/types';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import { getErrorMessage, getFieldErrors } from '../../../../utils/apiError';

type FormState = {
  name: string;
  context: string;
  providerId: string;
  version: string;
  description: string;
};

// Backend field names (from CreateProxyRequest) mapped onto this form's state keys.
// "provider.auth.*" errors are not mapped here — best-effort only, they surface
// via the general error banner instead of forcing an unclear match.
const FIELD_NAME_MAP: Partial<Record<string, keyof FormState>> = {
  displayName: 'name',
  description: 'description',
  version: 'version',
  context: 'context',
  'provider.id': 'providerId',
};

/** Derive a URL-safe proxy id from the display name (same logic as LLM provider). */
const toProxyId = (name: string): string =>
  name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const buildApiKeyResourceName = (displayName: string): string => {
  const normalizedDisplayName = displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalizedDisplayName || 'api-key';
};

type LLMProxyNewContentProps = {
  selectedProviderId: string;
  onSelectedProviderIdChange: (providerId: string) => void;
  lockedProviderId: string;
  isProviderSelectionLocked: boolean;
  preselectedProvider: LLMProvider | null;
  preselectedProjectId: string;
};

function LLMProxyNewContent({
  selectedProviderId,
  onSelectedProviderIdChange,
  lockedProviderId,
  isProviderSelectionLocked,
  preselectedProvider,
  preselectedProjectId,
}: LLMProxyNewContentProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const { createProxy } = useProxies();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { providersResponse, isLoading: isProvidersLoading } =
    useLLMProviders();
  const { provider: selectedProvider, isLoading: isSelectedProviderLoading } =
    useLLMProvider();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
    setCurrentProject,
  } = useAppShell();
  const projectFromState = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => project.id === preselectedProjectId
      ) ?? null,
    [preselectedProjectId, projectsForCurrentOrganization]
  );
  const effectiveProject = currentProject ?? projectFromState;
  const isProjectLevel = Boolean(effectiveProject?.id);
  const proxiesPath = isProjectLevel
    ? buildProjectPath(currentOrganization, effectiveProject, '/proxies')
    : buildOrgPath(currentOrganization, '/proxies');

  const providerOptions = providersResponse.list;

  const [formState, setFormState] = useState<FormState>(() => ({
    name: '',
    context: '/',
    providerId: lockedProviderId || selectedProviderId || '',
    version: 'v1.0',
    description: '',
  }));

  const [isCreating, setIsCreating] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof FormState, string>>>({});
  const [isApiKeyModalOpen, setIsApiKeyModalOpen] = useState(false);
  const [apiKeyDisplayName, setApiKeyDisplayName] = useState('');
  const [isGeneratingApiKey, setIsGeneratingApiKey] = useState(false);
  const [apiKeyError, setApiKeyError] = useState<string | null>(null);
  const [generatedKeyForDisplay, setGeneratedKeyForDisplay] = useState<
    string | null
  >(null);
  const [selectedProviderApiKeyValue, setSelectedProviderApiKeyValue] =
    useState<string | null>(null);
  const [
    selectedProviderApiKeyProviderId,
    setSelectedProviderApiKeyProviderId,
  ] = useState('');
  const [manualApiKeyValue, setManualApiKeyValue] = useState('');

  const latestProviderId = useMemo(() => {
    if (providerOptions.length === 0) return '';

    const getLastUpdatedTime = (value?: string): number => {
      if (!value) return 0;
      const timestamp = new Date(value).getTime();
      return Number.isNaN(timestamp) ? 0 : timestamp;
    };

    const latestProvider = providerOptions.reduce((latest, current) => {
      const latestTime = getLastUpdatedTime(
        latest.lastUpdated ?? latest.updatedAt ?? latest.createdAt
      );
      const currentTime = getLastUpdatedTime(
        current.lastUpdated ?? current.updatedAt ?? current.createdAt
      );
      return currentTime > latestTime ? current : latest;
    }, providerOptions[0]);

    return latestProvider.id ?? '';
  }, [providerOptions]);

  useEffect(() => {
    setFormState((prev) => {
      if (isProviderSelectionLocked && lockedProviderId) {
        if (prev.providerId === lockedProviderId) {
          return prev;
        }
        return {
          ...prev,
          providerId: lockedProviderId,
        };
      }

      const hasValidCurrentProvider =
        prev.providerId &&
        providerOptions.some((provider) => provider.id === prev.providerId);

      if (hasValidCurrentProvider) {
        return prev;
      }

      if (prev.providerId && isProvidersLoading) {
        return prev;
      }

      if (!latestProviderId) {
        return prev;
      }

      return {
        ...prev,
        providerId: latestProviderId,
      };
    });
  }, [
    isProviderSelectionLocked,
    isProvidersLoading,
    latestProviderId,
    lockedProviderId,
    providerOptions,
  ]);

  useEffect(() => {
    if (formState.providerId !== selectedProviderId) {
      onSelectedProviderIdChange(formState.providerId);
    }
  }, [formState.providerId, onSelectedProviderIdChange, selectedProviderId]);

  useEffect(() => {
    if (!preselectedProjectId) {
      return;
    }
    if (currentProject?.id === preselectedProjectId) {
      return;
    }
    const matchedProject = projectsForCurrentOrganization.find(
      (project) => project.id === preselectedProjectId
    );
    if (matchedProject) {
      setCurrentProject?.(matchedProject);
    }
  }, [
    currentProject?.id,
    preselectedProjectId,
    projectsForCurrentOrganization,
    setCurrentProject,
  ]);

  const providerDetail = useMemo(() => {
    if (selectedProvider?.id === formState.providerId) {
      return selectedProvider;
    }
    if (preselectedProvider?.id === formState.providerId) {
      return preselectedProvider;
    }
    return null;
  }, [formState.providerId, preselectedProvider, selectedProvider]);

  const selectedProviderRequiresApiKey = Boolean(
    providerDetail?.security?.enabled && providerDetail.security.apiKey?.enabled
  );
  const selectedProviderApiKeyName = resolveApiKeyAuthDisplay(
    providerDetail?.security,
    providerDetail?.globalPolicies
  ).headerName;
  const projectSlug = getProjectSlug(effectiveProject);
  const generatedProxyId = toProxyId(formState.name);
  const computedContext = projectSlug
    ? `/${projectSlug}/${generatedProxyId}`
    : `/${generatedProxyId}`;
  const [contextOverride, setContextOverride] = useState<string | null>(null);
  const effectiveContext = contextOverride ?? computedContext;

  const isGeneratedKeyReady =
    selectedProviderApiKeyProviderId === formState.providerId &&
    Boolean(selectedProviderApiKeyValue);
  const isManualKeyReady = Boolean(manualApiKeyValue.trim());
  const isSelectedProviderApiKeyReady =
    !selectedProviderRequiresApiKey || isGeneratedKeyReady || isManualKeyReady;

  useEffect(() => {
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
    setSelectedProviderApiKeyValue(null);
    setSelectedProviderApiKeyProviderId('');
    setManualApiKeyValue('');
    setApiKeyDisplayName('');
    setIsApiKeyModalOpen(false);
  }, [formState.providerId]);

  const lockedProviderDisplayName = useMemo(() => {
    if (!isProviderSelectionLocked || !lockedProviderId) {
      return '';
    }
    if (preselectedProvider?.id === lockedProviderId) {
      return truncateProviderDisplayName(preselectedProvider.displayName);
    }
    const option = providerOptions.find(
      (provider) => provider.id === lockedProviderId
    );
    if (option?.displayName) {
      return truncateProviderDisplayName(option.displayName);
    }
    return truncateProviderDisplayName(lockedProviderId);
  }, [
    isProviderSelectionLocked,
    lockedProviderId,
    preselectedProvider,
    providerOptions,
  ]);

  const handleCreate = async () => {
    const trimmedName = formState.name.trim();
    const generatedId = toProxyId(trimmedName);
    const payloadProjectId = effectiveProject?.id ?? '';
    if (
      !trimmedName ||
      !generatedId ||
      !formState.providerId ||
      !payloadProjectId ||
      !providerDetail ||
      !isSelectedProviderApiKeyReady
    ) {
      return;
    }

    let createdSecretHandle: string | null = null;
    try {
      setIsCreating(true);
      setFieldErrors({});

      // Encrypt the provider API key as a secret before storing it in the proxy
      // config — even though it is a platform-issued key, it is still a credential
      // that should not be persisted in plain text.
      let providerAuthType = providerDetail?.upstream?.main?.auth?.type ?? '';
      let providerAuthHeader = providerDetail?.upstream?.main?.auth?.header ?? '';
      let providerAuthValue = providerDetail?.upstream?.main?.auth?.value ?? '';
      if (selectedProviderRequiresApiKey) {
        const rawKey = manualApiKeyValue.trim() || selectedProviderApiKeyValue || '';
        const isAlreadyPlaceholder = rawKey.includes('{{ secret ');
        if (isAlreadyPlaceholder) {
          providerAuthType = 'api-key';
          providerAuthHeader = selectedProviderApiKeyName;
          providerAuthValue = rawKey;
        } else {
          const secretHandle = generateSecretHandle();
          const secretResponse = await createSecret({
            id: secretHandle,
            displayName: `${generatedId} Provider API Key`,
            description: `Auto-generated secret for LLM proxy ${generatedId}`,
            value: rawKey,
            type: 'GENERIC',
          });
          logger.info('Created secret for LLM proxy provider auth', {
            secretHandle: secretResponse.id,
            proxyId: generatedId,
          });
          createdSecretHandle = secretResponse.id;
          providerAuthType = 'api-key';
          providerAuthHeader = selectedProviderApiKeyName;
          providerAuthValue = buildSecretPlaceholder(secretResponse.id);
        }
      }

      const payload: CreateProxyRequest = {
        id: generatedId,
        displayName: trimmedName,
        description:
          formState.description.trim() ||
          intl.formatMessage({
            id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.no.description.provided.for.this.proxy',
            defaultMessage: 'No description provided for this proxy.',
          }),
        version: formState.version.trim() || 'v1.0',
        projectId: payloadProjectId,
        context: effectiveContext,
        provider: {
          id: formState.providerId,
          auth: {
            type: providerAuthType,
            header: providerAuthHeader,
            value: providerAuthValue,
          },
        },
        openapi: providerDetail?.openapi ?? '',
        policies: [],
        security: {
          enabled: Boolean(providerDetail?.security?.enabled),
          apiKey: {
            enabled: Boolean(providerDetail?.security?.apiKey?.enabled),
            key: providerDetail?.security?.apiKey?.key ?? '',
            in: providerDetail?.security?.apiKey?.in ?? 'header',
            valuePrefix: providerDetail?.security?.apiKey?.valuePrefix,
          },
        },
      };

      const newProxy = await createProxy(payload);
      navigate(
        isProjectLevel
          ? buildProjectPath(
              currentOrganization,
              effectiveProject,
              `/proxies/${newProxy.id}`
            )
          : buildOrgPath(currentOrganization, `/proxies/${newProxy.id}`),
        {
          state: { proxyAdded: true },
        }
      );
    } catch (error) {
      // Compensate: delete the orphaned secret if proxy creation failed.
      if (createdSecretHandle) {
        deleteSecret(createdSecretHandle).catch((err) => {
          logger.warn('Could not delete orphaned secret after proxy creation failure', {
            secretHandle: createdSecretHandle,
            err,
          });
        });
      }
      const backendFieldErrors = getFieldErrors(error);
      const mappedErrors: Partial<Record<keyof FormState, string>> = {};
      let hasUnmapped = false;
      backendFieldErrors?.forEach(({ field, message }) => {
        const formField = FIELD_NAME_MAP[field];
        if (formField) {
          mappedErrors[formField] = message;
        } else {
          hasUnmapped = true;
        }
      });
      if (Object.keys(mappedErrors).length > 0) {
        setFieldErrors(mappedErrors);
      }
      if (hasUnmapped || Object.keys(mappedErrors).length === 0) {
        const description = getErrorMessage(
          error,
          intl.formatMessage({
            id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.failed.to.create.proxy',
            defaultMessage: 'Failed to create proxy',
          })
        );
        showSnackbar(description, 'error');
      }
    } finally {
      setIsCreating(false);
    }
  };

  const handleOpenApiKeyModal = () => {
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
    setApiKeyDisplayName('');
    setIsApiKeyModalOpen(true);
  };

  const handleCloseApiKeyModal = () => {
    if (isGeneratingApiKey) return;
    setIsApiKeyModalOpen(false);
    setApiKeyDisplayName('');
    setApiKeyError(null);
    setGeneratedKeyForDisplay(null);
  };

  const handleGenerateApiKey = async () => {
    if (!currentOrganization?.uuid || !formState.providerId) {
      return;
    }

    const trimmedDisplayName = apiKeyDisplayName.trim();
    if (!trimmedDisplayName) {
      setApiKeyError('Display name is required.');
      return;
    }

    try {
      setIsGeneratingApiKey(true);
      setApiKeyError(null);

      const expiresAt = new Date();
      expiresAt.setDate(expiresAt.getDate() + 90);

      const response = await createLLMProviderAPIKey(
        formState.providerId,
        currentOrganization.uuid,
        {
          id: buildApiKeyResourceName(trimmedDisplayName),
          displayName: trimmedDisplayName,
          expiresAt: expiresAt.toISOString(),
          issuer: 'api-platform-ai-workspace',
        },
        PLATFORM_API_BASE_URL
      );

      setGeneratedKeyForDisplay(response.apiKey);
      setSelectedProviderApiKeyValue(response.apiKey);
      setSelectedProviderApiKeyProviderId(formState.providerId);
      showSnackbar('API key generated successfully.', 'success');
    } catch (error) {
      logger.error('Failed to generate provider API key:', error);
      const description = getErrorMessage(error, 'Failed to generate API key. Please try again.');
      setApiKeyError(description);
      showSnackbar(description, 'error');
    } finally {
      setIsGeneratingApiKey(false);
    }
  };

  const handleCopyGeneratedApiKey = async () => {
    if (!generatedKeyForDisplay) return;

    try {
      await navigator.clipboard.writeText(generatedKeyForDisplay);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = generatedKeyForDisplay;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={proxiesPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.back.to.list"
          defaultMessage="Back to list"
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.create.llm.proxy"
              defaultMessage="Create App LLM Proxy"
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 720 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 8 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.name.required"
                  defaultMessage="Name *"
                />
              </FormLabel>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.name.placeholder',
                  defaultMessage: 'WSO2 OpenAI Provider Proxy',
                })}
                value={formState.name}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, name: undefined }));
                }}
                error={Boolean(fieldErrors.name)}
                helperText={fieldErrors.name}
                data-cyid="proxy-name-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.version.required"
                  defaultMessage="Version *"
                />
              </FormLabel>
              <TextField
                fullWidth
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.version.placeholder',
                  defaultMessage: 'v1.0',
                })}
                value={formState.version}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    version: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, version: undefined }));
                }}
                error={Boolean(fieldErrors.version)}
                helperText={fieldErrors.version}
                data-cyid="proxy-version-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.description"
                  defaultMessage="Description"
                />
              </FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={3}
                placeholder={intl.formatMessage({
                  id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.description.placeholder',
                  defaultMessage: 'Primary OpenAI provider',
                })}
                value={formState.description}
                onChange={(event) => {
                  setFormState((prev) => ({
                    ...prev,
                    description: event.target.value,
                  }));
                  setFieldErrors((prev) => ({ ...prev, description: undefined }));
                }}
                error={Boolean(fieldErrors.description)}
                helperText={fieldErrors.description}
                data-cyid="proxy-description-input"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <Card sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="h6">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.configuration"
                    defaultMessage="Provider Configuration"
                  />
                </Typography>

                <FormControl fullWidth>
                  <FormLabel sx={{ mb: 0.5 }}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.llm.service.provider"
                      defaultMessage="LLM Provider"
                    />
                  </FormLabel>
                  <Select
                    value={formState.providerId}
                    onChange={(event) =>
                      setFormState((prev) => ({
                        ...prev,
                        providerId: event.target.value,
                      }))
                    }
                    displayEmpty
                    disabled={isProvidersLoading || isProviderSelectionLocked}
                    data-cyid="proxy-provider-select"
                  >
                    {isProviderSelectionLocked && lockedProviderId ? (
                      <MenuItem value={lockedProviderId}>
                        {lockedProviderDisplayName}
                      </MenuItem>
                    ) : isProvidersLoading ? (
                      <MenuItem value="" disabled>
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.loading.providers"
                          defaultMessage="Loading providers..."
                        />
                      </MenuItem>
                    ) : providerOptions.length === 0 ? (
                      <MenuItem value="" disabled>
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.no.providers.available"
                          defaultMessage="No providers available"
                        />
                      </MenuItem>
                    ) : (
                      providerOptions.map((provider) => (
                        <MenuItem key={provider.id} value={provider.id}>
                          {truncateProviderDisplayName(provider.displayName)}
                        </MenuItem>
                      ))
                    )}
                  </Select>
                </FormControl>

                {selectedProviderRequiresApiKey && (
                  <Stack spacing={1}>
                    <Typography variant="body2" sx={{fontWeight: '600'}}>
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.keys"
                        defaultMessage="API Keys"
                      />
                    </Typography>

                    <Stack spacing={0.5}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.enter.api.key.manually.label"
                          defaultMessage="Enter API Key Manually"
                        />
                      </Typography>
                      <Stack
                        sx={{
                          gap: 1,
                        }}
                      >
                        <TextField
                          fullWidth
                          size="small"
                          type="password"
                          placeholder={intl.formatMessage({
                            id: 'aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.api.key.placeholder',
                            defaultMessage: 'Enter API key',
                          })}
                          value={manualApiKeyValue}
                          onChange={(event) =>
                            setManualApiKeyValue(event.target.value)
                          }
                          data-cyid="proxy-api-key-input"
                        />
                        {isManualKeyReady && (
                          <Alert severity="success">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                              defaultMessage="API key will be attached when the proxy is created."
                            />
                          </Alert>
                        )}
                      </Stack>
                    </Stack>

                    <Divider>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.or.divider"
                          defaultMessage="or"
                        />
                      </Typography>
                    </Divider>

                    <Stack spacing={0.5}>
                      <Typography variant="caption" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key.label"
                          defaultMessage="Generate API Key"
                        />
                      </Typography>
                      <Stack
                        direction={{ xs: 'column', sm: 'row' }}
                        spacing={2}
                        alignItems={{ xs: 'flex-start', sm: 'center' }}
                        sx={{
                          px: 1.5,
                          py: 1.5,
                          bgcolor: 'background.paper',
                          border: '1px solid',
                          borderColor: isGeneratedKeyReady
                            ? 'success.main'
                            : 'divider',
                          borderRadius: 1,
                        }}
                      >
                        <Box sx={{ flex: 1 }}>
                          {isGeneratedKeyReady ? (
                            <Alert severity="success">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.provider.api.key.ready"
                                defaultMessage="API key will be attached when the proxy is created."
                              />
                            </Alert>
                          ) : (
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.provider.api.key.description"
                                defaultMessage="Generate an API key for the selected LLM provider."
                              />
                            </Typography>
                          )}
                        </Box>
                        <Tooltip
                          title={
                            isSelectedProviderLoading
                              ? 'Loading selected provider details.'
                              : ''
                          }
                          placement="top"
                        >
                          <span>
                            <Button
                              variant="contained"
                              size="medium"
                              onClick={handleOpenApiKeyModal}
                              disabled={isSelectedProviderLoading}
                            >
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.generate.api.key"
                                defaultMessage="Generate API Key"
                              />
                            </Button>
                          </span>
                        </Tooltip>
                      </Stack>
                    </Stack>

                    {apiKeyError && (
                      <Alert
                        severity="error"
                        onClose={() => setApiKeyError(null)}
                      >
                        {apiKeyError}
                      </Alert>
                    )}
                  </Stack>
                )}
              </Stack>
            </Card>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.context"
                  defaultMessage="Context"
                />
              </FormLabel>
              <TextField
                fullWidth
                value={effectiveContext}
                onChange={(event) => setContextOverride(event.target.value)}
                data-cyid="proxy-context-input"
              />
            </FormControl>
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button
            variant="contained"
            component={RouterLink}
            to={proxiesPath}
            color="secondary"
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.cancel"
              defaultMessage="Cancel"
            />
          </Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={
              !formState.name.trim() ||
              !formState.providerId ||
              !effectiveProject?.id ||
              !providerDetail ||
              !isSelectedProviderApiKeyReady ||
              isCreating
            }
            data-cyid="create-proxy-button"
          >
            {isCreating ? (
              <CircularProgress size={20} />
            ) : (
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.create.proxy"
                defaultMessage="Create Proxy"
              />
            )}
          </Button>
        </Box>
      </Box>

      <Dialog
        open={isApiKeyModalOpen}
        onClose={handleCloseApiKeyModal}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          {generatedKeyForDisplay
            ? 'API Key Generated Successfully'
            : 'Generate API Key'}
        </DialogTitle>
        <DialogContent>
          {apiKeyError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {apiKeyError}
            </Alert>
          )}
          {generatedKeyForDisplay ? (
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
                    id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyNew.copy.generated.api.key.warning"
                    defaultMessage="Please copy and save this API key. For security reasons, you won't be able to see it again."
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
                    header
                  </Typography>
                  <TextField
                    size="small"
                    fullWidth
                    value={selectedProviderApiKeyName}
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
                      {generatedKeyForDisplay}
                    </Box>
                    <IconButton
                      size="small"
                      onClick={() => {
                        void handleCopyGeneratedApiKey();
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
          {generatedKeyForDisplay ? (
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
                disabled={isGeneratingApiKey}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                size="small"
                onClick={() => {
                  void handleGenerateApiKey();
                }}
                disabled={isGeneratingApiKey || !apiKeyDisplayName.trim()}
              >
                {isGeneratingApiKey ? (
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
    </PageContent>
  );
}

type LLMProxyNewLocationState = {
  preselectedProviderId?: string;
  preselectedProvider?: LLMProvider;
  selectedProjectId?: string;
};

export default function LLMProxyNew() {
  const location = useLocation();
  const state = location.state as LLMProxyNewLocationState | null;
  const lockedProviderId = state?.preselectedProviderId?.trim() ?? '';
  const preselectedProjectId = state?.selectedProjectId?.trim() ?? '';
  const preselectedProvider =
    state?.preselectedProvider?.id === lockedProviderId
      ? state.preselectedProvider
      : null;
  const [selectedProviderId, setSelectedProviderId] =
    useState(lockedProviderId);

  useEffect(() => {
    if (!lockedProviderId) return;
    setSelectedProviderId(lockedProviderId);
  }, [lockedProviderId]);

  return (
    <LLMProviderProvider providerId={selectedProviderId}>
      <LLMProxyNewContent
        selectedProviderId={selectedProviderId}
        onSelectedProviderIdChange={setSelectedProviderId}
        lockedProviderId={lockedProviderId}
        isProviderSelectionLocked={Boolean(lockedProviderId)}
        preselectedProvider={preselectedProvider}
        preselectedProjectId={preselectedProjectId}
      />
    </LLMProviderProvider>
  );
}
