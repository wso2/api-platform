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
  ListItemButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, X } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';
import NoData from '../../../../assets/images/NoData.svg';
import {
  getGuardrails,
  getPolicyDefinition as fetchPolicyDefinitionYaml,
} from '../../../../apis/policyHubApis';
import type { PolicyHubPolicy } from '../../../../utils/types';
import PolicyParameterEditor from '../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  PolicyDefinition as PolicyDefinitionSchema,
  ParameterValues,
} from '../../PolicyParameterEditor/types';
import { parsePolicyYaml } from '../../PolicyParameterEditor/yamlParser';
import { logger } from '../../../../utils/logger';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import ExternalServersValidationDetails from './ExternalServersValidationDetails';
import type { EndpointValidationResponse } from './externalServersValidationTypes';
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
  validationResult?: EndpointValidationResponse | null;
  readOnly?: boolean;
};

function DragHandle(): JSX.Element {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: 'repeat(2, 3px)',
        gridTemplateRows: 'repeat(3, 3px)',
        gap: '2px',
        cursor: 'grab',
        py: 0.25,
        px: 0.25,
      }}
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <Box
          key={i}
          sx={{
            width: 3,
            height: 3,
            borderRadius: '50%',
            bgcolor: '#9CA3AF',
          }}
        />
      ))}
    </Box>
  );
}

export default function PolicyMapper({
  selectedPolicies,
  onAddPolicy,
  onUpdatePolicy,
  onRemovePolicy,
  onReorderPolicies,
  validationResult,
  readOnly = false,
}: Props): JSX.Element {
  const intl = useIntl();
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [draggedInstanceId, setDraggedInstanceId] = useState<string | null>(
    null
  );
  const [dragOverInstanceId, setDragOverInstanceId] = useState<string | null>(
    null
  );

  // Drawer state
  const [fetchedPolicies, setFetchedPolicies] = useState<PolicyHubPolicy[]>([]);
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

  const handleOpenDrawer = async () => {
    if (readOnly) return;
    setEditingInstanceId(null);
    setIsDrawerOpen(true);
    setIsFetchingPolicies(true);
    setFetchPoliciesError(null);
    try {
      const response = await getGuardrails('MCP');
      setFetchedPolicies(response.data ?? []);
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
      const response = await getGuardrails('MCP');
      const policies = response.data ?? [];
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

  const handlePolicyClick = async (policy: PolicyHubPolicy) => {
    if (readOnly) return;
    setSelectedDrawerPolicy(policy.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setPolicySettings({});
    setDefinitionError(null);

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
        version: policy.version ? `v${policy.version.split('.')[0]}` : 'v0',
        params,
      });
    }

    setPolicySettings(params);
    setIsDrawerOpen(false);
    setIsDetailView(false);
  };

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
      <Grid container spacing={3} sx={{ minHeight: 320 }}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Box
              sx={{
                display: 'flex',
                flexDirection: { xs: 'column', sm: 'row' },
                justifyContent: 'space-between',
                alignItems: { xs: 'flex-start', sm: 'flex-start' },
                gap: 1,
                mb: 2,
              }}
            >
              <Box>
                <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.policies"
                    defaultMessage="Policies"
                  />
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.policiesDescription"
                    defaultMessage="Add policies to this server and drag to change their execution order."
                  />
                </Typography>
              </Box>
              {hasPolicies && (
                <DisabledActionTooltip
                  disabled={readOnly}
                  title={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                >
                  <span>
                    <Button
                      variant="contained"
                      onClick={() => void handleOpenDrawer()}
                      startIcon={<Plus size={18} />}
                      disabled={readOnly}
                      sx={{ whiteSpace: 'nowrap', flexShrink: 0, mt: 2 }}
                    >
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.addPolicies"
                        defaultMessage="Add Policies"
                      />
                    </Button>
                  </span>
                </DisabledActionTooltip>
              )}
            </Box>

            {selectedPolicies.length === 0 ? (
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  minHeight: 200,
                  border: '0.7px solid',
                  borderColor: '#7c7c7c',
                  borderRadius: 2,
                }}
              >
                <Stack spacing={1} alignItems="center">
                  <Box
                    component="img"
                    src={NoData}
                    alt="No policies"
                    sx={{ width: 70 }}
                  />
                  <Typography variant="body2" color="text.secondary">
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.noPolicies"
                      defaultMessage="No policies added yet."
                    />
                  </Typography>
                  <DisabledActionTooltip
                    disabled={readOnly}
                    title={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                  >
                    <span>
                      <Button
                        variant="contained"
                        onClick={() => void handleOpenDrawer()}
                        startIcon={<Plus size={18} />}
                        disabled={readOnly}
                      >
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.addPolicies"
                          defaultMessage="Add Policies"
                        />
                      </Button>
                    </span>
                  </DisabledActionTooltip>
                </Stack>
              </Box>
            ) : (
              <Stack spacing={1}>
                {selectedPolicies.map((item) => {
                  const isDragOver =
                    dragOverInstanceId === item.instanceId &&
                    draggedInstanceId !== item.instanceId;

                  return (
                    <DisabledActionTooltip
                      key={item.instanceId}
                      // The card is always clickable to VIEW the policy's
                      // details (read-only just disables edit/remove/reorder)
                      disabled={false}
                      title={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                    >
                      <Box
                      key={item.instanceId}
                      draggable={!readOnly}
                      onDragStart={() =>
                        !readOnly && setDraggedInstanceId(item.instanceId)
                      }
                      onDragEnd={() => {
                        setDraggedInstanceId(null);
                        setDragOverInstanceId(null);
                      }}
                      onDragOver={(event: React.DragEvent) => {
                        if (readOnly) return;
                        event.preventDefault();
                        setDragOverInstanceId(item.instanceId);
                      }}
                      onDrop={(event: React.DragEvent) => {
                        if (readOnly) return;
                        event.preventDefault();
                        handleDrop(item.instanceId);
                      }}
                      onClick={() => void handleEditPolicyItem(item)}
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1.5,
                        p: 1,
                        py: 1,
                        width: '100%',
                        // maxWidth: 600,
                        mx: 'auto',
                        borderRadius: 1,
                        border: '1.5px solid',
                        borderColor: isDragOver ? '#1D4ED8' : '#E5E7EB',
                        bgcolor: '#fff',
                        boxShadow: isDragOver
                          ? '0 0 0 3px rgba(29, 78, 216, 0.12)'
                          : '0 1px 3px rgba(0, 0, 0, 0.04)',
                        // Clickable to view details even when read-only.
                        cursor: 'pointer',
                        opacity:
                          draggedInstanceId === item.instanceId ? 0.5 : 1,
                        transition: 'all 0.15s ease',
                        '&:hover': {
                          borderColor: '#D1D5DB',
                          boxShadow: '0 2px 6px rgba(0, 0, 0, 0.08)',
                        },
                      }}
                    >
                      <DragHandle />

                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Typography
                          variant="body2"
                          sx={{
                            fontWeight: 600,
                            fontSize: '0.9rem',
                            color: '#1F2937',
                          }}
                        >
                          {item.displayName}
                        </Typography>
                      </Box>

                      <IconButton
                        size="small"
                        aria-label={intl.formatMessage(
                          {
                            id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.removePolicy',
                            defaultMessage: 'Remove {name}',
                          },
                          { name: item.displayName }
                        )}
                        onClick={(event: React.MouseEvent) => {
                          event.stopPropagation();
                          onRemovePolicy(item.instanceId);
                        }}
                        disabled={readOnly}
                        sx={{
                          color: '#9CA3AF',
                          '&:hover': { color: '#EF4444' },
                        }}
                      >
                        <X size={16} />
                      </IconButton>
                      </Box>
                    </DisabledActionTooltip>
                  );
                })}
              </Stack>
            )}
          </Card>
        </Grid>
      </Grid>

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
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.title"
                  defaultMessage="Policies"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.description"
                  defaultMessage="Choose a policy to configure advanced options."
                />
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label={intl.formatMessage({
                id: 'aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.close',
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
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.loading"
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
                    <Stack spacing={1.25} sx={{ mt: 1 }}>
                      {fetchedPolicies.length === 0 ? (
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{ py: 2 }}
                        >
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.noPolicies"
                            defaultMessage="No policies available."
                          />
                        </Typography>
                      ) : (
                        fetchedPolicies.map((policy) => {
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
                                    <Chip
                                      label={policy.version || 'v0'}
                                      size="small"
                                      variant="outlined"
                                      color="default"
                                    />
                                  </Box>
                                </ListItemButton>
                              </Box>
                            </Card>
                          );
                        })
                      )}
                    </Stack>
                  ) : (
                    <Grid container spacing={2} sx={{ mt: 1 }}>
                      {validationResult && (
                        <Grid size={{ xs: 12, md: 6 }}>
                          <Card>
                            <CardContent sx={{ p: 2 }}>
                              <ExternalServersValidationDetails
                                validationResult={validationResult}
                                showHeader={false}
                                // showInputSchema
                                showSchemaInline
                              />
                            </CardContent>
                          </Card>
                        </Grid>
                      )}
                      <Grid size={{ xs: 12, md: validationResult ? 6 : 12 }}>
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
                                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.loadingDefinition"
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
                                  id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.noDefinition"
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
                      id="aiWorkspace.pages.appShell.appShellPages.externalServers.policyMapper.drawer.back"
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
