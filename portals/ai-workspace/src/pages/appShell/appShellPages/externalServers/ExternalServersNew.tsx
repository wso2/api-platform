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

import React, { useMemo, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Button,
  Card,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronLeft,
  Eye,
  EyeOff,
  HelpCircle,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import { useMCPServerValidation } from '../../../../contexts/MCP';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import {
  createSecret,
  buildSecretPlaceholder,
  generateSecretHandle,
} from '../../../../apis/secretApis';
import type { MCPServerInfoFetchRequest, CreateMCPServerRequest } from '../../../../utils/types';
import ExternalServersCreateForm from './ExternalServersCreateForm';
import ExternalServersValidationDetails from './ExternalServersValidationDetails';
import type { EndpointValidationResponse } from './externalServersValidationTypes';
import { getErrorMessage, getFieldErrors } from '../../../../utils/apiError';

// Backend field names (from CreateMCPServerRequest) mapped onto this form's state keys.
// "displayName" maps to the server name field; the rest match one-to-one.
const FIELD_NAME_MAP: Record<string, 'name' | 'version' | 'description' | 'context' | 'target'> = {
  displayName: 'name',
  version: 'version',
  description: 'description',
  context: 'context',
};

const SAMPLE_MCP_SERVER_URL = 'https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/mcp-everything-server/v1.0/mcp';

function generateServerId(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/**
 * Normalize a version string to match the required format: v<major>.<minor>
 * e.g. "1.0.0" -> "v1.0", "v2.1" -> "v2.1", "3" -> "v3.0"
 */
function normalizeVersion(version: string): string {
  const stripped = version.replace(/^v/i, '');
  const parts = stripped.split('.');
  const major = parts[0] || '1';
  const minor = parts[1] || '0';
  return `v${major}.${minor}`;
}

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

export default function ExternalServersNew(): JSX.Element {
  const intl = useIntl();
  const navigate = useNavigate();
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const {
    currentProject,
    currentOrganization,
    projectsForCurrentOrganization,
  } = useAppShell();
  const {
    fetchServerInfo,
    isLoading: isValidating,
    error: validationContextError,
    serverInfo,
    reset: resetValidation,
  } = useMCPServerValidation();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [isCreating, setIsCreating] = useState(false);
  const [createFieldErrors, setCreateFieldErrors] = useState<Record<string, string>>({});
  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const [endpointUrl, setEndpointUrl] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);
  const [validationResult, setValidationResult] =
    useState<EndpointValidationResponse | null>(null);
  const [isCreateStep, setIsCreateStep] = useState(false);
  const [authHeaderName, setAuthHeaderName] = useState('');
  const [authHeaderValue, setAuthHeaderValue] = useState('');
  const [showAuthHeaderValue, setShowAuthHeaderValue] = useState(false);
  const [serverName, setServerName] = useState('');
  const [serverVersion, setServerVersion] = useState('');
  const [serverDescription, setServerDescription] = useState('');
  const [serverTarget, setServerTarget] = useState('');
  const [serverContextOverride, setServerContextOverride] = useState<string | null>(null);
  const [lastValidatedUrl, setLastValidatedUrl] = useState('');
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/mcp-proxy'
  );

  const validateEndpoint = async (rawUrl: string) => {
    const normalizedUrl = rawUrl.trim();
    if (!normalizedUrl) {
      setValidationError(null);
      setValidationResult(null);
      setLastValidatedUrl('');
      return;
    }

    setValidationError(null);
    setValidationResult(null);
    setLastValidatedUrl(normalizedUrl);

    const request: MCPServerInfoFetchRequest = {
      url: normalizedUrl,
    };

    if (authHeaderName.trim() && authHeaderValue.trim()) {
      request.auth = {
        type: 'header',
        header: authHeaderName.trim(),
        value: authHeaderValue.trim(),
      };
    }

    try {
      const response = await fetchServerInfo(request);
      setValidationResult({
        endpointUrl: normalizedUrl,
        serverInfo: {
          name: response.serverInfo?.name ?? '',
          version: response.serverInfo?.version ?? '',
        },
        tools: (response.tools ?? []) as unknown as EndpointValidationResponse['tools'],
        resources: (response.resources ?? []) as unknown as EndpointValidationResponse['resources'],
        prompts: (response.prompts ?? []) as unknown as EndpointValidationResponse['prompts'],
      });
    } catch (err) {
      setValidationError(getErrorDescription(err, 'Unknown error'));
    }
  };

  const handleEndpointChange = (value: string) => {
    setEndpointUrl(value);
    if (value.trim() !== lastValidatedUrl) {
      setValidationError(null);
      setValidationResult(null);
    }
  };


  const handleTrySampleUrl = () => {
    setEndpointUrl(SAMPLE_MCP_SERVER_URL);
    void validateEndpoint(SAMPLE_MCP_SERVER_URL);
  };

  const handleNext = () => {
    if (!validationResult) {
      return;
    }

    setServerName((prev) => prev || validationResult.serverInfo.name || '');
    setServerVersion(
      (prev) => prev || normalizeVersion(validationResult.serverInfo.version || 'v1.0')
    );
    setServerTarget((prev) => prev || endpointUrl.trim());
    setIsCreateStep(true);
  };

  const handleCancelCreate = () => {
    setIsCreateStep(false);
  };

  const handleCreate = async () => {
    if (!effectiveProject?.id) return;

    // Encrypt the upstream auth value as a secret so the plaintext credential is
    // never stored in the MCP server config. Skip if already a placeholder.
    let resolvedAuthValue = authHeaderValue.trim();
    if (authHeaderName.trim() && resolvedAuthValue && !resolvedAuthValue.includes('{{ secret ')) {
      try {
        const secretHandle = generateSecretHandle();
        const secretResponse = await createSecret(
          {
            id: secretHandle,
            displayName: `${serverName.trim()} upstream auth`,
            description: `Auto-generated secret for MCP server ${serverName.trim()}`,
            value: resolvedAuthValue,
            type: 'GENERIC',
          },
        );
        resolvedAuthValue = buildSecretPlaceholder(secretResponse.id);
      } catch (err) {
        showSnackbar('Failed to encrypt upstream auth credential', 'error');
        return;
      }
    }

    const payload: CreateMCPServerRequest = {
      id: generateServerId(serverName),
      displayName: serverName.trim(),
      description: serverDescription.trim() || undefined,
      version: normalizeVersion(serverVersion.trim()),
      projectId: effectiveProject.id,
      context: serverContext,
      // vhost: 'mcp.gw.com', --- TODO Remove Tentatively ---
      upstream: {
        main: {
          url: serverTarget.trim().replace(/\/mcp$/, ''),
          ...(authHeaderName.trim() && resolvedAuthValue
            ? {
                auth: {
                  type: 'header',
                  header: authHeaderName.trim(),
                  value: resolvedAuthValue,
                },
              }
            : {}),
        },
      },
      mcpSpecVersion: '2025-06-18',
      kind: 'Mcp',
      policies: [],
      capabilities: {
        tools: (validationResult?.tools ?? []).map((tool) => ({
          name: tool.name,
          description: tool.description,
          inputSchema: tool.inputSchema,
        })),
        resources: (validationResult?.resources ?? []).map((resource) => ({
          name: resource.name ?? '',
          uri: resource.uri,
          mimeType: resource.mimeType,
        })),
        prompts: (validationResult?.prompts ?? []).map((prompt) => ({
          name: prompt.name,
          description: prompt.description,
          arguments: prompt.arguments,
        })),
      },
    };

    try {
      setIsCreating(true);
      setCreateFieldErrors({});
      const created = await mcpProxiesApis.createMCPServer(payload, PLATFORM_API_BASE_URL);
      showSnackbar('Successfully created MCP Proxy.', 'success');
      navigate(
        buildProjectPath(
          currentOrganization,
          effectiveProject,
          `/mcp-proxy/${created.id}`
        )
      );
    } catch (err) {
      const backendFieldErrors = getFieldErrors(err);
      const mappedErrors: Record<string, string> = {};
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
        setCreateFieldErrors(mappedErrors);
      }
      if (hasUnmapped || Object.keys(mappedErrors).length === 0) {
        showSnackbar(
          getErrorDescription(err, 'Failed to create MCP Proxy'),
          'error'
        );
      }
    } finally {
      setIsCreating(false);
    }
  };

  const effectiveProjectSlug = getProjectSlug(effectiveProject);
  const computedContext = effectiveProjectSlug
    ? `/${effectiveProjectSlug}/${generateServerId(serverName)}`
    : `/${generateServerId(serverName)}`;
  const serverContext = serverContextOverride ?? computedContext;

  const isCreateDisabled =
    isCreating ||
    !serverName.trim() ||
    !serverVersion.trim() ||
    !serverTarget.trim();

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
          id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.back.to.create.list"
          defaultMessage="Back to Create List"
        />
      </Button>

      <Stack spacing={2} mt={2} sx={{ maxWidth: 760 }}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.mcp.server.from.endpoint.title"
              defaultMessage="Create MCP Proxy from Endpoint"
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      {isCreateStep ? (
        <ExternalServersCreateForm
          isCreateDisabled={isCreateDisabled}
          serverContext={serverContext}
          serverDescription={serverDescription}
          serverName={serverName}
          serverTarget={serverTarget}
          serverVersion={serverVersion}
          fieldErrors={createFieldErrors}
          onCancel={handleCancelCreate}
          onCreate={handleCreate}
          onContextChange={setServerContextOverride}
          onDescriptionChange={setServerDescription}
          onNameChange={setServerName}
          onTargetChange={setServerTarget}
          onVersionChange={setServerVersion}
        />
      ) : (
        <Grid container spacing={2} sx={{ mt: 1, alignItems: 'flex-start' }}>
          <Grid size={{ xs: 12, md: 5 }}>
            <Card sx={{ p: { xs: 2.5, sm: 3 } }}>
              <Stack spacing={1.5}>
                <FormControl fullWidth>
                  <FormLabel>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.mcp.server.endpoint.url"
                      defaultMessage="MCP Proxy Endpoint URL"
                    />
                  </FormLabel>
                  <TextField
                    fullWidth
                    placeholder={intl.formatMessage({
                      id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.Main.enter.url.of.your.mcp.server',
                      defaultMessage: 'Enter URL of Your MCP Proxy',
                    })}
                    value={endpointUrl}
                    onChange={(event) =>
                      handleEndpointChange(event.target.value)
                    }

                    slotProps={{
                      input: {
                        endAdornment: isValidating ? (
                          <InputAdornment position="end">
                            <CircularProgress size={18} />
                          </InputAdornment>
                        ) : null,
                      },
                    }}
                  />
                </FormControl>

                <Button
                  variant="text"
                  onClick={handleTrySampleUrl}
                  sx={{
                    alignSelf: 'flex-start',
                    px: 0,
                    minWidth: 'auto',
                    py: 0,
                  }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.try.with.sample.url"
                    defaultMessage="Try with Sample URL"
                  />
                </Button>

                <Accordion
                  sx={{ borderRadius: 1, '&:before': { display: 'none' } }}
                >
                  <AccordionSummary expandIcon={<ChevronDown size={18} />}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography
                        sx={{ fontWeight: 500, fontSize: '0.875rem' }}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.advanced.configurations"
                          defaultMessage="Advanced Configurations"
                        />
                      </Typography>
                      <Tooltip title="If the MCP Proxy is protected, ensure to provide the security credentials (Authentication header and value) to authenticate with the server.">
                        <HelpCircle size={16} />
                      </Tooltip>
                    </Stack>
                  </AccordionSummary>
                  <AccordionDetails>
                    <Stack spacing={1.5}>
                      <Typography
                        variant="subtitle2"
                        sx={{ fontWeight: 600, fontSize: '0.8125rem' }}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.configure.authentication.header"
                          defaultMessage="Configure Authentication Header"
                        />
                      </Typography>
                      <Grid container spacing={1.5}>
                        <Grid size={{ xs: 12, sm: 6 }}>
                          <FormControl fullWidth>
                            <FormLabel>
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.authentication.header.label"
                                defaultMessage="Header"
                              />
                            </FormLabel>
                            <TextField
                              fullWidth
                              placeholder={intl.formatMessage({
                                id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.Main.authentication.header.placeholder',
                                defaultMessage: 'Header',
                              })}
                              value={authHeaderName}
                              onChange={(event) =>
                                setAuthHeaderName(event.target.value)
                              }
                            />
                          </FormControl>
                        </Grid>
                        <Grid size={{ xs: 12, sm: 6 }}>
                          <FormControl fullWidth>
                            <FormLabel>
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.authentication.value.label"
                                defaultMessage="Value"
                              />
                            </FormLabel>
                            <TextField
                              fullWidth
                              placeholder={intl.formatMessage({
                                id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.Main.authentication.value.placeholder',
                                defaultMessage: 'Value',
                              })}
                              type={showAuthHeaderValue ? 'text' : 'password'}
                              value={authHeaderValue}
                              onChange={(event) =>
                                setAuthHeaderValue(event.target.value)
                              }
                              slotProps={{
                                input: {
                                  endAdornment: (
                                    <InputAdornment position="end">
                                      <IconButton
                                        size="small"
                                        onClick={() =>
                                          setShowAuthHeaderValue(
                                            (prev) => !prev
                                          )
                                        }
                                        aria-label={
                                          showAuthHeaderValue
                                            ? 'Hide header value'
                                            : 'Show header value'
                                        }
                                      >
                                        {showAuthHeaderValue ? (
                                          <EyeOff size={18} />
                                        ) : (
                                          <Eye size={18} />
                                        )}
                                      </IconButton>
                                    </InputAdornment>
                                  ),
                                },
                              }}
                            />
                          </FormControl>
                        </Grid>
                      </Grid>
                    </Stack>
                  </AccordionDetails>
                </Accordion>

                {validationError ? (
                  <Alert severity="error">{validationError}</Alert>
                ) : null}

              </Stack>
            </Card>

            <Stack direction="row" spacing={1} mt={2}>
              <Button
                variant="outlined"
                component={RouterLink}
                to={listPath}
                color="secondary"
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.cancel"
                  defaultMessage="Cancel"
                />
              </Button>
              {validationResult ? (
                <Button variant="contained" onClick={handleNext}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.next"
                    defaultMessage="Next"
                  />
                </Button>
              ) : (
                <Button
                  variant="contained"
                  disabled={!endpointUrl.trim() || isValidating}
                  onClick={() => validateEndpoint(endpointUrl.trim())}
                >
                  {isValidating ? (
                    'Validating...'
                  ) : (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.fetch.tools"
                      defaultMessage="Fetch Server Info"
                    />
                  )}
                </Button>
              )}
            </Stack>
          </Grid>

          {validationResult ? (
            <Grid size={{ xs: 12, md: 7 }}>
              <ExternalServersValidationDetails
                validationResult={validationResult}
              />
            </Grid>
          ) : null}
        </Grid>
      )}
    </PageContent>
  );
}
