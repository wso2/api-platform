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
  Alert,
  Button,
  Card,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useAgentProxies } from '../../../../contexts/agentProxy';
import {
  buildProjectPath,
  getProjectSlug,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { agentProxiesApis } from '../../../../apis/agent/agentProxiesApis';
import { getErrorMessage } from '../../../../utils/apiError';
import type {
  AgentCardDocument,
  A2ATransport,
  CreateAgentProxyRequest,
} from '../../../../utils/types';
import AgentProxyCardDetails from './AgentProxyCardDetails';
import AgentProxiesCreateForm, {
  AGENT_TRANSPORT_OPTIONS,
} from './AgentProxiesCreateForm';
import { isValidHttpUrl } from '../../../../utils/providerTemplateFields';

export const AGENT_VERSION_PATTERN = /^v\d+\.\d+$/;
export const AGENT_VERSION_ERROR = 'Enter a valid version (e.g., v1.0)';
export const AGENT_TARGET_ERROR = 'The provided URL is invalid.';
const DEFAULT_AGENT_VERSION = 'v1.0';

function generateAgentProxyId(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

const SUPPORTED_BINDINGS = AGENT_TRANSPORT_OPTIONS.map(
  (transport) => transport.protocolBinding
);

/** Path each binding is served on, read off the card's interface URLs. */
function advertisedPaths(card: AgentCardDocument): Record<string, string> {
  const interfaces = Array.isArray(card.supportedInterfaces)
    ? card.supportedInterfaces
    : [];
  const paths: Record<string, string> = {};
  interfaces.forEach((entry) => {
    const record = entry as Record<string, unknown> | null;
    const binding = asString(record?.protocolBinding);
    const url = asString(record?.url);
    if (!binding || !url) return;
    try {
      const path = new URL(url).pathname.replace(/\/+$/, '');
      if (path) paths[binding] = path;
    } catch {
      // a card with an unparseable URL keeps the default path
    }
  });
  return paths;
}

/** Supported bindings the card advertises, narrowed to those a proxy can expose. */
function advertisedBindings(card: AgentCardDocument): string[] {
  const interfaces = Array.isArray(card.supportedInterfaces)
    ? card.supportedInterfaces
    : [];
  const advertised = interfaces.map((entry) =>
    asString((entry as Record<string, unknown> | null)?.protocolBinding)
  );
  return SUPPORTED_BINDINGS.filter((binding) => advertised.includes(binding));
}

function getErrorDescription(error: unknown, fallback: string): string {
  return getErrorMessage(error, fallback);
}

export default function AgentProxiesNew(): React.JSX.Element {
  const intl = useIntl();
  const navigate = useNavigate();
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const { currentProject, currentOrganization, projectsForCurrentOrganization } =
    useAppShell();
  const { createAgentProxy } = useAgentProxies();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const canCreateAgentProxy = hasPermission(SCOPES.AGENT_PROXY_CREATE);

  const routeProject = useMemo(
    () =>
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null,
    [projectSlug, projectsForCurrentOrganization]
  );
  const effectiveProject = routeProject ?? currentProject;
  const effectiveProjectSlug = effectiveProject
    ? getProjectSlug(effectiveProject)
    : '';
  const listPath = buildProjectPath(
    currentOrganization,
    effectiveProject,
    '/agent-proxy'
  );
  const apimBaseUrl = PLATFORM_API_BASE_URL;

  const [endpointUrl, setEndpointUrl] = useState('');
  const [isFetching, setIsFetching] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [urlError, setUrlError] = useState<string | null>(null);
  const [agentCard, setAgentCard] = useState<AgentCardDocument | null>(null);
  const [lastFetchedUrl, setLastFetchedUrl] = useState('');
  const [isCreateStep, setIsCreateStep] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [agentName, setAgentName] = useState('');
  const [agentVersion, setAgentVersion] = useState('');
  const [agentDescription, setAgentDescription] = useState('');
  const [agentTarget, setAgentTarget] = useState('');
  const [agentContextOverride, setAgentContextOverride] = useState<string | null>(
    null
  );
  const [selectedTransports, setSelectedTransports] =
    useState<string[]>(SUPPORTED_BINDINGS);
  const [transportPaths, setTransportPaths] = useState<Record<string, string>>(
    () =>
      Object.fromEntries(
        AGENT_TRANSPORT_OPTIONS.map((transport) => [
          transport.protocolBinding,
          transport.pathPrefix,
        ])
      )
  );

  const computedContext = effectiveProjectSlug
    ? `/${effectiveProjectSlug}/${generateAgentProxyId(agentName)}`
    : `/${generateAgentProxyId(agentName)}`;
  const agentContext = agentContextOverride ?? computedContext;
  const versionValidationError =
    agentVersion.trim() && !AGENT_VERSION_PATTERN.test(agentVersion.trim())
      ? AGENT_VERSION_ERROR
      : undefined;
  const targetValidationError =
    agentTarget.trim() && !isValidHttpUrl(agentTarget.trim())
      ? AGENT_TARGET_ERROR
      : undefined;

  const fetchCard = async (rawUrl: string) => {
    const normalizedUrl = rawUrl.trim();
    if (!normalizedUrl) return;

    if (!isValidHttpUrl(normalizedUrl)) {
      setUrlError(AGENT_TARGET_ERROR);
      setFetchError(null);
      setAgentCard(null);
      return;
    }

    setIsFetching(true);
    setUrlError(null);
    setFetchError(null);
    setAgentCard(null);
    setLastFetchedUrl(normalizedUrl);

    try {
      const card = await agentProxiesApis.fetchAgentCard(
        { url: normalizedUrl },
        apimBaseUrl
      );
      setAgentCard(card);
      setSelectedTransports(advertisedBindings(card));
      setTransportPaths((prev) => ({ ...prev, ...advertisedPaths(card) }));
    } catch {
      setFetchError(
        'Could not reach the upstream agent to retrieve its Agent Card. ' +
          'You can continue and create the agent proxy anyway.'
      );
      setSelectedTransports(SUPPORTED_BINDINGS);
    } finally {
      setIsFetching(false);
    }
  };

  const handleEndpointChange = (value: string) => {
    setEndpointUrl(value);
    setUrlError(null);
    if (value.trim() !== lastFetchedUrl) {
      setFetchError(null);
      setAgentCard(null);
    }
  };

  const handleNext = () => {
    setAgentName((prev) => prev || asString(agentCard?.name));
    setAgentVersion((prev) => prev || DEFAULT_AGENT_VERSION);
    setAgentDescription((prev) => prev || asString(agentCard?.description));
    setAgentTarget((prev) => prev || endpointUrl.trim());
    setIsCreateStep(true);
  };

  const handleTransportToggle = (protocolBinding: string) => {
    setSelectedTransports((prev) =>
      prev.includes(protocolBinding)
        ? prev.filter((binding) => binding !== protocolBinding)
        : [...prev, protocolBinding]
    );
  };

  const handleCreate = async () => {
    if (!effectiveProject?.id || versionValidationError || targetValidationError)
      return;

    const transports: A2ATransport[] = AGENT_TRANSPORT_OPTIONS.filter(
      (transport) => selectedTransports.includes(transport.protocolBinding)
    ).map((transport) => ({
      protocolBinding: transport.protocolBinding,
      pathPrefix:
        transportPaths[transport.protocolBinding] ?? transport.pathPrefix,
    }));

    // An unset agentCard serves the public card passthrough with URL rewriting.
    const payload: CreateAgentProxyRequest = {
      id: generateAgentProxyId(agentName),
      displayName: agentName.trim(),
      description: agentDescription.trim() || undefined,
      version: agentVersion.trim(),
      projectId: effectiveProject.id,
      context: agentContext,
      upstream: {
        main: {
          url: agentTarget.trim(),
        },
      },
      kind: 'AgentProxy',
      protocol: 'a2a',
      a2a: {
        protocolVersion: '1.0',
        operationConfigs: {
          transports,
        },
      },
    };

    try {
      setIsCreating(true);
      const created = await createAgentProxy(payload);
      showSnackbar('Agent Proxy created successfully.', 'success');
      navigate(
        buildProjectPath(
          currentOrganization,
          effectiveProject,
          `/agent-proxy/${created.id ?? payload.id}`
        )
      );
    } catch (error) {
      showSnackbar(
        getErrorDescription(error, 'Failed to create Agent Proxy.'),
        'error'
      );
    } finally {
      setIsCreating(false);
    }
  };

  const hasFetchAttempt = Boolean(agentCard) || Boolean(fetchError);

  const isCreateDisabled =
    isCreating ||
    !canCreateAgentProxy ||
    !agentName.trim() ||
    !agentVersion.trim() ||
    Boolean(versionValidationError) ||
    !agentTarget.trim() ||
    Boolean(targetValidationError) ||
    selectedTransports.length === 0;

  if (!canCreateAgentProxy) {
    return (
      <PageContent fullWidth>
        <Stack spacing={1}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.creation.unavailable"
              defaultMessage={'Agent Proxy creation is unavailable.'}
            />
          </Typography>
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.creation.unavailable.description"
              defaultMessage={
                'You do not have permission to create Agent Proxies. Please contact your admin.'
              }
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  return (
    <PageContent fullWidth>
      {isCreateStep ? (
        <Button
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ px: 0, minWidth: 'auto' }}
          onClick={() => setIsCreateStep(false)}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.back.to.connect"
            defaultMessage="Back to Connect"
          />
        </Button>
      ) : (
        <Button
          component={RouterLink}
          to={listPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ px: 0, minWidth: 'auto' }}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.back.to.list"
            defaultMessage="Back to List"
          />
        </Button>
      )}

      <Stack spacing={2} mt={2} sx={{ maxWidth: 760 }}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.agent.proxy.title"
              defaultMessage="Create Agent Proxy"
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      {isCreateStep ? (
        <AgentProxiesCreateForm
          isCreateDisabled={isCreateDisabled}
          agentContext={agentContext}
          agentDescription={agentDescription}
          agentName={agentName}
          agentTarget={agentTarget}
          agentVersion={agentVersion}
          selectedTransports={selectedTransports}
          transportPaths={transportPaths}
          onTransportPathChange={(binding, path) =>
            setTransportPaths((prev) => ({ ...prev, [binding]: path }))
          }
          fieldErrors={{
            version: versionValidationError,
            target: targetValidationError,
          }}
          onCancel={() => setIsCreateStep(false)}
          onCreate={handleCreate}
          onContextChange={setAgentContextOverride}
          onDescriptionChange={setAgentDescription}
          onNameChange={setAgentName}
          onTargetChange={setAgentTarget}
          onVersionChange={setAgentVersion}
          onTransportToggle={handleTransportToggle}
        />
      ) : (
        <Grid container spacing={2} sx={{ mt: 1, alignItems: 'flex-start' }}>
          <Grid size={{ xs: 12, md: 5 }}>
            <Card sx={{ p: { xs: 2.5, sm: 3 } }}>
              <Stack spacing={1.5}>
                <FormControl fullWidth>
                  <FormLabel>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.agent.url"
                      defaultMessage="Agent URL"
                    />
                  </FormLabel>
                  <TextField
                    fullWidth
                    placeholder={intl.formatMessage({
                      id: 'aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.enter.url.of.your.a2a.agent',
                      defaultMessage: 'Enter the URL of your A2A agent',
                    })}
                    value={endpointUrl}
                    onChange={(event) =>
                      handleEndpointChange(event.target.value)
                    }
                    slotProps={{
                      input: {
                        endAdornment: isFetching ? (
                          <InputAdornment position="end">
                            <CircularProgress size={18} />
                          </InputAdornment>
                        ) : null,
                      },
                    }}
                  />
                </FormControl>

                {fetchError || urlError ? (
                  <Alert severity={urlError ? 'error' : 'warning'}>
                    {fetchError ?? urlError}
                  </Alert>
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
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.cancel"
                  defaultMessage="Cancel"
                />
              </Button>
              {hasFetchAttempt ? (
                <Button variant="contained" onClick={handleNext}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.next"
                    defaultMessage="Next"
                  />
                </Button>
              ) : (
                <Button
                  variant="contained"
                  disabled={!endpointUrl.trim() || isFetching}
                  onClick={() => fetchCard(endpointUrl.trim())}
                >
                  {isFetching ? (
                    'Fetching...'
                  ) : (
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.fetch.agent.info"
                      defaultMessage="Fetch Agent Info"
                    />
                  )}
                </Button>
              )}
            </Stack>
          </Grid>

          {agentCard ? (
            <Grid size={{ xs: 12, md: 7 }}>
              <AgentProxyCardDetails agentCard={agentCard} />
            </Grid>
          ) : null}
        </Grid>
      )}
    </PageContent>
  );
}
