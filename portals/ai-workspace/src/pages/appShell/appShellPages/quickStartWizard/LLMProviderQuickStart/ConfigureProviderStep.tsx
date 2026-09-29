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

import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import {
  Box,
  CircularProgress,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';
import {
  ProviderTemplateProvider,
  useProviderTemplate,
} from '../../../../../contexts/llmProvider';
import type { ProviderTemplate } from '../../../../../utils/types';
import GuardrailsSection from '../../serviceProvider/AddNewProvider/GuardrailsSection';
import type {
  FormState,
  GuardrailSelection,
} from '../../serviceProvider/AddNewProvider/serviceProviderTypes';
import {
  buildAutoContext,
  CONTEXT_PATTERN,
  VERSION_PATTERN,
} from './utils';
import { getProviderLogoForTemplate } from '../providerTemplateVisuals';
import GatewayDeploySection from './GatewayDeploySection';
import type { GatewayFormState } from './types';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';

type ConfigureProviderStepProps = {
  selectedTemplateId: string;
  formState: FormState;
  setFormState: Dispatch<SetStateAction<FormState>>;
  showCredential: boolean;
  setShowCredential: Dispatch<SetStateAction<boolean>>;
  setOpenapiSpec: Dispatch<SetStateAction<string>>;
  guardrails: GuardrailSelection[];
  selectedGuardrail: string | null;
  guardrailSettings: Record<string, unknown>;
  guardrailDrawerOpen: boolean;
  onOpenGuardrailDrawer: () => void;
  onCloseGuardrailDrawer: () => void;
  onSelectGuardrail: (guardrail: string) => void;
  onAddGuardrail: (
    guardrail: { name: string; version: string },
    values: Record<string, unknown>
  ) => void;
  onRemoveGuardrail: (guardrailId: string) => void;
  onReorderGuardrail: (sourceId: string, targetId: string) => void;
  onResolvedTemplate: (template: ProviderTemplate | null) => void;
  gatewayFormState: GatewayFormState;
  setGatewayFormState: Dispatch<SetStateAction<GatewayFormState>>;
  preferredGatewayId: string | null;
  onPreferredGatewayChange: (gatewayId: string) => void;
  createdGateway: HybridGateway | null;
  onGatewayCreated: (gateway: HybridGateway) => void;
  onGatewayChange: (gateway: HybridGateway) => void;
  gatewayRegistrationToken: string | null;
  onRegistrationTokenChange: (token: string | null) => void;
  onGatewayReadyChange: (isReady: boolean) => void;
};

type ConfigureProviderStepInnerProps = Omit<
  ConfigureProviderStepProps,
  'selectedTemplateId'
>;

function ConfigureProviderStepInner({
  formState,
  setFormState,
  showCredential,
  setShowCredential,
  setOpenapiSpec,
  guardrails,
  selectedGuardrail,
  guardrailSettings,
  guardrailDrawerOpen,
  onOpenGuardrailDrawer,
  onCloseGuardrailDrawer,
  onSelectGuardrail,
  onAddGuardrail,
  onRemoveGuardrail,
  onReorderGuardrail,
  onResolvedTemplate,
  gatewayFormState,
  setGatewayFormState,
  preferredGatewayId,
  onPreferredGatewayChange,
  createdGateway,
  onGatewayCreated,
  onGatewayChange,
  gatewayRegistrationToken,
  onRegistrationTokenChange,
  onGatewayReadyChange,
}: ConfigureProviderStepInnerProps) {
  const { template, isLoading, error } = useProviderTemplate();
  const [versionTouched, setVersionTouched] = useState(false);
  const [contextTouched, setContextTouched] = useState(false);
  const contextEditedRef = useRef(false);
  const lastTemplateIdRef = useRef<string | null>(null);
  const providerLogo = getProviderLogoForTemplate(template?.displayName || '');

  useEffect(() => {
    onResolvedTemplate(template);
  }, [onResolvedTemplate, template]);

  useEffect(() => {
    const currentTemplateId = template?.id ?? null;
    const templateChanged = lastTemplateIdRef.current !== currentTemplateId;
    lastTemplateIdRef.current = currentTemplateId;

    setFormState((prev) => ({
      ...prev,
      providerType: template?.displayName || prev.providerType,
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
      context: contextEditedRef.current
        ? prev.context
        : buildAutoContext(prev.name),
    }));

    if (templateChanged) {
      const specUrl = template?.metadata?.openapiSpecUrl;
      if (specUrl) {
        fetch(specUrl)
          .then((response) => response.text())
          .then((text) => {
            setOpenapiSpec(text);
          })
          .catch(() => {
            setOpenapiSpec('');
          });
      } else {
        setOpenapiSpec('');
      }
    }
  }, [template, setFormState, setOpenapiSpec]);

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 6 }}>
        <CircularProgress size={20} />
        <Typography variant="body2" color="text.secondary">
          Loading template details...
        </Typography>
      </Box>
    );
  }

  if (error) {
    return (
      <Typography variant="body2" color="error.main">
        Failed to load template details: {error.message}
      </Typography>
    );
  }

  const hasTemplateUrl = Boolean(template?.metadata?.endpointUrl);
  const trimmedVersion = formState.version.trim();
  const versionErrorMessage = !versionTouched
    ? ''
    : !trimmedVersion
      ? 'Version is required.'
      : !VERSION_PATTERN.test(trimmedVersion)
        ? 'Version must match v<major>.<minor> (e.g. v1.0).'
        : '';
  const contextErrorMessage =
    !contextTouched || !formState.context
      ? ''
      : !CONTEXT_PATTERN.test(formState.context)
        ? 'Invalid context path (for example: /my-provider).'
        : '';

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, md: 10 }}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 8 }}>
            <FormControl fullWidth>
              <FormLabel required>Name</FormLabel>
              <TextField
                fullWidth
                value={formState.name}
                onChange={(event) => {
                  const nextName = event.target.value;
                  setFormState((prev) => ({
                    ...prev,
                    name: nextName,
                    context: contextEditedRef.current
                      ? prev.context
                      : buildAutoContext(nextName),
                  }));
                }}
                placeholder={`WSO2 ${template?.displayName || ''} Provider`}
                slotProps={{
                  input: providerLogo
                    ? {
                        startAdornment: (
                          <InputAdornment position="start">
                            <Box
                              component="img"
                              src={providerLogo}
                              alt={`${template?.displayName || 'Provider'} logo`}
                              sx={{ width: 20, height: 20, objectFit: 'contain' }}
                            />
                          </InputAdornment>
                        ),
                      }
                    : undefined,
                }}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth>
              <FormLabel required>Version</FormLabel>
              <TextField
                fullWidth
                value={formState.version}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, version: event.target.value }))
                }
                onBlur={() => setVersionTouched(true)}
                error={Boolean(versionErrorMessage)}
                helperText={versionErrorMessage || undefined}
                placeholder="v1.0"
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Context</FormLabel>
              <TextField
                fullWidth
                value={formState.context}
                onChange={(event) => {
                  contextEditedRef.current = true;
                  setFormState((prev) => ({ ...prev, context: event.target.value }));
                }}
                onBlur={() => setContextTouched(true)}
                error={Boolean(contextErrorMessage)}
                helperText={contextErrorMessage || undefined}
                placeholder="/"
              />
            </FormControl>
          </Grid>

          {!hasTemplateUrl && (
            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel required>Upstream URI</FormLabel>
                <TextField
                  fullWidth
                  value={formState.upstreamUrl}
                  onChange={(event) =>
                    setFormState((prev) => ({ ...prev, upstreamUrl: event.target.value }))
                  }
                  placeholder="https://example.com/openai/deployments/model"
                  helperText="The base URL of the upstream LLM provider"
                />
              </FormControl>
            </Grid>
          )}

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>API Key (Optional)</FormLabel>
              <TextField
                fullWidth
                type={showCredential ? 'text' : 'password'}
                value={formState.upstreamAuthValue}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, upstreamAuthValue: event.target.value }))
                }
                placeholder="Enter API key or token (optional)"
                slotProps={{
                  input: {
                    endAdornment: (
                      <InputAdornment position="end">
                        <IconButton
                          size="small"
                          onClick={() => setShowCredential((prev) => !prev)}
                          aria-label={showCredential ? 'Hide credentials' : 'Show credentials'}
                        >
                          {showCredential ? <EyeOff size={16} /> : <Eye size={16} />}
                        </IconButton>
                      </InputAdornment>
                    ),
                  },
                }}
              />
            </FormControl>
          </Grid>

          <GuardrailsSection
            guardrails={guardrails}
            selectedGuardrail={selectedGuardrail}
            guardrailSettings={guardrailSettings}
            guardrailDrawerOpen={guardrailDrawerOpen}
            selectedTemplateId={template?.id}
            onOpenDrawer={onOpenGuardrailDrawer}
            onCloseDrawer={onCloseGuardrailDrawer}
            onSelectGuardrail={onSelectGuardrail}
            onAddGuardrail={onAddGuardrail}
            onRemoveGuardrail={onRemoveGuardrail}
            onReorderGuardrail={onReorderGuardrail}
          />

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Description (Optional)</FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={2}
                value={formState.description}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, description: event.target.value }))
                }
                placeholder={`Primary ${template?.displayName || ''} provider`}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel sx={{ marginBottom: 1 }}>Deploy to:</FormLabel>
              <GatewayDeploySection
                gatewayFormState={gatewayFormState}
                setGatewayFormState={setGatewayFormState}
                preferredGatewayId={preferredGatewayId}
                onPreferredGatewayChange={onPreferredGatewayChange}
                createdGateway={createdGateway}
                onGatewayCreated={onGatewayCreated}
                onGatewayChange={onGatewayChange}
                gatewayRegistrationToken={gatewayRegistrationToken}
                onRegistrationTokenChange={onRegistrationTokenChange}
                onGatewayReadyChange={onGatewayReadyChange}
              />
            </FormControl>
          </Grid>
        </Grid>
      </Grid>
    </Grid>
  );
}

export default function ConfigureProviderStep({
  selectedTemplateId,
  ...rest
}: ConfigureProviderStepProps) {
  return (
    <ProviderTemplateProvider id={selectedTemplateId}>
      <ConfigureProviderStepInner {...rest} />
    </ProviderTemplateProvider>
  );
}
