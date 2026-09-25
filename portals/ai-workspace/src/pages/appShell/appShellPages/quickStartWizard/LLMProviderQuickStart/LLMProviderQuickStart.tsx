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

import { useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Box } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';
import {
  GatewayDeployProvider,
} from '../../../../../contexts/GatewayDeployContext';
import {
  LLMProviderProvider,
  useLLMProviders,
  useProviderTemplates,
} from '../../../../../contexts/llmProvider';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { useRole } from '../../../../../contexts/RoleContext';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';
import { buildOrgPath, getOrgSlug } from '../../../../../utils/projectRouting';
import { dismissQuickStart } from '../../../../../utils/quickStartUtils';
import type { ProviderTemplate } from '../../../../../utils/types';
import { getQuickStartProviderTemplates, isComingSoonTemplate } from '../providerTemplateVisuals';
import AILoader from '../../../../../Components/AILoader';
import {
  deployLLMProvider,
  getLLMProviderDeployment,
} from '../../../../../apis/llmProviderApis';
import ConfigureProviderStep from './ConfigureProviderStep';
import DeploymentFail from './DeploymentFail';
import SelectProviderTemplateStep from './SelectProviderTemplateStep';
import TestLLMProviderStep from './TestLLMProviderStep';
import type {
  GatewayFormState,
  LLMProviderQuickStartStep,
  ResourceType,
} from './types';
import type { GuardrailSelection } from '../../serviceProvider/AddNewProvider/serviceProviderTypes';
import WizardStepCard from './WizardStepCard';
import {
  buildProviderCreateRequest,
  buildProviderUpdateRequest,
  initialGatewayFormState,
  initialProviderFormState,
  VERSION_PATTERN,
} from './utils';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import { PLATFORM_API_BASE_URL } from '../../../../../paths';

const STEPS: LLMProviderQuickStartStep[] = [
  'select-template',
  'configure-provider',
  'test-provider',
];

const DEPLOYMENT_TERMINAL_STATUSES = ['DEPLOYED', 'UNDEPLOYED', 'ARCHIVED', 'FAILED'];
const DEPLOYMENT_POLL_INTERVAL_MS = 3000;
const DEPLOYMENT_POLL_MAX_ATTEMPTS = 40;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

type DeployResult = { success: boolean; reason?: string };

async function deployProviderToGateway(
  providerId: string,
  organizationId: string,
  gatewayId: string
): Promise<DeployResult> {
  const deploymentName = `${gatewayId.trim().replace(/\s+/g, '_') || 'gateway'}_${new Date()
    .toISOString()
    .slice(0, 10)}_1`;

  const result = await deployLLMProvider(
    providerId,
    organizationId,
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
    const updated = await getLLMProviderDeployment(
      providerId,
      result.deploymentId,
      organizationId,
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

type LLMProviderQuickStartProps = {
  resourceType: ResourceType;
  onResourceTypeChange: (resourceType: ResourceType) => void;
};

function StepWithProviderContexts({
  providerId,
  children,
}: {
  providerId: string;
  children: ReactNode;
}) {
  return (
    <LLMProviderProvider providerId={providerId}>
      <GatewayDeployProvider apiId={providerId}>{children}</GatewayDeployProvider>
    </LLMProviderProvider>
  );
}

export default function LLMProviderQuickStart({
  resourceType,
  onResourceTypeChange,
}: LLMProviderQuickStartProps) {
  const navigate = useNavigate();
  const { role } = useRole();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { createProvider, updateProvider } = useLLMProviders();
  const {
    templatesResponse,
    isLoading: templatesLoading,
    error: templatesError,
    refreshTemplates,
  } = useProviderTemplates();
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [resolvedTemplate, setResolvedTemplate] = useState<ProviderTemplate | null>(null);
  const [openapiSpec, setOpenapiSpec] = useState('');
  const [formState, setFormState] = useState(initialProviderFormState);
  const [showCredential, setShowCredential] = useState(false);
  const [guardrails, setGuardrails] = useState<GuardrailSelection[]>([]);
  const [guardrailDrawerOpen, setGuardrailDrawerOpen] = useState(false);
  const [selectedGuardrail, setSelectedGuardrail] = useState<string | null>(null);
  const [guardrailSettings, setGuardrailSettings] = useState<Record<string, unknown>>({});
  const [createdProviderId, setCreatedProviderId] = useState<string | null>(null);
  const [preferredGatewayId, setPreferredGatewayId] = useState<string | null>(null);
  const [createdGateway, setCreatedGateway] = useState<HybridGateway | null>(null);
  const [gatewayRegistrationToken, setGatewayRegistrationToken] = useState<string | null>(null);
  const [gatewayFormState, setGatewayFormState] =
    useState<GatewayFormState>(initialGatewayFormState);
  const [isSubmittingProvider, setIsSubmittingProvider] = useState(false);
  const [isDeployingProvider, setIsDeployingProvider] = useState(false);
  const [deploymentError, setDeploymentError] = useState<string | null>(null);
  const [providerSavedAt, setProviderSavedAt] = useState<string | null>(null);
  const [isGatewaySetupReady, setIsGatewaySetupReady] = useState(false);
  const [isTestingReady, setIsTestingReady] = useState(false);

  const currentStep = STEPS[currentStepIndex];
  const orgHomePath = buildOrgPath(currentOrganization, '/home');
  const sortedTemplates = useMemo(
    () => getQuickStartProviderTemplates(templatesResponse.list ?? []),
    [templatesResponse.list]
  );
  const selectedTemplate =
    resolvedTemplate ??
    templatesResponse.list.find((template) => template.id === selectedTemplateId) ??
    null;

  const isConfigureFormValid = Boolean(
    selectedTemplateId &&
      formState.name.trim() &&
      VERSION_PATTERN.test(formState.version.trim()) &&
      formState.upstreamUrl.trim()
  );

  const handleSkip = () => {
    dismissQuickStart(getOrgSlug(currentOrganization));
    navigate(orgHomePath);
  };

  const handleSelectTemplate = (template: ProviderTemplate) => {
    setSelectedTemplateId(template.id ?? null);
    setResolvedTemplate(null);
  };

  const handleOpenGuardrailDrawer = () => {
    setSelectedGuardrail(null);
    setGuardrailSettings({});
    setGuardrailDrawerOpen(true);
  };

  const handleSelectGuardrail = (guardrailName: string) => {
    setSelectedGuardrail(guardrailName);
    const existing = guardrails.find((item) => item.name === guardrailName);
    setGuardrailSettings(existing?.settings ?? {});
  };

  // Mirrors ServiceProviderNew's handlers so the shared GuardrailsSection
  // behaves identically here: entries are appended (never merged by name) so
  // the same guardrail can be attached more than once, each keyed by its own id.
  const handleAddGuardrail = (
    guardrail: { name: string; version: string },
    settings: Record<string, unknown>
  ) => {
    setGuardrailSettings(settings);
    setGuardrails((prev) => {
      const configurationSummary = Object.entries(settings)
        .filter(([, value]) => Boolean(value))
        .map(([key, value]) => {
          if (
            typeof value === 'string' ||
            typeof value === 'number' ||
            typeof value === 'boolean'
          ) {
            return `${key}: ${value}`;
          }
          if (value === null || value === undefined) {
            return `${key}:`;
          }
          return `${key}: ${JSON.stringify(value)}`;
        })
        .join(', ');

      return [
        ...prev,
        {
          id: crypto.randomUUID(),
          name: guardrail.name,
          version: guardrail.version || '1.0.0',
          configuration: configurationSummary,
          settings,
        },
      ];
    });
  };

  const handleRemoveGuardrail = (guardrailId: string) => {
    setGuardrails((prev) => {
      const removed = prev.find((item) => item.id === guardrailId);
      const next = prev.filter((item) => item.id !== guardrailId);
      if (
        removed &&
        selectedGuardrail === removed.name &&
        !next.some((item) => item.name === removed.name)
      ) {
        setSelectedGuardrail(null);
        setGuardrailSettings({});
      }
      return next;
    });
  };

  const handleReorderGuardrail = (sourceId: string, targetId: string) => {
    setGuardrails((prev) => {
      const sourceIndex = prev.findIndex((item) => item.id === sourceId);
      const targetIndex = prev.findIndex((item) => item.id === targetId);
      if (sourceIndex === -1 || targetIndex === -1) return prev;
      const next = [...prev];
      const [moved] = next.splice(sourceIndex, 1);
      next.splice(targetIndex, 0, moved);
      return next;
    });
  };

  const runDeployment = async (providerId: string, gatewayId: string): Promise<boolean> => {
    const organizationId = currentOrganization?.uuid;
    if (!organizationId) {
      return false;
    }

    try {
      setIsDeployingProvider(true);
      const result = await deployProviderToGateway(providerId, organizationId, gatewayId);
      if (!result.success) {
        const reason =
          result.reason || 'Failed to deploy the AI Provider to the selected gateway.';
        setDeploymentError(reason);
        showSnackbar(reason, 'error');
        return false;
      }
      setDeploymentError(null);
      setCurrentStepIndex(2);
      return true;
    } catch (error: unknown) {
      const typedError = error as { message?: string };
      const reason =
        typedError.message || 'Failed to deploy the AI Provider to the selected gateway.';
      setDeploymentError(reason);
      showSnackbar(reason, 'error');
      return false;
    } finally {
      setIsDeployingProvider(false);
    }
  };

  const handleCreateOrUpdateProvider = async () => {
    if (!selectedTemplateId || !selectedTemplate || !isConfigureFormValid) {
      return;
    }

    const gatewayId = createdGateway?.id ?? preferredGatewayId;
    let providerId = createdProviderId;
    const isNewProvider = !providerId;

    try {
      setIsSubmittingProvider(true);
      if (providerId) {
        await updateProvider(
          providerId,
          buildProviderUpdateRequest({
            formState,
            selectedTemplateId,
            template: selectedTemplate,
            openapiSpec,
            guardrails,
          })
        );
      } else {
        const createdProvider = await createProvider(
          buildProviderCreateRequest({
            formState,
            selectedTemplateId,
            template: selectedTemplate,
            openapiSpec,
            guardrails,
          })
        );
        providerId = createdProvider.id;
        setCreatedProviderId(providerId);
      }
      setProviderSavedAt(new Date().toISOString());
      if (isNewProvider) {
        showSnackbar(`'${formState.name.trim()}' successfully created.`, 'success');
      }
    } catch (error: unknown) {
      const typedError = error as {
        response?: {
          data?: {
            description?: string;
            message?: string;
          };
        };
        message?: string;
      };
      const description =
        typedError.response?.data?.description ||
        typedError.response?.data?.message ||
        typedError.message ||
        'Failed to save LLM provider.';
      showSnackbar(description, 'error');
      return;
    } finally {
      setIsSubmittingProvider(false);
    }

    if (!gatewayId) {
      setCurrentStepIndex(2);
      return;
    }

    await runDeployment(providerId, gatewayId);
  };

  const handleRedeploy = async () => {
    const gatewayId = createdGateway?.id ?? preferredGatewayId;
    if (!createdProviderId || !gatewayId) {
      return;
    }
    await runDeployment(createdProviderId, gatewayId);
  };

  const handleFinish = () => {
    if (!createdProviderId) return;
    navigate(
      buildOrgPath(currentOrganization, `/service-provider/${createdProviderId}`),
      {
        state: { providerAdded: true },
      }
    );
  };

  const stepMeta = {
    'select-template': {
      title: 'Create AI Service',
      description:
        'Select from the options below to start creating a new AI service.',
      nextLabel: 'Next',
      nextDisabled: !selectedTemplateId || Boolean(templatesError) || isComingSoonTemplate(selectedTemplateId ?? ''),
      onNext: () => setCurrentStepIndex(1),
      hideBack: true,
    },
    'configure-provider': deploymentError
      ? {
          title: 'Configure LLM Provider',
          description:
            "Edit and save your provider's details, and add guardrails and policies for using it.",
          nextLabel: 'Re-Deploy',
          nextIcon: <RefreshCw size={16} />,
          nextDisabled: !isGatewaySetupReady,
          onNext: () => {
            void handleRedeploy();
          },
          nextLoading: isDeployingProvider,
          backDisabled: isDeployingProvider,
        }
      : {
          title: 'Configure LLM Provider',
          description:
            "Edit and save your provider's details, and add guardrails and policies for using it.",
          nextLabel: 'Next',
          nextDisabled:
            !isConfigureFormValid || !isGatewaySetupReady,
          onNext: () => {
            void handleCreateOrUpdateProvider();
          },
          nextLoading: isSubmittingProvider || isDeployingProvider,
          backDisabled: isSubmittingProvider || isDeployingProvider,
        },
    'test-provider': {
      title: 'Generate API Key & Test',
      description:
        'Generate an API Key and follow the commands to test your LLM Provider in your CLI.',
      nextLabel: 'Finish',
      nextDisabled: !isTestingReady,
      onNext: handleFinish,
    },
  }[currentStep];

  let stepContent: ReactNode = null;

  if (currentStep === 'select-template') {
    stepContent = (
      <SelectProviderTemplateStep
        role={role}
        resourceType={resourceType}
        selectedTemplateId={selectedTemplateId}
        sortedTemplates={sortedTemplates}
        templatesLoading={templatesLoading}
        templatesError={templatesError}
        templatesResponse={templatesResponse}
        onSelectTemplate={handleSelectTemplate}
        onResourceTypeChange={onResourceTypeChange}
        onRetryTemplates={refreshTemplates}
      />
    );
  } else if (currentStep === 'configure-provider') {
    stepContent = isDeployingProvider ? (
      <AILoader label="We are deploying your AI Provider to the gateway..." />
    ) : isSubmittingProvider ? (
      <AILoader
        label={
          createdProviderId
            ? 'We are in the process of updating your AI Provider...'
            : 'We are in the process of creating your AI Provider...'
        }
      />
    ) : deploymentError ? (
      <DeploymentFail
        providerName={formState.name}
        providerVersion={formState.version}
        providerDescription={formState.description}
        providerContext={formState.context}
        templateName={selectedTemplate?.displayName}
        providerSavedAt={providerSavedAt}
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
    ) : selectedTemplateId ? (
      <ConfigureProviderStep
        selectedTemplateId={selectedTemplateId}
        formState={formState}
        setFormState={setFormState}
        showCredential={showCredential}
        setShowCredential={setShowCredential}
        setOpenapiSpec={setOpenapiSpec}
        guardrails={guardrails}
        selectedGuardrail={selectedGuardrail}
        guardrailSettings={guardrailSettings}
        guardrailDrawerOpen={guardrailDrawerOpen}
        onOpenGuardrailDrawer={handleOpenGuardrailDrawer}
        onCloseGuardrailDrawer={() => setGuardrailDrawerOpen(false)}
        onSelectGuardrail={handleSelectGuardrail}
        onAddGuardrail={handleAddGuardrail}
        onRemoveGuardrail={handleRemoveGuardrail}
        onReorderGuardrail={handleReorderGuardrail}
        onResolvedTemplate={setResolvedTemplate}
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
      <Alert severity="warning">Select a provider template to continue.</Alert>
    );
  } else if (currentStep === 'test-provider') {
    stepContent = createdProviderId ? (
      <StepWithProviderContexts providerId={createdProviderId}>
        <TestLLMProviderStep onTestingReadyChange={setIsTestingReady} />
      </StepWithProviderContexts>
    ) : (
      <Alert severity="warning">Create and deploy the provider before testing it.</Alert>
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
        onSkip={handleSkip}
        onBack={
          currentStepIndex > 0
            ? () => {
                setCurrentStepIndex((prev) => Math.max(0, prev - 1));
              }
            : undefined
        }
        onNext={stepMeta.onNext}
        nextLabel={stepMeta.nextLabel}
        nextDisabled={stepMeta.nextDisabled}
        nextLoading={'nextLoading' in stepMeta ? stepMeta.nextLoading : false}
        nextIcon={'nextIcon' in stepMeta ? stepMeta.nextIcon : undefined}
        hideBack={stepMeta.hideBack}
        backDisabled={'backDisabled' in stepMeta ? stepMeta.backDisabled : false}
      >
        {stepContent}
      </WizardStepCard>
    </Box>
  );
}
