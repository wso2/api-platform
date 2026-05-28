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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Collapse,
  Divider,
  Drawer,
  Grid,
  IconButton,
  InputAdornment,
  ListItemButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Plus,
  ArrowUpDown,
  ChevronDown,
  ChevronUp,
  Search,
  ShieldCheck,
  X,
} from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { getGuardrails } from '../../../../apis/policyHubApis';
import { useProxy } from '../../../../contexts/proxy';
import { useGuardrails } from '../../../../contexts/GuardrailsContext';
import { logger } from '../../../../utils/logger';
import NoData from '../../../../assets/images/NoData.svg';
import { GuardrailPill, PolicyCategorySelector } from '../../../../Components/GuardrailPill';
import { ResourceRow } from '../../../../Components/ResourceView';
import PolicyParameterEditor from '../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  PolicyDefinition,
  ParameterValues,
} from '../../PolicyParameterEditor/types';
import { parsePolicyYaml } from '../../PolicyParameterEditor/yamlParser';
import { FormattedMessage } from 'react-intl';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

// ─── Shared helpers ──────────────────────────────────────────────────────────

type ResourceItem = {
  method: string;
  path: string;
  summary?: string;
  tags?: string[];
};

type DrawerContext =
  | { scope: 'global' }
  | { scope: 'resource'; method: string; path: string };

const matchesResource = (
  pathConfig: { path: string; methods: string[] },
  resource: ResourceItem
) => {
  if (!pathConfig || !resource) return false;
  if (pathConfig.path !== resource.path) return false;
  return (pathConfig.methods || [])
    .map((m) => m.toUpperCase())
    .includes(resource.method.toUpperCase());
};

function extractResourcesFromSpecJson(spec: any): ResourceItem[] {
  const paths = spec?.paths;
  if (!paths || typeof paths !== 'object') return [];
  const httpMethods = new Set([
    'get',
    'post',
    'put',
    'delete',
    'patch',
    'head',
    'options',
  ]);
  const extracted: ResourceItem[] = [];
  Object.keys(paths).forEach((path) => {
    const operations = paths[path];
    if (!operations || typeof operations !== 'object') return;
    Object.keys(operations).forEach((methodKey) => {
      if (!httpMethods.has(methodKey.toLowerCase())) return;
      const op = operations[methodKey] || {};
      extracted.push({
        method: methodKey.toUpperCase(),
        path,
        summary: op?.summary || op?.description || undefined,
        tags: Array.isArray(op?.tags) ? op.tags : undefined,
      });
    });
  });
  extracted.sort((a, b) => {
    const p = a.path.localeCompare(b.path);
    return p !== 0 ? p : a.method.localeCompare(b.method);
  });
  return extracted;
}

const parseOpenApiText = (text: string): ResourceItem[] => {
  if (!text.trim()) return [];
  try {
    return extractResourcesFromSpecJson(JSON.parse(text));
  } catch {
    /* fall through */
  }
  try {
    return extractResourcesFromSpecJson(YAML.parse(text));
  } catch (err) {
    logger.error('Failed to parse OpenAPI spec:', err);
    return [];
  }
};

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * Guardrails tab — mirrors `ServiceProviderGuardrailsTab` but operates on
 * `proxy.policies`.
 */
export default function LLMProxyGuardrailsTab() {
  const { proxy, setLocalProxy } = useProxy();
  const {
    guardrails: availableGuardrails = [],
    isLoading: isLoadingGuardrails,
    error: guardrailsError,
    refreshGuardrails,
    getGuardrailDefinition,
  } = useGuardrails();

  const [openKey, setOpenKey] = useState<string | null>(null);
  const [resourceSearchQuery, setResourceSearchQuery] = useState('');
  const [sortByPolicy, setSortByPolicy] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerContext, setDrawerContext] = useState<DrawerContext>({
    scope: 'global',
  });
  const [selectedGuardrail, setSelectedGuardrail] = useState<string | null>(
    null
  );
  const [isDetailView, setIsDetailView] = useState(false);
  const [policyDefinition, setPolicyDefinition] =
    useState<PolicyDefinition | null>(null);
  const [guardrailSettings, setGuardrailSettings] = useState<ParameterValues>(
    {}
  );
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [definitionError, setDefinitionError] = useState<string | null>(null);
  const [guardrailSearchQuery, setGuardrailSearchQuery] = useState('');
  const [editingTarget, setEditingTarget] = useState<{
    policyIndex: number;
    pathIndex: number | null;
  } | null>(null);
  const [selectedCategories, setSelectedCategories] = useState<string[]>(['AI']);
  const [drawerGuardrails, setDrawerGuardrails] = useState<typeof availableGuardrails>([]);
  const [drawerGuardrailsLoading, setDrawerGuardrailsLoading] = useState(false);

  const fetchDrawerGuardrails = useCallback(async (categories: string[]) => {
    if (categories.length === 0) {
      setDrawerGuardrails([]);
      return;
    }
    setDrawerGuardrailsLoading(true);
    try {
      const response = await getGuardrails(categories.join(','));
      setDrawerGuardrails(response.data);
    } catch {
      setDrawerGuardrails([]);
    } finally {
      setDrawerGuardrailsLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchDrawerGuardrails(selectedCategories);
  }, [selectedCategories, fetchDrawerGuardrails]);

  // Reset drawer state on close
  useEffect(() => {
    if (!drawerOpen) {
      setIsDetailView(false);
      setPolicyDefinition(null);
      setGuardrailSettings({});
      setDefinitionError(null);
      setDefinitionLoading(false);
      setEditingTarget(null);
    }
  }, [drawerOpen]);

  const policies = proxy?.policies ?? [];

  // Create display name map from available guardrails
  const displayNameMap = useMemo(() => {
    const map: Record<string, string> = {};
    availableGuardrails.forEach((guardrail) => {
      if (guardrail.displayName) {
        map[guardrail.name] = guardrail.displayName;
      }
    });
    return map;
  }, [availableGuardrails]);

  // Helper to get display name with fallback chain
  const getDisplayName = (policyName: string): string =>
    displayNameMap[policyName] || policyName;

  const globalGuardrails = useMemo(
    () =>
      policies.flatMap((policy, policyIndex) => {
        const version = policy.version
          ? `v${policy.version.split('.')[0]}`
          : 'v0';
        const displayName = getDisplayName(policy.name);
        if (!policy.paths || policy.paths.length === 0) {
          return [
            {
              id: `${policy.name}-${version}-${policyIndex}-default`,
              name: policy.name,
              displayName,
              version,
              policyIndex,
              pathIndex: null as number | null,
            },
          ];
        }

        return policy.paths.flatMap((pathConfig, pathIndex) =>
          pathConfig.path === '/*'
            ? [
                {
                  id: `${policy.name}-${version}-${policyIndex}-${pathIndex}`,
                  name: policy.name,
                  displayName,
                  version,
                  policyIndex,
                  pathIndex,
                },
              ]
            : []
        );
      }),
    [policies, displayNameMap]
  );

  const resources = useMemo(
    () => parseOpenApiText(proxy?.openapi || ''),
    [proxy?.openapi]
  );

  const selectedGuardrailPolicy =
    drawerGuardrails.find((p) => p.name === selectedGuardrail) ??
    availableGuardrails.find((p) => p.name === selectedGuardrail);
  const selectedGuardrailVersion = selectedGuardrailPolicy?.version
    ? `v${selectedGuardrailPolicy.version.split('.')[0]}`
    : 'v0';

  // ── Drawer openers ──────────────────────────────────────────────────────

  const openAddDrawerForGlobal = () => {
    setEditingTarget(null);
    setDrawerContext({ scope: 'global' });
    setDrawerOpen(true);
  };

  const openAddDrawerForResource = (method: string, path: string) => {
    setEditingTarget(null);
    setDrawerContext({ scope: 'resource', method, path });
    setDrawerOpen(true);
  };

  const handleEditGuardrailPill = (
    policyIndex: number,
    pathIndex: number | null,
    scope: DrawerContext
  ) => {
    const policy = policies[policyIndex];
    if (!policy) return;

    const existingParams =
      pathIndex !== null ? policy.paths?.[pathIndex]?.params ?? {} : {};

    setEditingTarget({ policyIndex, pathIndex });
    setDrawerContext(scope);
    setSelectedGuardrail(policy.name);
    setGuardrailSettings(existingParams);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setDefinitionError(null);
    setDrawerOpen(true);

    const guardrailMeta = availableGuardrails.find(
      (g) => g.name === policy.name
    );
    if (!guardrailMeta?.version) {
      setDefinitionError('No version available for this guardrail.');
      return;
    }

    setDefinitionLoading(true);
    getGuardrailDefinition(guardrailMeta.name, guardrailMeta.version)
      .then((response) => {
        const parsedDefinition = parsePolicyYaml(response);
        setPolicyDefinition(parsedDefinition);
      })
      .catch((e) => {
        logger.error('Failed to load guardrail definition:', e);
        setDefinitionError('Failed to load guardrail definition.');
      })
      .finally(() => {
        setDefinitionLoading(false);
      });
  };

  // ── Guardrail selection & definition loading ────────────────────────────

  const handleGuardrailClick = async (guardrail: {
    name: string;
    version?: string;
  }) => {
    setSelectedGuardrail(guardrail.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setGuardrailSettings({});
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
      setPolicyDefinition(parsePolicyYaml(response));
    } catch (e) {
      logger.error('Failed to load guardrail definition:', e);
      setDefinitionError('Failed to load guardrail definition.');
    } finally {
      setDefinitionLoading(false);
    }
  };

  const handleRetryDefinition = () => {
    if (!selectedGuardrailPolicy) return;

    void handleGuardrailClick({
      name: selectedGuardrailPolicy.name,
      version: selectedGuardrailPolicy.version,
    });
  };

  // ── Build path & submit ─────────────────────────────────────────────────

  const buildPolicyPath = (params: ParameterValues) => {
    if (drawerContext.scope === 'resource') {
      return {
        path: drawerContext.path,
        methods: [drawerContext.method],
        params,
      };
    }
    return { path: '/*', methods: ['*'], params };
  };

  const handlePolicySubmit = async (params: ParameterValues) => {
    if (!proxy || !selectedGuardrailPolicy) return;

    const updatedPolicies = (() => {
      // Edit mode: update existing policy path params
      if (editingTarget) {
        const { policyIndex, pathIndex } = editingTarget;
        const updated = [...policies];
        const existing = updated[policyIndex];
        if (!existing) return policies;

        if (pathIndex !== null) {
          const existingPaths = [...(existing.paths ?? [])];
          existingPaths[pathIndex] = {
            ...existingPaths[pathIndex],
            params,
          };
          updated[policyIndex] = { ...existing, paths: existingPaths };
        }
        return updated;
      }

      // Add mode: add new policy path
      const nextPath = buildPolicyPath(params);
      const existingIndex = policies.findIndex(
        (p) =>
          p.name === selectedGuardrailPolicy.name &&
          p.version === selectedGuardrailVersion
      );

      if (existingIndex === -1) {
        return [
          ...policies,
          {
            name: selectedGuardrailPolicy.name,
            version: selectedGuardrailVersion,
            paths: [nextPath],
          },
        ];
      }

      const existing = policies[existingIndex];
      const existingPaths = existing.paths ?? [];

      const updated = [...policies];
      updated[existingIndex] = {
        ...existing,
        paths: [...existingPaths, nextPath],
      };
      return updated;
    })();

    setLocalProxy((prev) =>
      prev
        ? {
            ...prev,
            policies: updatedPolicies,
          }
        : prev
    );
    setGuardrailSettings(params);
    setDrawerOpen(false);
    setIsDetailView(false);
  };

  const handleRemoveAppliedGuardrail = (
    policyIndex: number,
    pathIndex: number | null
  ) => {
    setLocalProxy((prev) => {
      if (!prev) return prev;
      const existingPolicies = prev.policies ?? [];

      const updatedPolicies = existingPolicies.flatMap((policy, index) => {
        if (index !== policyIndex) return [policy];

        if (pathIndex === null) {
          return [];
        }

        const existingPaths = policy.paths ?? [];
        const nextPaths = existingPaths.filter((_, idx) => idx !== pathIndex);
        if (nextPaths.length === 0) {
          return [];
        }

        return [{ ...policy, paths: nextPaths }];
      });

      return {
        ...prev,
        policies: updatedPolicies,
      };
    });
  };

  // ── Render ──────────────────────────────────────────────────────────────

  return (
    <>
      <Grid container spacing={3}>
        {/* Global guardrails */}
        <Grid size={{ xs: 12 }}>
          <Grid container spacing={2} sx={{ alignItems: 'center' }}>
            <Grid size={{ xs: 12, sm: 'grow' }}>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.apply.global.guardrails"
                  defaultMessage={'Global Guardrails & Policies'}
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.applies.for.all.resources"
                  defaultMessage={'Applies for all resources'}
                />
              </Typography>
            </Grid>

            <Grid size={{ xs: 12, sm: 'auto' }}>
              <Button
                size="small"
                variant="outlined"
                startIcon={<Plus size={16} />}
                onClick={openAddDrawerForGlobal}
                sx={{ whiteSpace: 'nowrap' }}
              >
                Add
              </Button>
            </Grid>
          </Grid>

          <Stack
            direction="row"
            useFlexGap
            sx={{
              mt: 1.5,
              flexWrap: 'wrap',
              columnGap: 1,
              rowGap: 1.25,
            }}
          >
            {globalGuardrails.map((g) => (
              <GuardrailPill
                key={g.id}
                label={`${g.displayName} (${g.version.replace(/^v/, '')})`}
                onClick={() =>
                  handleEditGuardrailPill(g.policyIndex, g.pathIndex, {
                    scope: 'global',
                  })
                }
                onRemove={() =>
                  void handleRemoveAppliedGuardrail(g.policyIndex, g.pathIndex)
                }
              />
            ))}
          </Stack>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Divider />
        </Grid>

        {/* Resource-wise guardrails */}
        <Grid size={{ xs: 12, md: 12, lg: 12 }}>
          <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.apply.guardrails.resource.wise"
              defaultMessage="Resource-wise Guardrails & Policies"
            />
          </Typography>

          {resources.length > 0 && (
            <Grid container spacing={1} sx={{ mb: 1.5, alignItems: 'center' }}>
              <Grid size={{ xs: 12, sm: 'grow' }}>
                <TextField
                  size="small"
                  fullWidth
                  placeholder="Search resources"
                  value={resourceSearchQuery}
                  onChange={(e) => setResourceSearchQuery(e.target.value)}
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
              </Grid>
              <Grid size={{ xs: 12, sm: 'auto' }}>
                <Tooltip
                  title={
                    sortByPolicy
                      ? 'Show default order'
                      : 'Show resources with guardrails first'
                  }
                  arrow
                >
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<ArrowUpDown size={16} />}
                    onClick={() => setSortByPolicy((prev) => !prev)}
                    sx={{
                      whiteSpace: 'nowrap',
                      ...(sortByPolicy && {
                        borderColor: 'success.main',
                        color: 'success.main',
                        backgroundColor: 'rgba(46, 125, 50, 0.08)',
                      }),
                    }}
                  >
                    Policy applied
                  </Button>
                </Tooltip>
              </Grid>
            </Grid>
          )}

          {resources.length === 0 ? (
            <Stack alignItems="center" spacing={1} sx={{ py: 4 }}>
              <Box
                component="img"
                src={NoData}
                alt="No resources"
                sx={{ width: 70 }}
              />
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.no.resources.available.add.an.openapi.spec.in.resources.tab"
                  defaultMessage={'No resources available.'}
                />
              </Typography>
            </Stack>
          ) : (
            <Box
              sx={{
                maxHeight: 360,
                overflowY: 'auto',
                pr: 0.5,
                mt: 1,
              }}
            >
              <Stack spacing={1.25}>
                {(() => {
                  const hasResourcePolicy = (resource: ResourceItem) =>
                    policies.some((policy) =>
                      (policy.paths ?? []).some(
                        (pc) =>
                          pc.path !== '/*' && matchesResource(pc, resource)
                      )
                    );

                  const filteredResources = resources
                    .filter((resource) => {
                      if (!resourceSearchQuery.trim()) return true;
                      const query = resourceSearchQuery.toLowerCase();
                      return (
                        resource.path.toLowerCase().includes(query) ||
                        resource.method.toLowerCase().includes(query) ||
                        (resource.summary &&
                          resource.summary.toLowerCase().includes(query))
                      );
                    })
                    .sort((a, b) => {
                      if (sortByPolicy) {
                        const aHas = hasResourcePolicy(a) ? 0 : 1;
                        const bHas = hasResourcePolicy(b) ? 0 : 1;
                        if (aHas !== bHas) return aHas - bHas;
                        const aIsTextGen = a.tags?.includes('text-generation') ? 0 : 1;
                        const bIsTextGen = b.tags?.includes('text-generation') ? 0 : 1;
                        return aIsTextGen - bIsTextGen;
                      }
                      const aIsTextGen = a.tags?.includes('text-generation') ? 0 : 1;
                      const bIsTextGen = b.tags?.includes('text-generation') ? 0 : 1;
                      return aIsTextGen - bIsTextGen;
                    });

                  if (filteredResources.length === 0) {
                    return (
                      <Stack alignItems="center" spacing={1} sx={{ py: 4 }}>
                        <Typography variant="body2" color="text.secondary">
                          <FormattedMessage
                            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.no.matching.resources"
                            defaultMessage="No resources match your search."
                          />
                        </Typography>
                      </Stack>
                    );
                  }
                  return filteredResources.map((resource) => {
                    const method = resource.method.toUpperCase();
                    const key = `${method}-${resource.path}`;
                    const isOpen = openKey === key;

                    const resourceGuardrails = (() => {
                      const items: Array<{
                        id: string;
                        name: string;
                        displayName: string;
                        version: string;
                        policyIndex: number;
                        pathIndex: number;
                      }> = [];
                      policies.forEach((policy, policyIndex) => {
                        (policy.paths ?? []).forEach((pc, pathIndex) => {
                          if (pc.path === '/*') return;
                          if (!matchesResource(pc, resource)) return;
                          const version = policy.version
                            ? `v${policy.version.split('.')[0]}`
                            : 'v0';
                          const displayName = getDisplayName(policy.name);
                          items.push({
                            id: `${policy.name}-${version}-${policyIndex}-${pathIndex}`,
                            name: policy.name,
                            displayName,
                            version,
                            policyIndex,
                            pathIndex,
                          });
                        });
                      });
                      return items;
                    })();

                    return (
                      <Box key={key}>
                        <ResourceRow
                          resource={{ ...resource, method }}
                          onClick={() => setOpenKey(isOpen ? null : key)}
                          enablePolicyMapping
                          policies={resourceGuardrails.map((g) => ({
                            id: g.id,
                            displayName: g.displayName,
                            version: g.version,
                          }))}
                          trailing={
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 0.5,
                              }}
                            >
                              {resourceGuardrails.length > 0 && (
                                <Tooltip title="Guardrails applied" arrow>
                                  <Box
                                    component="span"
                                    sx={{
                                      display: 'inline-flex',
                                      color: 'success.main',
                                    }}
                                  >
                                    <ShieldCheck size={18} />
                                  </Box>
                                </Tooltip>
                              )}
                              <IconButton
                                size="small"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setOpenKey(isOpen ? null : key);
                                }}
                              >
                                {isOpen ? (
                                  <ChevronUp size={18} />
                                ) : (
                                  <ChevronDown size={18} />
                                )}
                              </IconButton>
                            </Box>
                          }
                        />

                        <Collapse in={isOpen} timeout="auto" unmountOnExit>
                          <Card sx={{ mt: 1 }}>
                            <CardContent>
                              <Grid
                                container
                                spacing={1}
                                sx={{ alignItems: 'center' }}
                              >
                                <Grid size={{ xs: 12, sm: 'grow' }}>
                                  <Typography variant="subtitle2">
                                    <FormattedMessage
                                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.guardrails"
                                      defaultMessage={'Guardrails & Policies'}
                                    />
                                  </Typography>
                                </Grid>
                                <Grid size={{ xs: 12, sm: 'auto' }}>
                                  <Button
                                    size="small"
                                    variant="outlined"
                                    startIcon={<Plus size={16} />}
                                    onClick={() =>
                                      openAddDrawerForResource(
                                        method,
                                        resource.path
                                      )
                                    }
                                  >
                                    Add Guardrail
                                  </Button>
                                </Grid>
                                <Grid size={{ xs: 12 }}>
                                  <Stack
                                    direction="row"
                                    spacing={1}
                                    useFlexGap
                                    sx={{ flexWrap: 'wrap' }}
                                  >
                                    {resourceGuardrails.length > 0 ? (
                                      resourceGuardrails.map((g) => (
                                        <GuardrailPill
                                          key={g.id}
                                          label={`${
                                            g.displayName
                                          } (${g.version.replace(/^v/, '')})`}
                                          onClick={() =>
                                            handleEditGuardrailPill(
                                              g.policyIndex,
                                              g.pathIndex,
                                              {
                                                scope: 'resource',
                                                method,
                                                path: resource.path,
                                              }
                                            )
                                          }
                                          onRemove={() =>
                                            void handleRemoveAppliedGuardrail(
                                              g.policyIndex,
                                              g.pathIndex
                                            )
                                          }
                                        />
                                      ))
                                    ) : (
                                      <Typography
                                        variant="body2"
                                        color="text.secondary"
                                      >
                                        <FormattedMessage
                                          id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.no.guardrails.added.yet"
                                          defaultMessage={
                                            'No added yet.'
                                          }
                                        />
                                      </Typography>
                                    )}
                                  </Stack>
                                </Grid>
                              </Grid>
                            </CardContent>
                          </Card>
                        </Collapse>
                      </Box>
                    );
                  });
                })()}
              </Stack>
            </Box>
          )}
        </Grid>
      </Grid>

      {/* Guardrail policy selector drawer */}
      <Drawer
        anchor="right"
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <Box
          sx={{
            width: { xs: '100vw', sm: 450, md: 600 },
            maxWidth: '100vw',
            p: 2,
          }}
        >
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
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.guardrail.policies"
                  defaultMessage="Guardrails"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.choose.a.guardrail.to.configure.advanced.options"
                  defaultMessage={
                    'Choose a guardrail to configure advanced options.'
                  }
                />
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close guardrail drawer"
              onClick={() => setDrawerOpen(false)}
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
                      id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.loading.guardrails"
                      defaultMessage={'Loading guardrails…'}
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
                      <Box sx={{ mt: 1 }}>
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
                        {drawerGuardrailsLoading ? (
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2 }}>
                            <CircularProgress size={16} />
                            <Typography variant="body2" color="text.secondary">Loading...</Typography>
                          </Box>
                        ) : drawerGuardrails
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
                                    ? '0 6px 18px rgba(0,0,0,0.12)'
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
                                        color="default"
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
                      <Box>
                        <Typography variant="subtitle2">
                          {selectedGuardrailPolicy?.displayName ||
                            selectedGuardrailPolicy?.name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {selectedGuardrailPolicy?.version}
                        </Typography>
                      </Box>
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
                                  id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.loading.definition"
                                  defaultMessage={'Loading definition…'}
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
                              existingValues={editingTarget ? guardrailSettings : undefined}
                              onCancel={() => setIsDetailView(false)}
                              onSubmit={handlePolicySubmit}
                            />
                          ) : (
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyGuardrailsTab.no.definition.available"
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
