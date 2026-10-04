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

import { useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Box } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import { useMCPServerValidation, MCPServerValidationProvider } from '../../../../../contexts/MCP';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';
import { mcpProxiesApis } from '../../../../../apis/MCP/mcpProxiesApis';
import {
  deployMCPServer,
  getMCPServerDeployment,
} from '../../../../../apis/MCP/mcpServerDeployApis';
import { PLATFORM_API_BASE_URL } from '../../../../../paths';
import type { MCPServerInfoFetchRequest, ProjectBase } from '../../../../../utils/types';
import { buildProjectPath, getProjectSlug } from '../../../../../utils/projectRouting';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import type { ParameterValues } from '../../../PolicyParameterEditor/types';
import type { EndpointValidationResponse } from '../../externalServers/externalServersValidationTypes';
import AILoader from '../../../../../Components/AILoader';
import WizardStepCard from '../LLMProviderQuickStart/WizardStepCard';
import {
  initialGatewayFormState,
} from '../LLMProviderQuickStart/utils';
import type { GatewayFormState } from '../LLMProviderQuickStart/types';
import type { MCPProxyQuickStartStep } from './types';
import type { SelectedPolicy } from '../../externalServers/PolicyMapper';
import EnterEndpointStep, { UNREACHABLE_URL_ERROR } from './EnterEndpointStep';
import ConfigureMCPStep from './ConfigureMCPStep';
import DeploymentFail from './DeploymentFail';

const STEPS: MCPProxyQuickStartStep[] = ['enter-endpoint', 'configure-mcp'];

const DEPLOYMENT_TERMINAL_STATUSES = ['DEPLOYED', 'UNDEPLOYED', 'ARCHIVED', 'FAILED'];
const DEPLOYMENT_POLL_INTERVAL_MS = 3000;
const DEPLOYMENT_POLL_MAX_ATTEMPTS = 40;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

type DeployResult = { success: boolean; reason?: string };

async function deployMcpServerToGateway(
  mcpServerId: string,
  organizationId: string,
  gatewayId: string
): Promise<DeployResult> {
  const deploymentName = `${gatewayId.trim().replace(/\s+/g, '_') || 'gateway'}_${new Date()
    .toISOString()
    .slice(0, 10)}_1`;

  const result = await deployMCPServer(
    mcpServerId,
    {
      name: deploymentName,
      base: 'current',
      gatewayId,
      metadata: { host: '' },
    },
    PLATFORM_API_BASE_URL
  );

  if (DEPLOYMENT_TERMINAL_STATUSES.includes(result.status)) {
    return result.status === 'DEPLOYED'
      ? { success: true }
      : {
          success: false,
          reason: result.statusReason || `Deployment ${result.status.toLowerCase()}.`,
        };
  }

  for (let attempt = 0; attempt < DEPLOYMENT_POLL_MAX_ATTEMPTS; attempt += 1) {
    await sleep(DEPLOYMENT_POLL_INTERVAL_MS);
    const updated = await getMCPServerDeployment(
      mcpServerId,
      result.deploymentId,
      PLATFORM_API_BASE_URL
    );
    if (DEPLOYMENT_TERMINAL_STATUSES.includes(updated.status)) {
      return updated.status === 'DEPLOYED'
        ? { success: true }
        : {
            success: false,
            reason: updated.statusReason || `Deployment ${updated.status.toLowerCase()}.`,
          };
    }
  }

  return {
    success: false,
    reason: 'Deployment timed out. Gateway did not respond.',
  };
}

function generateServerId(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function normalizeVersion(version: string): string {
  const stripped = version.replace(/^v/i, '');
  const parts = stripped.split('.');
  const major = parts[0] || '1';
  const minor = parts[1] || '0';
  return `v${major}.${minor}`;
}

function getErrorDescription(error: unknown, fallback: string): string {
  const responseData = (error as { response?: { data?: { description?: unknown; message?: unknown } } })?.response?.data;
  const description = responseData?.description;
  const message = responseData?.message;
  if (typeof description === 'string' && description.trim()) return description;
  if (typeof message === 'string' && message.trim()) return message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

type ResourceType = 'provider' | 'mcp';

type MCPProxyQuickStartProps = {
  role: string;
  resourceType: ResourceType;
  onResourceTypeChange: (type: ResourceType) => void;
  onClose: () => void;
};

function MCPProxyQuickStartInner({
  role,
  resourceType,
  onResourceTypeChange,
  onClose,
}: MCPProxyQuickStartProps) {
  const navigate = useNavigate();
  const { currentOrganization, projectsForCurrentOrganization } = useAppShell();
  const { fetchServerInfo, isLoading: isValidating } = useMCPServerValidation();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [currentStepIndex, setCurrentStepIndex] = useState(0);

  // Step 1 state
  const [endpointUrl, setEndpointUrl] = useState('');
  const [authHeaderName, setAuthHeaderName] = useState('');
  const [authHeaderValue, setAuthHeaderValue] = useState('');
  const [validationResult, setValidationResult] = useState<EndpointValidationResponse | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [lastValidatedUrl, setLastValidatedUrl] = useState('');

  // Step 2 state
  const [selectedProjectId, setSelectedProjectId] = useState(
    () => projectsForCurrentOrganization[0]?.id ?? ''
  );
  const [serverName, setServerName] = useState('');
  const [serverVersion, setServerVersion] = useState('v1.0');
  const [serverDescription, setServerDescription] = useState('');
  const [serverContextOverride, setServerContextOverride] = useState<string | null>(null);
  const [serverTarget, setServerTarget] = useState('');
  const [selectedPolicies, setSelectedPolicies] = useState<SelectedPolicy[]>([]);
  const [isCreating, setIsCreating] = useState(false);
  const [isDeployingMcp, setIsDeployingMcp] = useState(false);
  const [deploymentError, setDeploymentError] = useState<string | null>(null);
  const [serverSavedAt, setServerSavedAt] = useState<string | null>(null);
  const [createdServerId, setCreatedServerId] = useState<string | null>(null);

  // Gateway state
  const [gatewayFormState, setGatewayFormState] = useState<GatewayFormState>(initialGatewayFormState);
  const [preferredGatewayId, setPreferredGatewayId] = useState<string | null>(null);
  const [createdGateway, setCreatedGateway] = useState<HybridGateway | null>(null);
  const [gatewayRegistrationToken, setGatewayRegistrationToken] = useState<string | null>(null);
  const [isGatewaySetupReady, setIsGatewaySetupReady] = useState(false);

  const currentStep = STEPS[currentStepIndex];
  const organizationId = currentOrganization?.uuid ?? '';

  const selectedProject =
    projectsForCurrentOrganization.find((p) => p.id === selectedProjectId) ?? null;

  const effectiveProjectSlug = getProjectSlug(selectedProject);
  const computedContext = effectiveProjectSlug
    ? `/${effectiveProjectSlug}/${generateServerId(serverName)}`
    : `/${generateServerId(serverName)}`;
  const serverContext = serverContextOverride ?? computedContext;

  const isStep2Valid =
    Boolean(serverName.trim()) &&
    Boolean(serverVersion.trim()) &&
    Boolean(serverTarget.trim()) &&
    Boolean(selectedProjectId);

  // Validation
  const handleValidate = async (rawUrl: string) => {
    const normalizedUrl = rawUrl.trim();
    if (!normalizedUrl) return;

    setValidationError(null);
    setValidationResult(null);
    setLastValidatedUrl(normalizedUrl);

    const request: MCPServerInfoFetchRequest = { url: normalizedUrl };
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
        tools: (response.tools ?? []) as EndpointValidationResponse['tools'],
        resources: (response.resources ?? []) as EndpointValidationResponse['resources'],
        prompts: (response.prompts ?? []) as EndpointValidationResponse['prompts'],
      });
      // Pre-fill server name/version/target from validation
      setServerName((prev) => prev || response.serverInfo?.name || '');
      setServerVersion((prev) =>
        prev !== 'v1.0'
          ? prev
          : normalizeVersion(response.serverInfo?.version || 'v1.0')
      );
      setServerTarget((prev) => prev || normalizedUrl.trim());
    } catch (err) {
      setValidationError(getErrorDescription(err, 'Unknown error'));
      setServerTarget((prev) => prev || normalizedUrl.trim());
    }
  };

  // Step 1 → step 2
  const handleStep1Next = () => {
    if (!serverTarget) {
      setServerTarget(endpointUrl.trim());
    }
    setCurrentStepIndex(1);
  };

  // Policy management
  const handleAddPolicy = (policy: Omit<SelectedPolicy, 'instanceId'>) => {
    setSelectedPolicies((prev) => [
      ...prev,
      { ...policy, instanceId: crypto.randomUUID() },
    ]);
  };
  const handleUpdatePolicy = (instanceId: string, params: ParameterValues) => {
    setSelectedPolicies((prev) =>
      prev.map((p) => (p.instanceId === instanceId ? { ...p, params } : p))
    );
  };
  const handleRemovePolicy = (instanceId: string) => {
    setSelectedPolicies((prev) => prev.filter((p) => p.instanceId !== instanceId));
  };
  const handleReorderPolicies = (draggedId: string, targetId: string) => {
    setSelectedPolicies((prev) => {
      const draggedIndex = prev.findIndex((p) => p.instanceId === draggedId);
      const targetIndex = prev.findIndex((p) => p.instanceId === targetId);
      if (draggedIndex === -1 || targetIndex === -1) return prev;
      const next = [...prev];
      const [dragged] = next.splice(draggedIndex, 1);
      next.splice(targetIndex, 0, dragged);
      return next;
    });
  };

  const navigateToOverview = (serverId: string) => {
    if (!selectedProject) return;
    navigate(buildProjectPath(currentOrganization, selectedProject, `/mcp-proxy/${serverId}`));
  };

  const runDeployment = async (mcpServerId: string, gatewayId: string): Promise<boolean> => {
    if (!organizationId) {
      return false;
    }

    try {
      setIsDeployingMcp(true);
      const result = await deployMcpServerToGateway(mcpServerId, organizationId, gatewayId);
      if (!result.success) {
        const reason =
          result.reason || 'Failed to deploy the MCP Server to the selected gateway.';
        setDeploymentError(reason);
        showSnackbar(reason, 'error');
        return false;
      }
      setDeploymentError(null);
      navigateToOverview(mcpServerId);
      return true;
    } catch (error: unknown) {
      const reason = getErrorDescription(
        error,
        'Failed to deploy the MCP Server to the selected gateway.'
      );
      setDeploymentError(reason);
      showSnackbar(reason, 'error');
      return false;
    } finally {
      setIsDeployingMcp(false);
    }
  };

  // Create or update the MCP server, then deploy it to the selected gateway
  const handleCreateMCPServer = async () => {
    if (!selectedProject?.id || !isStep2Valid) return;

    const gatewayId = createdGateway?.id ?? preferredGatewayId;
    let serverId = createdServerId;
    const isNewServer = !serverId;

    const commonFields = {
      displayName: serverName.trim(),
      description: serverDescription.trim() || undefined,
      version: normalizeVersion(serverVersion.trim()),
      projectId: selectedProject.id,
      context: serverContext,
      upstream: {
        main: {
          url: serverTarget.trim().replace(/\/mcp$/, ''),
          ...(authHeaderName.trim() && authHeaderValue.trim()
            ? { auth: { type: 'header', header: authHeaderName.trim(), value: authHeaderValue.trim() } }
            : {}),
        },
      },
      kind: 'Mcp',
      policies: selectedPolicies.map((p) => ({
        name: p.policyName,
        version: p.version,
        params: p.params ?? {},
      })),
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
      if (serverId) {
        await mcpProxiesApis.updateMCPServer(
          serverId,
          commonFields,
          PLATFORM_API_BASE_URL
        );
      } else {
        const created = await mcpProxiesApis.createMCPServer(
          {
            id: generateServerId(serverName),
            mcpSpecVersion: '2025-06-18',
            ...commonFields,
          },
          PLATFORM_API_BASE_URL
        );
        serverId = created.id;
        setCreatedServerId(serverId);
      }
      setServerSavedAt(new Date().toISOString());
      if (isNewServer) {
        showSnackbar(`'${serverName.trim()}' successfully created.`, 'success');
      }
    } catch (err) {
      showSnackbar(getErrorDescription(err, 'Failed to save MCP Proxy'), 'error');
      return;
    } finally {
      setIsCreating(false);
    }

    if (!gatewayId) {
      navigateToOverview(serverId);
      return;
    }

    await runDeployment(serverId, gatewayId);
  };

  const handleRedeploy = async () => {
    const gatewayId = createdGateway?.id ?? preferredGatewayId;
    if (!createdServerId || !gatewayId) {
      return;
    }
    await runDeployment(createdServerId, gatewayId);
  };

  const stepMeta: {
    title: string;
    description: string;
    nextLabel: string;
    nextDisabled?: boolean;
    nextLoading?: boolean;
    nextIcon?: ReactNode;
    onNext?: () => void;
    hideBack?: boolean;
    backDisabled?: boolean;
  } = (() => {
    switch (currentStep) {
      case 'enter-endpoint':
        return {
          title: 'Create MCP Server',
          description: 'Enter the MCP Proxy endpoint URL and fetch server capabilities.',
          nextLabel: 'Next',
          nextDisabled:
            isValidating ||
            (validationResult === null && validationError !== UNREACHABLE_URL_ERROR),
          onNext: handleStep1Next,
          hideBack: true,
        };
      case 'configure-mcp':
        return deploymentError
          ? {
              title: 'Configure MCP Server',
              description: "Edit and save your server's details, and add policies for using it.",
              nextLabel: 'Re-Deploy',
              nextIcon: <RefreshCw size={16} />,
              nextDisabled: !isGatewaySetupReady,
              onNext: () => { void handleRedeploy(); },
              nextLoading: isDeployingMcp,
              backDisabled: isDeployingMcp,
            }
          : {
              title: 'Configure MCP Server',
              description: "Edit and save your server's details, and add policies for using it.",
              nextLabel: 'Finish',
              nextDisabled: !isStep2Valid || !isGatewaySetupReady,
              nextLoading: isCreating || isDeployingMcp,
              onNext: () => { void handleCreateMCPServer(); },
              backDisabled: isCreating || isDeployingMcp,
            };
      default:
        return {
          title: '',
          description: '',
          nextLabel: 'Next',
        };
    }
  })();

  let stepContent: ReactNode = null;

  if (currentStep === 'enter-endpoint') {
    stepContent = (
      <EnterEndpointStep
        role={role}
        resourceType={resourceType}
        onResourceTypeChange={onResourceTypeChange}
        endpointUrl={endpointUrl}
        authHeaderName={authHeaderName}
        authHeaderValue={authHeaderValue}
        validationResult={validationResult}
        validationError={validationError}
        isValidating={isValidating}
        lastValidatedUrl={lastValidatedUrl}
        onEndpointUrlChange={setEndpointUrl}
        onAuthHeaderNameChange={setAuthHeaderName}
        onAuthHeaderValueChange={setAuthHeaderValue}
        onValidationResultChange={setValidationResult}
        onValidationErrorChange={setValidationError}
        onLastValidatedUrlChange={setLastValidatedUrl}
        onValidate={handleValidate}
      />
    );
  } else if (currentStep === 'configure-mcp') {
    stepContent = isDeployingMcp ? (
      <AILoader label="We are deploying your MCP Server to the gateway..." />
    ) : isCreating ? (
      <AILoader
        label={
          createdServerId
            ? 'We are in the process of updating your MCP Server...'
            : 'We are in the process of creating your MCP Server...'
        }
      />
    ) : deploymentError ? (
      <DeploymentFail
        serverName={serverName}
        serverVersion={serverVersion}
        serverDescription={serverDescription}
        serverContext={serverContext}
        serverSavedAt={serverSavedAt}
        deploymentError={deploymentError}
        onEditDetails={() => setDeploymentError(null)}
        gatewayFormState={gatewayFormState}
        setGatewayFormState={setGatewayFormState}
        preferredGatewayId={preferredGatewayId}
        onPreferredGatewayChange={setPreferredGatewayId}
        createdGateway={createdGateway}
        onGatewayCreated={setCreatedGateway}
        onGatewayChange={setCreatedGateway}
        gatewayRegistrationToken={gatewayRegistrationToken}
        onRegistrationTokenChange={setGatewayRegistrationToken}
        onGatewayReadyChange={setIsGatewaySetupReady}
      />
    ) : (
      <ConfigureMCPStep
        projects={projectsForCurrentOrganization as ProjectBase[]}
        selectedProjectId={selectedProjectId}
        onProjectChange={setSelectedProjectId}
        serverName={serverName}
        serverVersion={serverVersion}
        serverDescription={serverDescription}
        serverContext={serverContext}
        serverTarget={serverTarget}
        onNameChange={setServerName}
        onVersionChange={setServerVersion}
        onDescriptionChange={setServerDescription}
        onContextChange={setServerContextOverride}
        onTargetChange={setServerTarget}
        selectedPolicies={selectedPolicies}
        onAddPolicy={handleAddPolicy}
        onUpdatePolicy={handleUpdatePolicy}
        onRemovePolicy={handleRemovePolicy}
        onReorderPolicies={handleReorderPolicies}
        validationResult={validationResult}
        gatewayFormState={gatewayFormState}
        setGatewayFormState={setGatewayFormState}
        preferredGatewayId={preferredGatewayId}
        onPreferredGatewayChange={setPreferredGatewayId}
        createdGateway={createdGateway}
        onGatewayCreated={setCreatedGateway}
        onGatewayChange={setCreatedGateway}
        gatewayRegistrationToken={gatewayRegistrationToken}
        onRegistrationTokenChange={setGatewayRegistrationToken}
        onGatewayReadyChange={setIsGatewaySetupReady}
      />
    );
  }

  return (
    <Box
      sx={{
        width: '100%',
        maxWidth: 1320,
        mx: 'auto',
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <WizardStepCard
        currentStep={currentStepIndex}
        totalSteps={STEPS.length}
        title={stepMeta.title}
        description={stepMeta.description}
        onSkip={onClose}
        onBack={
          currentStepIndex > 0
            ? () => setCurrentStepIndex((prev) => Math.max(0, prev - 1))
            : undefined
        }
        onNext={stepMeta.onNext}
        nextLabel={stepMeta.nextLabel}
        nextDisabled={stepMeta.nextDisabled}
        nextLoading={stepMeta.nextLoading ?? false}
        nextIcon={stepMeta.nextIcon}
        hideBack={stepMeta.hideBack}
        backDisabled={stepMeta.backDisabled ?? false}
      >
        {stepContent}
      </WizardStepCard>
    </Box>
  );
}

export default function MCPProxyQuickStart(props: MCPProxyQuickStartProps) {
  return (
    <MCPServerValidationProvider>
      <MCPProxyQuickStartInner {...props} />
    </MCPServerValidationProvider>
  );
}
