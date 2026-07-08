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

import React, { useState, useEffect, useRef } from 'react';
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  useLLMProviders,
  useProviderTemplates,
  ProviderTemplateProvider,
  useProviderTemplate,
} from '../../../../contexts/llmProvider';
import { useGuardrails } from '../../../../contexts/GuardrailsContext';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useAppShell } from '../../../../contexts/AppShellContext';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { logger } from '../../../../utils/logger';
import { useAIWorkspaceSnackbar } from '../../../../hooks/aiWorkspaceSnackbar';
import ProviderTemplateSelector from './AddNewProvider/ProviderTemplateSelector';
import ProviderTemplateFormFields from './AddNewProvider/ProviderTemplateFormFields';
import GuardrailsSection from './AddNewProvider/GuardrailsSection';
import type {
  FormState,
  GuardrailSelection,
} from './AddNewProvider/serviceProviderTypes';
import type { ProviderTemplate } from '../../../../utils/types';
import { familyHandle } from '../../../../utils/providerTemplateDisplay';
import TemplateVersionDialog from './AddNewProvider/TemplateVersionDialog';
import { FormattedMessage } from 'react-intl';

const VERSION_PATTERN = /^v\d+\.\d+$/;

type TemplateBasedFormFieldsContainerProps = {
  formState: FormState;
  setFormState: React.Dispatch<React.SetStateAction<FormState>>;
  showCredential: boolean;
  setShowCredential: React.Dispatch<React.SetStateAction<boolean>>;
  setOpenapiSpec: React.Dispatch<React.SetStateAction<string>>;
};

function TemplateBasedFormFieldsContainer({
  formState,
  setFormState,
  showCredential,
  setShowCredential,
  setOpenapiSpec,
}: TemplateBasedFormFieldsContainerProps) {
  const { template, isLoading, error } = useProviderTemplate();
  const lastTemplateIdRef = useRef<string | null>(null);

  // Apply template metadata values to form when template loads
  useEffect(() => {
    const currentTemplateId = template?.id ?? null;
    const templateChanged = lastTemplateIdRef.current !== currentTemplateId;
    lastTemplateIdRef.current = currentTemplateId;

    setFormState((prev) => ({
      ...prev,
      upstreamUrl: templateChanged
        ? template?.metadata?.endpointUrl || ''
        : prev.upstreamUrl,
      upstreamAuthType: templateChanged
        ? template?.metadata?.auth?.type || 'api-key'
        : prev.upstreamAuthType,
      upstreamAuthHeader: templateChanged
        ? template?.metadata?.auth?.header || 'Authorization'
        : prev.upstreamAuthHeader,
      upstreamAuthValue: templateChanged ? '' : prev.upstreamAuthValue,
      valuePrefix: template?.metadata?.auth?.valuePrefix || '',
      // Consumer-facing security config (how callers authenticate to our
      // gateway) mirrors the template's upstream auth metadata, when present.
      securityKeyName: templateChanged
        ? template?.metadata?.auth?.header || 'X-API-Key'
        : prev.securityKeyName,
      securityKeyPrefix: templateChanged
        ? template?.metadata?.auth?.valuePrefix || ''
        : prev.securityKeyPrefix,
    }));

    // Resolve OpenAPI spec. Prefer the inline spec stored on the template
    // (e.g. templates created by uploading an OpenAPI spec); otherwise fall
    // back to fetching it from the template's openapiSpecUrl.
    if (templateChanged) {
      const inlineSpec = template?.openapi;
      const specUrl = template?.metadata?.openapiSpecUrl;
      if (inlineSpec) {
        setOpenapiSpec(inlineSpec);
      } else if (specUrl) {
        fetch(specUrl)
          .then((res) => res.text())
          .then((text) => {
            setOpenapiSpec(text);
          })
          .catch((err) => {
            logger.error('Failed to fetch OpenAPI spec:', err);
            setOpenapiSpec('');
          });
      } else {
        setOpenapiSpec('');
      }
    }
  }, [template, setFormState, setOpenapiSpec]);

  return (
    <ProviderTemplateFormFields
      formState={formState}
      setFormState={setFormState}
      showCredential={showCredential}
      setShowCredential={setShowCredential}
      template={template}
      isLoading={isLoading}
      error={error}
    />
  );
}

export default function ServiceProviderNew() {
  const navigate = useNavigate();
  const { hasPermission } = useAppAuth();
  const { createProvider } = useLLMProviders();
  const showSnackbar = useAIWorkspaceSnackbar();
  const {
    templatesResponse,
    isLoading: templatesLoading,
    error: templatesError,
    refreshTemplates,
  } = useProviderTemplates();
  const { currentProject, currentOrganization, setCurrentProject } =
    useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const providersPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/service-provider')
    : buildOrgPath(currentOrganization, '/service-provider');

  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(
    null
  );

  const [selectedTemplateVersion, setSelectedTemplateVersion] = useState<
    string | null
  >(null);

  const [selectedVersionTemplateId, setSelectedVersionTemplateId] = useState<
    string | null
  >(null);
  // Template just clicked, pending version selection in the dialog.
  const [pendingTemplate, setPendingTemplate] = useState<ProviderTemplate | null>(
    null
  );
  const [versionDialogOpen, setVersionDialogOpen] = useState(false);
  const [openapiSpec, setOpenapiSpec] = useState<string>('');

  const [formState, setFormState] = useState<FormState>({
    name: '',
    description: '',
    version: 'v1.0',
    context: '/',
    providerType: '',
    upstreamUrl: '',
    upstreamAuthType: 'api-key',
    upstreamAuthHeader: 'Authorization',
    upstreamAuthValue: '',
    valuePrefix: '',
    securityKeyName: 'X-API-Key',
    securityKeyPrefix: '',
  });
  const isVersionValid = VERSION_PATTERN.test(formState.version.trim());
  const [showCredential, setShowCredential] = useState(false);
  const [guardrails, setGuardrails] = useState<GuardrailSelection[]>([]);
  const [guardrailDrawerOpen, setGuardrailDrawerOpen] = useState(false);
  const [selectedGuardrail, setSelectedGuardrail] = useState<string | null>(
    null
  );
  const [guardrailSettings, setGuardrailSettings] = useState<
    Record<string, unknown>
  >({});

  const { guardrails: availableGuardrails = [] } = useGuardrails();

  useEffect(() => {
    if (isProjectLevel && hasPermission(SCOPES.LLM_PROVIDER_MANAGE)) {
      setCurrentProject(null);
      navigate(buildOrgPath(currentOrganization, '/service-provider/new'), {
        replace: true,
      });
    }
  }, [isProjectLevel, hasPermission, currentOrganization, navigate, setCurrentProject]);

  if (!hasPermission(SCOPES.LLM_PROVIDER_CREATE)) {
    return (
      <PageContent fullWidth>
        <Stack spacing={1}>
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.service.provider.creation.is.unavailable"
              defaultMessage={'Service provider creation is unavailable.'}
            />
          </Typography>
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.switch.to.the.admin.role.to.add.new.service.providers"
              defaultMessage={
                'Switch to the admin role to add new service providers.'
              }
            />
          </Typography>
          {/* <Button component={RouterLink} to={providersPath}>
            Back to list
          </Button> */}
        </Stack>
      </PageContent>
    );
  }

  const selectedGuardrailPolicy = availableGuardrails.find(
    (policy) => policy.name === selectedGuardrail
  );

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

  const handleAddGuardrail = (settings: Record<string, unknown>) => {
    if (!selectedGuardrail || !selectedGuardrailPolicy) return;
    setGuardrailSettings(settings);
    setGuardrails((prev) => {
      const existingIndex = prev.findIndex(
        (item) => item.name === selectedGuardrail
      );
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
      if (existingIndex === -1) {
        return [
          ...prev,
          {
            name: selectedGuardrail,
            version: selectedGuardrailPolicy.version || '1.0.0',
            configuration: configurationSummary,
            settings,
          },
        ];
      }
      const next = [...prev];
      next[existingIndex] = {
        name: selectedGuardrail,
        version: selectedGuardrailPolicy.version || '1.0.0',
        configuration: configurationSummary,
        settings,
      };
      return next;
    });
  };

  const handleRemoveGuardrail = (guardrailName: string) => {
    setGuardrails((prev) => prev.filter((item) => item.name !== guardrailName));
    if (selectedGuardrail === guardrailName) {
      setSelectedGuardrail(null);
      setGuardrailSettings({});
    }
  };

  const isFormValid = Boolean(
    formState.name.trim() &&
      formState.version.trim() &&
      isVersionValid &&
      selectedTemplateId &&
      formState.upstreamUrl.trim() &&
      formState.upstreamAuthType
  );

  const toProviderId = (name: string): string =>
    name
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '');

  const handleCreateProvider = async () => {
    if (!isFormValid || !selectedTemplateId) return;

    try {
      const providerId = toProviderId(formState.name);

      const upstream = {
        main: {
          url: formState.upstreamUrl,
          ref: '',
          auth: {
            type: formState.upstreamAuthType,
            header: formState.upstreamAuthHeader,

            value:
              formState.valuePrefix && formState.upstreamAuthValue.trim()
                ? `${formState.valuePrefix.trimEnd()} ${formState.upstreamAuthValue}`
                : formState.upstreamAuthValue,
          },
        },
      };

      const security = {
        enabled: true,
        apiKey: {
          enabled: true,
          key: formState.securityKeyName || 'X-API-Key',
          in: 'header' as const,
          ...(formState.securityKeyPrefix
            ? { keyPrefix: formState.securityKeyPrefix }
            : {}),
        },
      };

      const payload = {
        id: providerId,
        displayName: formState.name.trim(),
        description: formState.description.trim(),
        version: formState.version.trim(),
        context: formState.context.trim() || '/',
        template: selectedVersionTemplateId ?? selectedTemplateId,
        openapi: openapiSpec,
        upstream,
        security,
        globalPolicies: [
          ...(familyHandle(selectedTemplateId) !== 'azure-openai' &&
          familyHandle(selectedTemplateId) !== 'azureai-foundry'
            ? [{ name: 'llm-cost', version: 'v1', params: {} }]
            : []),
          ...guardrails.map((guardrail) => ({
            name: guardrail.name,
            version: guardrail.version,
            params: guardrail.settings ?? {},
          })),
        ],
        accessControl: {
          mode: 'allow_all' as const,
          exceptions: [],
        },
      };

      const createdProvider = await createProvider(payload);

      const createdProviderId = createdProvider.id ?? providerId;
      navigate(
        isProjectLevel
          ? buildProjectPath(
              currentOrganization,
              currentProject,
              `/service-provider/${createdProviderId}`
            )
          : buildOrgPath(
              currentOrganization,
              `/service-provider/${createdProviderId}`
            ),
        {
          state: { providerAdded: true },
        }
      );
    } catch (error) {
      logger.error('Failed to create LLM provider:', error);
      const description =
        (error as any)?.response?.data?.description ||
        (error as any)?.response?.data?.message ||
        (error instanceof Error ? error.message : null) ||
        'Failed to create LLM provider.';
      showSnackbar(description, 'error');
    }
  };

  const applyTemplateSelection = (
    baseTemplate: ProviderTemplate,
    version: string,
    versionTemplateId: string | null,
    groupId: string | null
  ) => {
    setSelectedTemplateId(baseTemplate.id ?? null);
    setSelectedVersionTemplateId(versionTemplateId);
    setSelectedTemplateVersion(version);
    setFormState((prev) => ({ ...prev, providerType: baseTemplate.displayName }));
    setOpenapiSpec('');
    setVersionDialogOpen(false);
    setPendingTemplate(null);
  };

  const handleSelectTemplate = async (template: ProviderTemplate) => {
    const organizationId = currentOrganization?.uuid;
    if (!template.id || !organizationId) return;
    let groupId = template.groupId;
    if (!groupId) {
      try {
        groupId = (
          await providerTemplateApis.getProviderTemplate(
            template.id,
            PLATFORM_API_BASE_URL
          )
        ).groupId;
      } catch {
        // fall back to the handle below
      }
    }
    const resolvedGroupId = groupId ?? template.id;
    try {
      const enabledVersions = (
        await providerTemplateApis.getProviderTemplateVersions(
          resolvedGroupId,
          PLATFORM_API_BASE_URL
        )
      ).filter((v) => v.enabled !== false);
      if (enabledVersions.length === 0) {
        showSnackbar(
          'No enabled versions are available for this template.',
          'error'
        );
        return;
      }
      if (enabledVersions.length === 1) {
        const only = enabledVersions[0];
        applyTemplateSelection(
          template,
          only?.version ?? template.version ?? 'v1.0',
          only?.id ?? null,
          resolvedGroupId
        );
        return;
      }
    } catch {
      // Couldn't load versions — fall back to the picker.
    }
    setPendingTemplate({ ...template, groupId: resolvedGroupId });
    setVersionDialogOpen(true);
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={providersPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.back.to.list"
          defaultMessage={'Back to list'}
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.add.llm.service.provider"
              defaultMessage={'Add LLM Provider'}
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        {/* MAIN FORM */}
        <Grid container spacing={2}>
          <ProviderTemplateSelector
            templatesLoading={templatesLoading}
            templatesError={templatesError}
            templatesResponse={templatesResponse}
            selectedTemplateId={selectedTemplateId}
            onRetryTemplates={refreshTemplates}
            selectedTemplateVersion={selectedTemplateVersion ?? undefined}
            onSelectTemplate={(template) => void handleSelectTemplate(template)}
          />

          {pendingTemplate && (
            <TemplateVersionDialog
              open={versionDialogOpen}
              groupId={pendingTemplate.groupId ?? pendingTemplate.id ?? ''}
              displayName={pendingTemplate.displayName}
              onClose={() => {
                setVersionDialogOpen(false);
                setPendingTemplate(null);
              }}
              onConfirm={(vt) =>
                applyTemplateSelection(
                  pendingTemplate,
                  vt.version ?? '',
                  vt.id ?? null,
                  pendingTemplate.groupId ?? pendingTemplate.id ?? null
                )
              }
            />
          )}

          {selectedTemplateId && (
            <ProviderTemplateProvider
              id={selectedVersionTemplateId ?? selectedTemplateId}
            >
              <TemplateBasedFormFieldsContainer
                formState={formState}
                setFormState={setFormState}
                showCredential={showCredential}
                setShowCredential={setShowCredential}
                setOpenapiSpec={setOpenapiSpec}
              />
            </ProviderTemplateProvider>
          )}

          {selectedTemplateId && (
            <GuardrailsSection
              guardrails={guardrails}
              selectedGuardrail={selectedGuardrail}
              guardrailSettings={guardrailSettings}
              guardrailDrawerOpen={guardrailDrawerOpen}
              selectedTemplateId={selectedTemplateId}
              onOpenDrawer={handleOpenGuardrailDrawer}
              onCloseDrawer={() => setGuardrailDrawerOpen(false)}
              onSelectGuardrail={handleSelectGuardrail}
              onAddGuardrail={handleAddGuardrail}
              onRemoveGuardrail={handleRemoveGuardrail}
            />
          )}
        </Grid>
        {selectedTemplateId && (
          <Box
            sx={{
              mt: 3,
              display: 'flex',
              justifyContent: 'flex-start',
              gap: 1,
            }}
          >
            <Button
              variant="outlined"
              component={RouterLink}
              to={providersPath}
              color="secondary"
              data-cyid="cancel-provider-button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.cancel"
                defaultMessage={'Cancel'}
              />
            </Button>
            <Button
              variant="contained"
              onClick={handleCreateProvider}
              disabled={!isFormValid}
              data-cyid="add-provider-button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderNew.add.provider"
                defaultMessage={'Add Provider'}
              />
            </Button>
          </Box>
        )}
      </Box>
    </PageContent>
  );
}
