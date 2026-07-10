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

import React, { useMemo, useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Drawer,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, X } from '@wso2/oxygen-ui-icons-react';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { FormattedMessage } from 'react-intl';
import NoModelsImage from '../../../../assets/images/NoModels.svg';
import { DisabledActionTooltip } from '../../../../utils/readOnlyArtifacts';

type ProviderOption = {
  id: string;
  name: string;
  description: string;
};

function ProviderRow({
  name,
  selected,
  onClick,
  onRemove,
  removeDisabled = false,
  removeAriaLabel = 'Remove model provider',
}: {
  name: string;
  selected: boolean;
  onClick: () => void;
  onRemove?: () => void;
  removeDisabled?: boolean;
  removeAriaLabel?: string;
}) {
  return (
    <Box
      onClick={onClick}
      sx={{
        cursor: 'pointer',
        userSelect: 'none',
        border: '1px solid',
        borderColor: selected ? 'primary.main' : 'divider',
        backgroundColor: selected
          ? 'rgba(197, 216, 234, 0.06)'
          : 'background.paper',
        borderRadius: 1,
        px: 1.25,
        py: 1,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        boxShadow: selected ? '0 3px 4px rgba(0, 0, 0, 0.18)' : 'none',
      }}
    >
      <Typography variant="body2" sx={{ fontWeight: selected ? 700 : 600 }}>
        {name}
      </Typography>
      {onRemove ? (
        <IconButton
          size="small"
          aria-label={removeAriaLabel}
          disabled={removeDisabled}
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
          sx={{ ml: 'auto', p: 0.25 }}
        >
          <X size={14} />
        </IconButton>
      ) : null}
    </Box>
  );
}

type ModelPillProps = {
  label: string;
  onRemove?: () => void;
  removeDisabled?: boolean;
  removeAriaLabel?: string;
};

function ModelPill({
  label,
  onRemove,
  removeDisabled = false,
  removeAriaLabel = 'Remove model',
}: ModelPillProps) {
  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: '#000000',
        borderRadius: 0.5,
        px: 1.25,
        py: 0.75,
        display: 'inline-flex',
        alignItems: 'center',
        backgroundColor: '#fff',
        boxShadow: '0 1px 2px rgba(0,0,0,0.06)',
      }}
    >
      <Typography variant="body2" color="primary.main" sx={{ fontWeight: 500 }}>
        {label}
      </Typography>
      {onRemove ? (
        <IconButton
          size="small"
          color="primary"
          aria-label={removeAriaLabel}
          disabled={removeDisabled}
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
          sx={{ ml: 0.5, p: 0.25 }}
        >
          <X size={14} />
        </IconButton>
      ) : null}
    </Box>
  );
}

export default function ServiceProviderModelsTab() {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  // The model catalog is control-plane-only metadata: it is NOT part of the gateway
  // runtime artifact (the deployment spec carries no model list), so it stays editable
  // even for gateway-created (read-only) providers.
  const isReadOnlyProvider = false;
  const modelCatalog = useMemo<Record<string, string[]>>(
    () => ({
      meta: [
        'us.meta.llama3-3-70b-instruct-v1:0',
        'us.meta.llama4-maverick-17b-instruct-v1:0',
      ],
      openai: ['gpt-4o-mini', 'gpt-4.1-mini', 'o4-mini'],
      anthropic: ['claude-3.5-sonnet', 'claude-3-opus'],
      'google-vertex': ['gemini-1.5-pro', 'gemini-1.5-flash'],
      'aws-bedrock': ['amazon.titan-text-premier', 'anthropic.claude-v2'],
    }),
    []
  );

  const providerOptions = useMemo<ProviderOption[]>(
    () => [
      { id: 'meta', name: 'Meta', description: 'Llama family models' },
      { id: 'openai', name: 'OpenAI', description: 'GPT model lineup' },
      { id: 'anthropic', name: 'Anthropic', description: 'Claude models' },
      {
        id: 'google-vertex',
        name: 'Google Vertex',
        description: 'Gemini models',
      },
      {
        id: 'aws-bedrock',
        name: 'AWS Bedrock',
        description: 'Bedrock catalog',
      },
    ],
    []
  );

  const [providers, setProviders] = useState(provider?.modelProviders ?? []);
  const [selectedProviderId, setSelectedProviderId] = useState<string>('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedOptionId, setSelectedOptionId] = useState<string | null>(null);
  const [newModelName, setNewModelName] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const showSnackbar = useAIWorkspaceSnackbar();

  useEffect(() => {
    const nextProviders = provider?.modelProviders ?? [];
    setProviders(nextProviders);
    setSelectedProviderId(nextProviders[0]?.id || '');
  }, [provider]);

  const selectedProvider = useMemo(
    () => providers.find((p) => p.id === selectedProviderId),
    [providers, selectedProviderId]
  );

  const selectedOption = providerOptions.find(
    (option) => option.id === selectedOptionId
  );

  const updateModelProviders = async (
    nextProviders: typeof providers,
    successMessage: string,
    errorMessage: string,
    nextSelectedProviderId = selectedProviderId
  ) => {
    if (!provider || isLoading || error || isSaving || isReadOnlyProvider) {
      return false;
    }

    const {
      status,
      createdAt,
      createdBy,
      updatedAt,
      lastUpdated,
      ...updatePayload
    } = provider;

    setIsSaving(true);
    try {
      await updateProvider({
        ...updatePayload,
        modelProviders: nextProviders,
      });
      setProviders(nextProviders);
      setSelectedProviderId(nextSelectedProviderId);
      if (!isDraftMode) {
        showSnackbar(successMessage, 'success');
      }
      return true;
    } catch {
      if (!isDraftMode) {
        showSnackbar(errorMessage, 'error');
      }
      return false;
    } finally {
      setIsSaving(false);
    }
  };

  const addProvider = async () => {
    if (!provider || !selectedOption || isLoading || error || isSaving) return;

    const baseProviders = provider.modelProviders ?? [];
    if (baseProviders.length >= 1) {
      showSnackbar(
        'Only one model provider is supported for this service provider.',
        'warning'
      );
      setDrawerOpen(false);
      setSelectedOptionId(null);
      return;
    }

    const exists = baseProviders.some((p) => p.id === selectedOption.id);
    if (exists) {
      setDrawerOpen(false);
      setSelectedOptionId(null);
      return;
    }

    const modelsForProvider = modelCatalog[selectedOption.id] ?? [];
    const nextProviders = [
      ...baseProviders,
      {
        id: selectedOption.id,
        displayName: selectedOption.name,
        models: modelsForProvider.map((modelId) => ({
          id: modelId,
          displayName: modelId,
          description: '',
        })),
      },
    ];

    const isProviderAdded = await updateModelProviders(
      nextProviders,
      'Model provider added successfully.',
      'Failed to add model provider.',
      selectedOption.id
    );
    if (isProviderAdded) {
      setDrawerOpen(false);
      setSelectedOptionId(null);
    }
  };

  const addModel = async () => {
    if (!selectedProvider || isSaving) return;
    const trimmedName = newModelName.trim();
    if (!trimmedName) return;

    const normalizedModelName = trimmedName.toLowerCase();
    const existingModels = selectedProvider.models ?? [];
    const alreadyExists = existingModels.some(
      (model) =>
        model.id.toLowerCase() === normalizedModelName ||
        (model.displayName ?? '').toLowerCase() === normalizedModelName
    );

    if (alreadyExists) {
      showSnackbar(
        'This model already exists for the selected provider.',
        'warning'
      );
      return;
    }

    const nextProviders = providers.map((providerItem) =>
      providerItem.id === selectedProvider.id
        ? {
            ...providerItem,
            models: [
              ...existingModels,
              {
                id: trimmedName,
                displayName: trimmedName,
                description: '',
              },
            ],
          }
        : providerItem
    );

    const isModelAdded = await updateModelProviders(
      nextProviders,
      'Model added successfully.',
      'Failed to add model.'
    );
    if (isModelAdded) {
      setNewModelName('');
    }
  };

  const removeModel = async (modelId: string) => {
    if (!selectedProvider || isSaving) return;

    const nextProviders = providers.map((providerItem) =>
      providerItem.id === selectedProvider.id
        ? {
            ...providerItem,
            models: (providerItem.models ?? []).filter(
              (model) => model.id !== modelId
            ),
          }
        : providerItem
    );

    await updateModelProviders(
      nextProviders,
      'Model removed successfully.',
      'Failed to remove model.'
    );
  };

  const removeProvider = async (providerId: string) => {
    if (isSaving) return;

    const nextProviders = providers.filter(
      (providerItem) => providerItem.id !== providerId
    );
    const nextSelectedProviderId =
      providerId === selectedProviderId
        ? nextProviders[0]?.id ?? ''
        : selectedProviderId;

    const isProviderRemoved = await updateModelProviders(
      nextProviders,
      'Model provider removed successfully.',
      'Failed to remove model provider.',
      nextSelectedProviderId
    );

    if (isProviderRemoved && providerId === selectedProviderId) {
      setNewModelName('');
    }
  };

  const isSingleProviderLimitReached = providers.length >= 1;
  const disableAddProviderButton = isSingleProviderLimitReached || isSaving;

  return (
    <Stack spacing={2}>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '320px 1fr' },
          gap: 2,
        }}
      >
        {/* LEFT */}
        <Card>
          <CardContent>
            <Stack spacing={2}>
              <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.model.provider"
                  defaultMessage={'Model Provider'}
                />
              </Typography>

              {providers.length ? (
                <Stack spacing={2}>
                  <Stack spacing={1}>
                    {providers.map((p) => (
                      <ProviderRow
                        key={p.id}
                        name={p.displayName || p.id}
                        selected={p.id === selectedProviderId}
                        onClick={() => setSelectedProviderId(p.id)}
                        removeDisabled={isSaving || isReadOnlyProvider}
                        onRemove={
                          p.id === 'azureai-foundry' && !isReadOnlyProvider
                            ? () => {
                                void removeProvider(p.id);
                              }
                            : undefined
                        }
                        removeAriaLabel={`Remove model provider ${p.displayName}`}
                      />
                    ))}
                  </Stack>

                  <DisabledActionTooltip disabled={isReadOnlyProvider}>
                    <Tooltip
                      placement="top"
                      title={
                        disableAddProviderButton ? (
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.single.provider.support.tooltip"
                            defaultMessage={
                              'Only one model provider is supported for this service provider.'
                            }
                          />
                        ) : (
                          ''
                        )
                      }
                    >
                      <Box component="span">
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<Plus size={16} />}
                          disabled={disableAddProviderButton || isReadOnlyProvider}
                          onClick={() => setDrawerOpen(true)}
                        >
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.add.model.provider.2"
                            defaultMessage={'Add Model Provider'}
                          />
                        </Button>
                      </Box>
                    </Tooltip>
                  </DisabledActionTooltip>
                </Stack>
              ) : (
                <Box
                  sx={{
                    minHeight: 180,
                    border: '1px dashed',
                    borderColor: 'divider',
                    borderRadius: 1,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    p: 2,
                  }}
                >
                  <Stack spacing={0.5} alignItems="center" textAlign="center">
                    <Box
                      component="img"
                      src={NoModelsImage}
                      alt="No model providers available"
                      sx={{ width: 45, height: 'auto' }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.no.providers.added.yet"
                        defaultMessage={'No providers added yet.'}
                      />
                    </Typography>
                    <DisabledActionTooltip disabled={isReadOnlyProvider}>
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<Plus size={16} />}
                        disabled={isSaving || isReadOnlyProvider}
                        onClick={() => setDrawerOpen(true)}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.add.model.provider.2"
                          defaultMessage={'Add Model Provider'}
                        />
                      </Button>
                    </DisabledActionTooltip>
                  </Stack>
                </Box>
              )}
            </Stack>
          </CardContent>
        </Card>

        {/* RIGHT */}
        <Card>
          <CardContent>
            <Stack spacing={2}>
              <Box>
                <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.models.available"
                    defaultMessage={'Models Available'}
                  />
                  {selectedProvider?.displayName
                    ? ` — ${selectedProvider.displayName}`
                    : ''}
                </Typography>
              </Box>

              {selectedProvider ? (
                <FormControl fullWidth>
                  <FormLabel>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.type.model.id.and.press.enter"
                      defaultMessage={'Type model id and press Enter'}
                    />
                  </FormLabel>
                  <TextField
                    size="small"
                    fullWidth
                    value={newModelName}
                    disabled={isSaving || isReadOnlyProvider}
                    onChange={(event) => setNewModelName(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault();
                        void addModel();
                      }
                    }}
                    placeholder="e.g., gpt-4.1-mini"
                  />
                </FormControl>
              ) : null}

              <Box
                sx={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: 1,
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  p: 1.5,
                  backgroundColor: 'background.paper',
                }}
              >
                {selectedProvider ? (
                  selectedProvider.models?.length ? (
                    selectedProvider.models.map((m) => (
                      <ModelPill
                        key={m.id}
                        label={m.displayName || m.id}
                        removeDisabled={isSaving || isReadOnlyProvider}
                        onRemove={() => {
                          if (!isReadOnlyProvider) {
                            void removeModel(m.id);
                          }
                        }}
                        removeAriaLabel={`Remove model ${m.displayName || m.id}`}
                      />
                    ))
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.no.models.found.for.this.provider.yet"
                        defaultMessage={
                          'No models found for this provider yet.'
                        }
                      />
                    </Typography>
                  )
                ) : (
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.select.a.provider.to.see.models"
                      defaultMessage={'Select a provider to see models.'}
                    />
                  </Typography>
                )}
              </Box>
            </Stack>
          </CardContent>
        </Card>
      </Box>

      <Drawer
        anchor="right"
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <Box sx={{ width: 420, p: 2 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 1,
            }}
          >
            <Stack spacing={0.5}>
              <Typography variant="subtitle1">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.add.model.provider"
                  defaultMessage={'Add Model Provider'}
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.select.a.provider.to.add.its.model.catalog"
                  defaultMessage={'Select a provider to add its model catalog.'}
                />
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close add provider drawer"
              onClick={() => setDrawerOpen(false)}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          <Stack spacing={1.5}>
            {providerOptions.map((option) => {
              const isSelected = selectedOptionId === option.id;
              const isAdded = providers.some(
                (providerItem) => providerItem.id === option.id
              );
              return (
                <Card
                  key={option.id}
                  variant="outlined"
                  onClick={() => setSelectedOptionId(option.id)}
                  sx={{
                    cursor: 'pointer',
                    borderColor: isSelected ? 'primary.main' : 'divider',
                    boxShadow: isSelected
                      ? '0 6px 18px rgba(0, 0, 0, 0.12)'
                      : 'none',
                    opacity: isAdded ? 0.7 : 1,
                  }}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1.5,
                      padding: 1,
                    }}
                  >
                    <Box sx={{ flex: 1 }}>
                      <Typography variant="subtitle2">{option.name}</Typography>
                      <Typography variant="body2" color="text.secondary">
                        {option.description}
                      </Typography>
                    </Box>
                    {isAdded ? (
                      <Chip
                        size="small"
                        label="Added"
                        variant="outlined"
                        color="success"
                      />
                    ) : null}
                  </Box>
                </Card>
              );
            })}
          </Stack>

          <Stack
            direction="row"
            spacing={1}
            justifyContent="flex-end"
            sx={{ mt: 3 }}
          >
            <Button
              variant="outlined"
              color="secondary"
              onClick={() => {
                setSelectedOptionId(null);
                setDrawerOpen(false);
              }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.cancel"
                defaultMessage={'Cancel'}
              />
            </Button>
            <Button
              variant="contained"
              onClick={addProvider}
              disabled={!selectedOption || isSaving || isReadOnlyProvider}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderModelsTab.add"
                defaultMessage={'Add'}
              />
            </Button>
          </Stack>
        </Box>
      </Drawer>
    </Stack>
  );
}
