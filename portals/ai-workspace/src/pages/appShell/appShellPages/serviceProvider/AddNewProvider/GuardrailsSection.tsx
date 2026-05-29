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
import { useGuardrails } from '../../../../../contexts/GuardrailsContext';
import { GuardrailPill } from '../../../../../Components/GuardrailPill';
import PolicyParameterEditor from '../../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  PolicyDefinition,
  ParameterValues,
} from '../../../PolicyParameterEditor/types';
import { parsePolicyYaml } from '../../../PolicyParameterEditor/yamlParser';
import type { GuardrailSelection } from './serviceProviderTypes';
import { FormattedMessage } from 'react-intl';
import ErrorAlert from '../../../../../Components/common/ErrorAlert';

type GuardrailsSectionProps = {
  guardrails: GuardrailSelection[];
  selectedGuardrail: string | null;
  guardrailSettings: ParameterValues;
  guardrailDrawerOpen: boolean;
  selectedTemplateId?: string | null;
  onOpenDrawer: () => void;
  onCloseDrawer: () => void;
  onSelectGuardrail: (guardrail: string) => void;
  onAddGuardrail: (values: ParameterValues) => void;
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
    selectedTemplateId !== 'azure-openai' &&
    selectedTemplateId !== 'azureai-foundry';
  const {
    guardrails: availableGuardrails = [],
    isLoading: isLoadingGuardrails,
    error: guardrailsError,
    refreshGuardrails,
    getGuardrailDefinition,
  } = useGuardrails();

  const [isDetailView, setIsDetailView] = useState(false);
  const [policyDefinition, setPolicyDefinition] =
    useState<PolicyDefinition | null>(null);
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [definitionError, setDefinitionError] = useState<string | null>(null);
  const [guardrailSearchQuery, setGuardrailSearchQuery] = useState('');

  const selectedGuardrailPolicy = availableGuardrails.find(
    (policy) => policy.name === selectedGuardrail
  );

  useEffect(() => {
    if (!guardrailDrawerOpen) {
      setIsDetailView(false);
      setPolicyDefinition(null);
      setDefinitionError(null);
      setDefinitionLoading(false);
    }
  }, [guardrailDrawerOpen]);

  const handleGuardrailClick = async (guardrail: {
    name: string;
    version?: string;
  }) => {
    onSelectGuardrail(guardrail.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setDefinitionError(null);

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
    onAddGuardrail(values);
    setIsDetailView(false);
  };

  const handleRetryDefinition = () => {
    if (!selectedGuardrailPolicy) return;

    void handleGuardrailClick({
      name: selectedGuardrailPolicy.name,
      version: selectedGuardrailPolicy.version,
    });
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
              {isLoadingGuardrails ? (
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
              ) : guardrailsError ? (
                <Box sx={{ mt: 1 }}>
                  <ErrorAlert
                    error={guardrailsError}
                    onRetry={() => {
                      void refreshGuardrails();
                    }}
                  />
                </Box>
              ) : (
                <>
                  {!isDetailView ? (
                    <>
                      <TextField
                        size="small"
                        fullWidth
                        placeholder="Search guardrails"
                        value={guardrailSearchQuery}
                        onChange={(e) =>
                          setGuardrailSearchQuery(e.target.value)
                        }
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
                        {availableGuardrails
                          .filter((g) => !g.categories?.includes('MCP'))
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
                                    onClick={() =>
                                      handleGuardrailClick({
                                        name: guardrail.name,
                                        version: guardrail.version,
                                      })
                                    }
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
                                      <Chip
                                        label={guardrail.version || 'v0'}
                                        size="small"
                                        variant="outlined"
                                      />
                                    </Box>
                                  </ListItemButton>
                                </Box>
                              </Card>
                            );
                          })}
                      </Stack>
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
                </>
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
