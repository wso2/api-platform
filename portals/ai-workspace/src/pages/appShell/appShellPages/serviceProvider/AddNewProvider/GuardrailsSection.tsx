/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  Drawer,
  Grid,
  IconButton,
  InputAdornment,
  ListItemButton,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, Search, X } from '@wso2/oxygen-ui-icons-react';
import { useGuardrails } from '../../../../../contexts/GuardrailsContext';
import { getGuardrails } from '../../../../../apis/policyHubApis';
import { getGatewayCustomPolicies } from '../../../../../apis/gatewayPolicyApis';
import type { GatewayCustomPolicy } from '../../../../../apis/gatewayPolicyApis';
import { GuardrailPill, PolicyCategorySelector } from '../../../../../Components/GuardrailPill';
import PolicyParameterEditor from '../../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  ParameterSchema,
  PolicyDefinition,
  ParameterValues,
} from '../../../PolicyParameterEditor/types';
import { parsePolicyYaml } from '../../../PolicyParameterEditor/yamlParser';
import type { GuardrailSelection } from './serviceProviderTypes';
import { FormattedMessage } from 'react-intl';
import ErrorAlert from '../../../../../Components/common/ErrorAlert';
import { familyHandle } from '../../../../../utils/providerTemplateDisplay';
import type { PolicyHubPolicy } from '../../../../../utils/types';
import { logger } from '../../../../../utils/logger';

const GUARDRAILS_PAGE_SIZE = 40;

/** A drawer list entry — either a Policy Hub guardrail or a synced gateway
 * custom policy, rendered and clicked through the same UI. */
type DrawerGuardrailItem = PolicyHubPolicy & {
  isCustomPolicy?: boolean;
  customPolicyUuid?: string;
  customPolicyDefinition?: Record<string, unknown>;
};

/** Custom policy versions come back as full semver with a "v" prefix (e.g.
 * "v1.0.0"), unlike Policy Hub guardrails which are already display-formatted
 * (e.g. "1.0"). Reformat to major.minor so both render the same way. */
const formatPolicyVersion = (version?: string): string => {
  if (!version) return '0';
  const [major = '0', minor = '0'] = version.replace(/^v/i, '').split('.');
  return `${major}.${minor}`;
};

const toDrawerItem = (policy: GatewayCustomPolicy): DrawerGuardrailItem => ({
  name: policy.name,
  version: formatPolicyVersion(policy.version),
  displayName: policy.displayName || policy.name,
  description: policy.description,
  provider: policy.provider,
  isCustomPolicy: true,
  customPolicyUuid: policy.uuid,
  customPolicyDefinition: policy.policyDefinition,
});

/** Custom policies already carry their full definition inline (no policy-hub
 * YAML fetch needed) — just reshape it into a PolicyDefinition. */
const buildPolicyDefinitionFromCustomPolicy = (item: {
  name: string;
  version: string;
  description?: string;
  policyDefinition?: Record<string, unknown>;
}): PolicyDefinition => {
  const def = (item.policyDefinition ?? {}) as {
    description?: string;
    parameters?: ParameterSchema;
    systemParameters?: ParameterSchema;
  };
  return {
    name: item.name,
    version: item.version,
    description: item.description || def.description || '',
    parameters: def.parameters ?? { type: 'object', properties: {} },
    systemParameters: def.systemParameters,
  };
};

type GuardrailsSectionProps = {
  guardrails: GuardrailSelection[];
  selectedGuardrail: string | null;
  guardrailSettings: ParameterValues;
  guardrailDrawerOpen: boolean;
  selectedTemplateId?: string | null;
  onOpenDrawer: () => void;
  onCloseDrawer: () => void;
  onSelectGuardrail: (guardrail: string) => void;
  onAddGuardrail: (
    guardrail: { name: string; version: string },
    values: ParameterValues
  ) => void;
  onRemoveGuardrail: (guardrailName: string) => void;
};

export default function GuardrailsSection({
  guardrails,
  selectedGuardrail,
  guardrailSettings,
  guardrailDrawerOpen,
  selectedTemplateId,
  onOpenDrawer,
  onCloseDrawer,
  onSelectGuardrail,
  onAddGuardrail,
  onRemoveGuardrail,
}: GuardrailsSectionProps) {
  const showCostPolicy =
    familyHandle(selectedTemplateId) !== 'azure-openai' &&
    familyHandle(selectedTemplateId) !== 'azureai-foundry';
  const {
    guardrails: availableGuardrails = [],
    getGuardrailDefinition,
  } = useGuardrails();

  const [isDetailView, setIsDetailView] = useState(false);
  const [policyDefinition, setPolicyDefinition] =
    useState<PolicyDefinition | null>(null);
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [definitionError, setDefinitionError] = useState<string | null>(null);
  const [guardrailSearchQuery, setGuardrailSearchQuery] = useState('');

  // Drawer-local guardrail list — fetched by selected category with its own
  // pagination, independent of the GuardrailsContext (which only loads the
  // default 'Guardrails,AI' category, capped at one page).
  const [selectedCategories, setSelectedCategories] = useState<string[]>(['AI']);
  const [drawerGuardrails, setDrawerGuardrails] = useState<PolicyHubPolicy[]>([]);
  const [drawerGuardrailsLoading, setDrawerGuardrailsLoading] = useState(false);
  const [drawerGuardrailsError, setDrawerGuardrailsError] =
    useState<Error | null>(null);
  const [guardrailsOffset, setGuardrailsOffset] = useState(0);
  const [hasMoreGuardrails, setHasMoreGuardrails] = useState(false);
  const [isLoadingMoreGuardrails, setIsLoadingMoreGuardrails] = useState(false);
  const [customPolicies, setCustomPolicies] = useState<GatewayCustomPolicy[]>([]);
  const [customPoliciesLoading, setCustomPoliciesLoading] = useState(false);

  // Drawer list = Policy Hub guardrails for the selected categories, plus all
  // synced gateway custom policies (always shown, independent of category
  // filtering) — merged and sorted alphabetically by display name.
  const drawerItems: DrawerGuardrailItem[] = useMemo(() => {
    const customItems = customPolicies.map(toDrawerItem);
    return [...drawerGuardrails, ...customItems].sort((a, b) =>
      (a.displayName || a.name).localeCompare(b.displayName || b.name)
    );
  }, [drawerGuardrails, customPolicies]);

  const drawerItemsLoading = drawerGuardrailsLoading || customPoliciesLoading;

  const selectedGuardrailPolicy =
    drawerItems.find((policy) => policy.name === selectedGuardrail) ??
    availableGuardrails.find((policy) => policy.name === selectedGuardrail);

  const fetchDrawerGuardrails = useCallback(
    async (categories: string[], offset: number, append: boolean) => {
      if (categories.length === 0) {
        setDrawerGuardrails([]);
        setHasMoreGuardrails(false);
        return;
      }
      if (append) {
        setIsLoadingMoreGuardrails(true);
      } else {
        setDrawerGuardrailsLoading(true);
      }
      setDrawerGuardrailsError(null);
      try {
        const response = await getGuardrails(
          categories.join(','),
          GUARDRAILS_PAGE_SIZE,
          offset
        );
        setDrawerGuardrails((prev) =>
          append ? [...prev, ...response.data] : response.data
        );
        const total =
          response.pagination?.total ?? response.count ?? response.data.length;
        setHasMoreGuardrails(offset + response.data.length < total);
      } catch (err) {
        if (!append) setDrawerGuardrails([]);
        setDrawerGuardrailsError(
          err instanceof Error ? err : new Error('Failed to load guardrails')
        );
      } finally {
        setDrawerGuardrailsLoading(false);
        setIsLoadingMoreGuardrails(false);
      }
    },
    []
  );

  useEffect(() => {
    if (!guardrailDrawerOpen) return;
    setGuardrailsOffset(0);
    void fetchDrawerGuardrails(selectedCategories, 0, false);
  }, [guardrailDrawerOpen, selectedCategories, fetchDrawerGuardrails]);

  const fetchCustomPolicies = useCallback(async () => {
    setCustomPoliciesLoading(true);
    try {
      const response = await getGatewayCustomPolicies();
      setCustomPolicies(response.list || []);
    } catch (e) {
      logger.error('Failed to load custom policies:', e);
      setCustomPolicies([]);
    } finally {
      setCustomPoliciesLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!guardrailDrawerOpen) return;
    void fetchCustomPolicies();
  }, [guardrailDrawerOpen, fetchCustomPolicies]);

  const handleLoadMoreGuardrails = useCallback(() => {
    if (isLoadingMoreGuardrails || drawerGuardrailsLoading || !hasMoreGuardrails) {
      return;
    }
    const nextOffset = guardrailsOffset + GUARDRAILS_PAGE_SIZE;
    setGuardrailsOffset(nextOffset);
    void fetchDrawerGuardrails(selectedCategories, nextOffset, true);
  }, [
    isLoadingMoreGuardrails,
    drawerGuardrailsLoading,
    hasMoreGuardrails,
    guardrailsOffset,
    selectedCategories,
    fetchDrawerGuardrails,
  ]);

  const handleGuardrailListScroll = (
    event: React.UIEvent<HTMLDivElement>
  ) => {
    const { scrollTop, scrollHeight, clientHeight } = event.currentTarget;
    if (scrollHeight - scrollTop - clientHeight < 80) {
      handleLoadMoreGuardrails();
    }
  };

  useEffect(() => {
    if (!guardrailDrawerOpen) {
      setIsDetailView(false);
      setPolicyDefinition(null);
      setDefinitionError(null);
      setDefinitionLoading(false);
    }
  }, [guardrailDrawerOpen]);

  const handleGuardrailClick = async (guardrail: DrawerGuardrailItem) => {
    onSelectGuardrail(guardrail.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setDefinitionError(null);

    if (guardrail.isCustomPolicy) {
      if (!guardrail.customPolicyDefinition) {
        setDefinitionError('No definition available for this custom policy.');
        return;
      }
      setPolicyDefinition(
        buildPolicyDefinitionFromCustomPolicy({
          name: guardrail.name,
          version: guardrail.version,
          description: guardrail.description,
          policyDefinition: guardrail.customPolicyDefinition,
        })
      );
      return;
    }

    if (!guardrail.version) {
      setDefinitionError('No version available for this guardrail.');
      return;
    }

    try {
      setDefinitionLoading(true);
      const response = await getGuardrailDefinition(
        guardrail.name,
        guardrail.version
      );
      const parsedDefinition = parsePolicyYaml(response);
      setPolicyDefinition(parsedDefinition);
    } catch (error) {
      setDefinitionError('Failed to load guardrail definition.');
    } finally {
      setDefinitionLoading(false);
    }
  };

  const handlePolicySubmit = (values: ParameterValues) => {
    if (!selectedGuardrailPolicy) return;
    onAddGuardrail(
      {
        name: selectedGuardrailPolicy.name,
        version: selectedGuardrailPolicy.version || '1.0.0',
      },
      values
    );
    setIsDetailView(false);
  };

  const handleRetryDefinition = () => {
    if (!selectedGuardrailPolicy) return;

    void handleGuardrailClick(selectedGuardrailPolicy);
  };

  return (
    <>
      <Grid size={{ xs: 12 }}>
        <Card>
          <CardContent>
            <Stack spacing={1.5} display="flex">
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  justifyContent: 'space-between',
                  gap: 2,
                }}
              >
                <Box>
                  <Typography variant="subtitle1">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.guardrails"
                      defaultMessage={'Guardrails & Policies'}
                    />
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.add.safety.policies.to.enforce.consistent.protections"
                      defaultMessage={
                        'Add safety policies to enforce consistent protections.'
                      }
                    />
                  </Typography>
                </Box>
                <Button
                  variant="contained"
                  startIcon={<Plus size={16} />}
                  onClick={onOpenDrawer}
                >
                  Add
                </Button>
              </Box>

              {(showCostPolicy || guardrails.length > 0) ? (
                <Stack direction="row" spacing={1} flexWrap="wrap">
                  {showCostPolicy && (
                    <Box sx={{ mr: 1.5, mb: 1.5 }}>
                      <GuardrailPill label="llm-cost (v1)" />
                    </Box>
                  )}
                  {guardrails.map((guardrail) => (
                    <Box key={guardrail.name} sx={{ mr: 1.5, mb: 1.5 }}>
                      <GuardrailPill
                        label={`${guardrail.name} (${guardrail.version})`}
                        onRemove={() => onRemoveGuardrail(guardrail.name)}
                      />
                      {/* {guardrail.configuration ? (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ display: 'block', mt: 0.5 }}
                        >
                          {guardrail.configuration}
                        </Typography>
                      ) : null} */}
                    </Box>
                  ))}
                </Stack>
              ) : null}
            </Stack>
          </CardContent>
        </Card>
      </Grid>

      <Drawer anchor="right" open={guardrailDrawerOpen} onClose={onCloseDrawer}>
        <Box sx={{ width: 600, p: 2 }}>
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
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.guardrail.policies"
                  defaultMessage="Guardrails"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.choose.a.guardrail.to.configure.advanced.options"
                  defaultMessage={
                    'Choose a guardrail to configure advanced options.'
                  }
                />
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close guardrail drawer"
              onClick={onCloseDrawer}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          <Stack spacing={3}>
            <Box>
              {!isDetailView ? (
                <>
                  <Box sx={{ my: 1 }}>
                    <PolicyCategorySelector
                      value={selectedCategories}
                      onChange={setSelectedCategories}
                    />
                  </Box>

                  <TextField
                    size="small"
                    fullWidth
                    placeholder="Search guardrails"
                    value={guardrailSearchQuery}
                    onChange={(e) => setGuardrailSearchQuery(e.target.value)}
                    sx={{ mt: 1 }}
                    slotProps={{
                      input: {
                        startAdornment: (
                          <InputAdornment position="start">
                            <Search size={16} />
                          </InputAdornment>
                        ),
                      },
                    }}
                  />

                  {drawerItemsLoading ? (
                    <Box
                      sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2 }}
                    >
                      <CircularProgress size={20} />
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.loading.guardrails"
                          defaultMessage={'Loading guardrails...'}
                        />
                      </Typography>
                    </Box>
                  ) : drawerGuardrailsError ? (
                    <Box sx={{ mt: 1 }}>
                      <ErrorAlert
                        error={drawerGuardrailsError}
                        onRetry={() => {
                          void fetchDrawerGuardrails(selectedCategories, 0, false);
                        }}
                      />
                    </Box>
                  ) : (
                    <Box
                      onScroll={handleGuardrailListScroll}
                      sx={{
                        mt: 1,
                        overflowY: 'auto',
                        pr: 0.5,
                      }}
                    >
                      <Stack spacing={1.25}>
                        {drawerItems
                          .filter((g) => {
                            if (!guardrailSearchQuery.trim()) return true;
                            const query = guardrailSearchQuery.toLowerCase();
                            return (
                              g.displayName?.toLowerCase().includes(query) ||
                              g.name.toLowerCase().includes(query)
                            );
                          })
                          .map((guardrail) => {
                            const isSelected =
                              selectedGuardrail === guardrail.name;
                            return (
                              <Card
                                key={guardrail.name}
                                sx={{
                                  borderColor: isSelected
                                    ? 'primary.main'
                                    : 'divider',
                                  boxShadow: isSelected
                                    ? '0 6px 18px rgba(0, 0, 0, 0.12)'
                                    : 'none',
                                }}
                              >
                                <Box sx={{ p: 1 }}>
                                  <ListItemButton
                                    selected={isSelected}
                                    onClick={() => handleGuardrailClick(guardrail)}
                                    sx={{
                                      p: 0.75,
                                      borderRadius: 1,
                                      '&.Mui-selected': {
                                        backgroundColor: 'transparent',
                                      },
                                    }}
                                  >
                                    <Box
                                      sx={{
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'space-between',
                                        width: '100%',
                                      }}
                                    >
                                      <Typography
                                        variant="body2"
                                        fontWeight={500}
                                      >
                                        {guardrail.displayName ||
                                          guardrail.name}
                                      </Typography>
                                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                        {guardrail.provider && (
                                          <Chip
                                            label={guardrail.provider}
                                            size="small"
                                            variant="outlined"
                                          />
                                        )}
                                        <Chip
                                          label={guardrail.version || 'v0'}
                                          size="small"
                                          variant="outlined"
                                        />
                                      </Box>
                                    </Box>
                                  </ListItemButton>
                                </Box>
                              </Card>
                            );
                          })}
                        {isLoadingMoreGuardrails && (
                          <Box
                            sx={{
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              gap: 1,
                              py: 1.5,
                            }}
                          >
                            <CircularProgress size={16} />
                            <Typography variant="body2" color="text.secondary">
                              Loading more...
                            </Typography>
                          </Box>
                        )}
                      </Stack>
                    </Box>
                  )}
                </>
              ) : (
                    <Stack spacing={1.5} sx={{ mt: 1 }}>
                      {/* <Box>
                        <Typography variant="subtitle2">
                          {selectedGuardrailPolicy?.displayName || selectedGuardrailPolicy?.name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {selectedGuardrailPolicy?.version}
                        </Typography>
                      </Box> */}
                      <Card>
                        <CardContent sx={{ p: 2 }}>
                          {definitionLoading ? (
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 2,
                              }}
                            >
                              <CircularProgress size={20} />
                              <Typography
                                variant="body2"
                                color="text.secondary"
                              >
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.loading.definition"
                                  defaultMessage={'Loading definition...'}
                                />
                              </Typography>
                            </Box>
                          ) : definitionError ? (
                            <ErrorAlert
                              error={new Error(definitionError)}
                              onRetry={handleRetryDefinition}
                            />
                          ) : policyDefinition ? (
                            <PolicyParameterEditor
                              policyDefinition={policyDefinition}
                              policyDisplayName={
                                selectedGuardrailPolicy?.displayName ||
                                selectedGuardrailPolicy?.name
                              }
                              existingValues={undefined}
                              onCancel={() => setIsDetailView(false)}
                              onSubmit={handlePolicySubmit}
                            />
                          ) : (
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.AddNewProvider.GuardrailsSection.no.definition.available"
                                defaultMessage={'No definition available.'}
                              />
                            </Typography>
                          )}
                        </CardContent>
                      </Card>
                    </Stack>
              )}
            </Box>

            {isDetailView &&
              !definitionLoading &&
              !definitionError &&
              !policyDefinition && (
                <Stack direction="row" spacing={1} justifyContent="flex-end">
                  <Button variant="text" onClick={() => setIsDetailView(false)}>
                    Back
                  </Button>
                </Stack>
              )}
          </Stack>
        </Box>
      </Drawer>
    </>
  );
}
