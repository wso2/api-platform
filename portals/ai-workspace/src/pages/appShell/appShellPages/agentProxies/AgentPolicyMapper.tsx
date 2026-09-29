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

import React, { useEffect, useState } from 'react';
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
import DraggableGuardrailPill from '../../../../Components/GuardrailPill/DraggableGuardrailPill';
import {
  POLICY_CATEGORIES,
  PolicyCategorySelector,
} from '../../../../Components/GuardrailPill';
import PoliciesReorderHelp from '../../../../Components/GuardrailPill/PoliciesReorderHelp';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  getGuardrails,
  getPolicies,
  getPolicyDefinition as fetchPolicyDefinitionYaml,
} from '../../../../apis/policyHubApis';
import { getGatewayCustomPolicies } from '../../../../apis/gatewayPolicyApis';
import type { GatewayCustomPolicy } from '../../../../apis/gatewayPolicyApis';
import type { PolicyHubPolicy } from '../../../../utils/types';
import PolicyParameterEditor from '../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  ParameterSchema,
  PolicyDefinition as PolicyDefinitionSchema,
  ParameterValues,
} from '../../PolicyParameterEditor/types';
import { parsePolicyYaml } from '../../PolicyParameterEditor/yamlParser';
import { logger } from '../../../../utils/logger';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  DisabledActionTooltip,
  GATEWAY_MANAGED_ARTIFACT_TOOLTIP,
} from '../../../../utils/readOnlyArtifacts';

export type SelectedPolicy = {
  instanceId: string;
  policyId: string;
  policyName: string;
  displayName: string;
  version: string;
  params?: ParameterValues;
};

type Props = {
  selectedPolicies: SelectedPolicy[];
  onAddPolicy: (policy: Omit<SelectedPolicy, 'instanceId'>) => void;
  onUpdatePolicy: (instanceId: string, params: ParameterValues) => void;
  onRemovePolicy: (instanceId: string) => void;
  onReorderPolicies: (
    draggedInstanceId: string,
    targetInstanceId: string
  ) => void;
  readOnly?: boolean;
  title: string;
  description?: string;
};

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
 * YAML fetch needed) — just reshape it into a PolicyDefinitionSchema. */
const buildPolicyDefinitionFromCustomPolicy = (item: {
  name: string;
  version: string;
  description?: string;
  policyDefinition?: Record<string, unknown>;
}): PolicyDefinitionSchema => {
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

/** Fetches Policy Hub policies for the chosen categories and the synced gateway
 * custom policies in parallel, merging and sorting them alphabetically. Either
 * source failing independently still yields the other's results. Selecting all
 * categories, or none, asks for the unfiltered set. */
const fetchAllPolicies = async (
  categories: string[]
): Promise<DrawerGuardrailItem[]> => {
  const showAll =
    categories.length === 0 || categories.length === POLICY_CATEGORIES.length;
  const [hubResult, customResult] = await Promise.allSettled([
    showAll ? getPolicies() : getGuardrails(categories.join(',')),
    getGatewayCustomPolicies(),
  ]);
  const hubItems: DrawerGuardrailItem[] =
    hubResult.status === 'fulfilled' ? hubResult.value.data ?? [] : [];
  if (customResult.status === 'rejected') {
    logger.error('Failed to load custom policies:', customResult.reason);
  }
  const customItems: DrawerGuardrailItem[] =
    customResult.status === 'fulfilled'
      ? (customResult.value.list ?? []).map(toDrawerItem)
      : [];
  return [...hubItems, ...customItems].sort((a, b) =>
    (a.displayName || a.name).localeCompare(b.displayName || b.name)
  );
};

export default function AgentPolicyMapper({
  selectedPolicies,
  onAddPolicy,
  onUpdatePolicy,
  onRemovePolicy,
  onReorderPolicies,
  readOnly = false,
  title,
  description,
}: Props): React.JSX.Element {
  const intl = useIntl();
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [draggedInstanceId, setDraggedInstanceId] = useState<string | null>(
    null
  );
  const [dragOverInstanceId, setDragOverInstanceId] = useState<string | null>(
    null
  );

  // Drawer state
  const [fetchedPolicies, setFetchedPolicies] = useState<DrawerGuardrailItem[]>([]);
  const [policySearch, setPolicySearch] = useState('');
  const [selectedCategories, setSelectedCategories] = useState<string[]>([
    'AI',
  ]);
  const [isFetchingPolicies, setIsFetchingPolicies] = useState(false);
  const [fetchPoliciesError, setFetchPoliciesError] = useState<string | null>(
    null
  );
  const [selectedDrawerPolicy, setSelectedDrawerPolicy] = useState<
    string | null
  >(null);
  const [isDetailView, setIsDetailView] = useState(false);
  const [policyDefinition, setPolicyDefinition] =
    useState<PolicyDefinitionSchema | null>(null);
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [definitionError, setDefinitionError] = useState<string | null>(null);
  const [policySettings, setPolicySettings] = useState<ParameterValues>({});
  const [editingInstanceId, setEditingInstanceId] = useState<string | null>(
    null
  );

  // Reset drawer state on close
  useEffect(() => {
    if (!isDrawerOpen) {
      setIsDetailView(false);
      setPolicyDefinition(null);
      setPolicySettings({});
      setDefinitionError(null);
      setDefinitionLoading(false);
      setSelectedDrawerPolicy(null);
      setEditingInstanceId(null);
    }
  }, [isDrawerOpen]);

  /** Reloads the drawer list for a new category selection, leaving the drawer
   * open and the current selection intact. */
  const refetchForCategories = async (categories: string[]) => {
    setIsFetchingPolicies(true);
    setFetchPoliciesError(null);
    try {
      setFetchedPolicies(await fetchAllPolicies(categories));
    } catch {
      setFetchPoliciesError('Failed to fetch policies.');
    } finally {
      setIsFetchingPolicies(false);
    }
  };

  const handleOpenDrawer = async () => {
    if (readOnly) return;
    setEditingInstanceId(null);
    setIsDrawerOpen(true);
    setIsFetchingPolicies(true);
    setFetchPoliciesError(null);
    try {
      setFetchedPolicies(await fetchAllPolicies(selectedCategories));
    } catch {
      setFetchPoliciesError('Failed to fetch policies.');
    } finally {
      setIsFetchingPolicies(false);
    }
  };

  const handleEditPolicyItem = async (item: SelectedPolicy) => {
    setEditingInstanceId(item.instanceId);
    setIsDrawerOpen(true);
    setIsFetchingPolicies(true);
    setFetchPoliciesError(null);

    try {
      const policies = await fetchAllPolicies(selectedCategories);
      setFetchedPolicies(policies);

      const matchedPolicy = policies.find((p) => p.name === item.policyName);
      if (!matchedPolicy) {
        setIsFetchingPolicies(false);
        return;
      }

      setSelectedDrawerPolicy(matchedPolicy.name);
      setPolicySettings(item.params ?? {});
      setIsDetailView(true);
      setPolicyDefinition(null);
      setDefinitionError(null);

      if (matchedPolicy.isCustomPolicy) {
        setIsFetchingPolicies(false);
        if (!matchedPolicy.customPolicyDefinition) {
          setDefinitionError('No definition available for this custom policy.');
          return;
        }
        setPolicyDefinition(
          buildPolicyDefinitionFromCustomPolicy({
            name: matchedPolicy.name,
            version: matchedPolicy.version,
            description: matchedPolicy.description,
            policyDefinition: matchedPolicy.customPolicyDefinition,
          })
        );
        return;
      }

      if (!matchedPolicy.version) {
        setDefinitionError('No version available for this policy.');
        setIsFetchingPolicies(false);
        return;
      }

      setDefinitionLoading(true);
      setIsFetchingPolicies(false);
      const defResponse = await fetchPolicyDefinitionYaml(
        matchedPolicy.name,
        matchedPolicy.version
      );
      setPolicyDefinition(parsePolicyYaml(defResponse));
    } catch (e) {
      logger.error('Failed to load policy definition:', e);
      setDefinitionError('Failed to load policy definition.');
    } finally {
      setIsFetchingPolicies(false);
      setDefinitionLoading(false);
    }
  };

  const handlePolicyClick = async (policy: DrawerGuardrailItem) => {
    if (readOnly) return;
    setSelectedDrawerPolicy(policy.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setPolicySettings({});
    setDefinitionError(null);

    if (policy.isCustomPolicy) {
      if (!policy.customPolicyDefinition) {
        setDefinitionError('No definition available for this custom policy.');
        return;
      }
      setPolicyDefinition(
        buildPolicyDefinitionFromCustomPolicy({
          name: policy.name,
          version: policy.version,
          description: policy.description,
          policyDefinition: policy.customPolicyDefinition,
        })
      );
      return;
    }

    if (!policy.version) {
      setDefinitionError('No version available for this policy.');
      return;
    }

    try {
      setDefinitionLoading(true);
      const response = await fetchPolicyDefinitionYaml(
        policy.name,
        policy.version
      );
      setPolicyDefinition(parsePolicyYaml(response));
    } catch (e) {
      logger.error('Failed to load policy definition:', e);
      setDefinitionError('Failed to load policy definition.');
    } finally {
      setDefinitionLoading(false);
    }
  };

  const handleRetryDefinition = () => {
    const policy = fetchedPolicies.find((p) => p.name === selectedDrawerPolicy);
    if (policy) {
      void handlePolicyClick(policy);
    }
  };

  const handlePolicySubmit = async (params: ParameterValues) => {
    if (readOnly) return;
    const policy = fetchedPolicies.find((p) => p.name === selectedDrawerPolicy);
    if (!policy) return;

    if (editingInstanceId) {
      onUpdatePolicy(editingInstanceId, params);
    } else {
      onAddPolicy({
        policyId: policy.name,
        policyName: policy.name,
        displayName: policy.displayName || policy.name,
        version: policy.version
          ? `v${policy.version.replace(/^v/i, '').split('.')[0]}`
          : 'v0',
        params,
      });
    }

    setPolicySettings(params);
    setIsDrawerOpen(false);
    setIsDetailView(false);
  };

  const visiblePolicies = fetchedPolicies.filter((policy) => {
    const query = policySearch.trim().toLowerCase();
    if (!query) return true;
    return (
      policy.displayName?.toLowerCase().includes(query) ||
      policy.name.toLowerCase().includes(query)
    );
  });

  const selectedDrawerPolicyData = fetchedPolicies.find(
    (p) => p.name === selectedDrawerPolicy
  );

  const hasPolicies = selectedPolicies.length > 0;

  const handleDrop = (targetInstanceId: string) => {
    if (readOnly) {
      setDraggedInstanceId(null);
      setDragOverInstanceId(null);
      return;
    }
    if (!draggedInstanceId || draggedInstanceId === targetInstanceId) {
      setDraggedInstanceId(null);
      setDragOverInstanceId(null);
      return;
    }

    onReorderPolicies(draggedInstanceId, targetInstanceId);
    setDraggedInstanceId(null);
    setDragOverInstanceId(null);
  };

  return (
    <>
      <Box>
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={1}
          alignItems={{ xs: 'flex-start', sm: 'center' }}
          justifyContent="space-between"
        >
          <Box>
            <Stack direction="row" alignItems="center" spacing={0.5}>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {title}
              </Typography>
              <PoliciesReorderHelp />
            </Stack>
            {description ? (
              <Typography variant="body2" color="text.secondary">
                {description}
              </Typography>
            ) : null}
          </Box>
          <DisabledActionTooltip
            disabled={readOnly}
            title={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
          >
            <Button
              variant="outlined"
              size="small"
              startIcon={<Plus size={16} />}
              disabled={readOnly}
              onClick={() => void handleOpenDrawer()}
            >
              Add Policy
            </Button>
          </DisabledActionTooltip>
        </Stack>

        {selectedPolicies.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
            No policies added yet.
          </Typography>
        ) : (
          <Stack
            direction="row"
            useFlexGap
            sx={{ mt: 1.5, flexWrap: 'wrap', columnGap: 1, rowGap: 1.25 }}
          >
            {selectedPolicies.map((item) => (
              <DraggableGuardrailPill
                key={item.instanceId}
                id={item.instanceId}
                label={`${item.displayName} (${item.version})`}
                reorderable={!readOnly}
                isDragging={draggedInstanceId === item.instanceId}
                isDragOver={
                  dragOverInstanceId === item.instanceId &&
                  draggedInstanceId !== item.instanceId
                }
                onDragStart={() => setDraggedInstanceId(item.instanceId)}
                onDragOver={() => setDragOverInstanceId(item.instanceId)}
                onDrop={() => handleDrop(item.instanceId)}
                onDragEnd={() => {
                  setDraggedInstanceId(null);
                  setDragOverInstanceId(null);
                }}
                onClick={() => void handleEditPolicyItem(item)}
                onRemove={
                  readOnly ? undefined : () => onRemovePolicy(item.instanceId)
                }
              />
            ))}
          </Stack>
        )}
      </Box>

      {/* Policy selector drawer */}
      <Drawer
        anchor="right"
        open={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        slotProps={{
          paper: {
            sx: {
              width: isDetailView ? '80vw' : 600,
              maxWidth: isDetailView ? 1400 : 600,
              transition: 'width 0.3s ease',
            },
          },
        }}
      >
        <Box sx={{ p: 2 }}>
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
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.title"
                  defaultMessage="Policies"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.description"
                  defaultMessage="Choose a policy to configure advanced options."
                />
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label={intl.formatMessage({
                id: 'aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.close',
                defaultMessage: 'Close policy drawer',
              })}
              onClick={() => setIsDrawerOpen(false)}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          <Stack spacing={3}>
            <Box>
              {isFetchingPolicies ? (
                <Box
                  sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2 }}
                >
                  <CircularProgress size={20} />
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.loading"
                      defaultMessage="Loading policies…"
                    />
                  </Typography>
                </Box>
              ) : fetchPoliciesError ? (
                <Box sx={{ mt: 1 }}>
                  <ErrorAlert
                    error={new Error(fetchPoliciesError)}
                    onRetry={() => void handleOpenDrawer()}
                  />
                </Box>
              ) : (
                <>
                  {!isDetailView ? (
                    <>
                    <Box sx={{ my: 1 }}>
                      <PolicyCategorySelector
                        value={selectedCategories}
                        onChange={(categories) => {
                          setSelectedCategories(categories);
                          void refetchForCategories(categories);
                        }}
                      />
                    </Box>
                    <TextField
                      size="small"
                      fullWidth
                      placeholder="Search policies"
                      value={policySearch}
                      onChange={(event) => setPolicySearch(event.target.value)}
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
                    <Stack spacing={1.25} sx={{ mt: 1 }}>
                      {visiblePolicies.length === 0 ? (
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{ py: 2 }}
                        >
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.noPolicies"
                            defaultMessage="No policies available."
                          />
                        </Typography>
                      ) : (
                        visiblePolicies.map((policy) => {
                          const isSelected =
                            selectedDrawerPolicy === policy.name;
                          return (
                            <Card
                              key={policy.name}
                              sx={{
                                borderColor: isSelected
                                  ? 'primary.main'
                                  : 'divider',
                                boxShadow: isSelected
                                  ? '0 6px 18px rgba(0,0,0,0.12)'
                                  : 'none',
                              }}
                            >
                              <Box sx={{ p: 1 }}>
                                <ListItemButton
                                  selected={isSelected}
                                  onClick={() => void handlePolicyClick(policy)}
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
                                      {policy.displayName || policy.name}
                                    </Typography>
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                      {policy.provider && (
                                        <Chip
                                          label={policy.provider}
                                          size="small"
                                          variant="outlined"
                                          color="default"
                                        />
                                      )}
                                      <Chip
                                        label={policy.version || 'v0'}
                                        size="small"
                                        variant="outlined"
                                        color="default"
                                      />
                                    </Box>
                                  </Box>
                                </ListItemButton>
                              </Box>
                            </Card>
                          );
                        })
                      )}
                    </Stack>
                    </>
                  ) : (
                    <Grid container spacing={2} sx={{ mt: 1 }}>
                      <Grid size={{ xs: 12 }}>
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
                                    id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.loadingDefinition"
                                    defaultMessage="Loading definition…"
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
                                  selectedDrawerPolicyData?.displayName ||
                                  selectedDrawerPolicyData?.name
                                }
                                existingValues={editingInstanceId ? policySettings : undefined}
                                onCancel={() => setIsDetailView(false)}
                                onSubmit={handlePolicySubmit}
                                readOnly={readOnly}
                              />
                            ) : (
                              <Typography
                                variant="body2"
                                color="text.secondary"
                              >
                                <FormattedMessage
                                  id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.noDefinition"
                                  defaultMessage="No definition available."
                                />
                              </Typography>
                            )}
                          </CardContent>
                        </Card>
                      </Grid>
                    </Grid>
                  )}
                </>
              )}
            </Box>

            {isDetailView &&
              !definitionLoading &&
              !definitionError &&
              !policyDefinition && (
                <Stack direction="row" spacing={1} justifyContent="flex-end">
                  <Button variant="text" onClick={() => setIsDetailView(false)}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.agentProxies.agentPolicyMapper.drawer.back"
                      defaultMessage="Back"
                    />
                  </Button>
                </Stack>
              )}
          </Stack>
        </Box>
      </Drawer>
    </>
  );
}
