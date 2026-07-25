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
  ChevronRight,
  Search,
  ShieldCheck,
  X,
  ChevronUp,
} from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { getGuardrails } from '../../../../apis/policyHubApis';
import { getGatewayCustomPolicies } from '../../../../apis/gatewayPolicyApis';
import type { GatewayCustomPolicy } from '../../../../apis/gatewayPolicyApis';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import { useGuardrails } from '../../../../contexts/GuardrailsContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import { filterOpenApiSpecByAccessControl } from '../../../../utils/openApiAccessControl';
import { GuardrailPill, PolicyCategorySelector } from '../../../../Components/GuardrailPill';
import { ResourceRow } from '../../../../Components/ResourceView';
import PolicyParameterEditor from '../../PolicyParameterEditor/PolicyParameterEditor';
import type {
  ParameterSchema,
  PolicyDefinition,
  ParameterValues,
} from '../../PolicyParameterEditor/types';
import type { AccessControl, PolicyHubPolicy } from '../../../../utils/types';
import { parsePolicyYaml } from '../../PolicyParameterEditor/yamlParser';
import { FormattedMessage } from 'react-intl';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import {
  DisabledActionTooltip,
} from '../../../../utils/readOnlyArtifacts';

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
  const method = resource.method.toUpperCase();
  return (pathConfig.methods || [])
    .map((m) => m.toUpperCase())
    .includes(method);
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
    if (p !== 0) return p;
    return a.method.localeCompare(b.method);
  });

  return extracted;
}

const parseOpenApiText = (
  text: string,
  accessControl?: AccessControl
): ResourceItem[] => {
  if (!text.trim()) return [];
  try {
    const spec = JSON.parse(text);
    const filteredSpec = filterOpenApiSpecByAccessControl(spec, accessControl);
    return extractResourcesFromSpecJson(filteredSpec ?? spec);
  } catch {
    try {
      const spec = YAML.parse(text);
      const filteredSpec = filterOpenApiSpecByAccessControl(
        spec,
        accessControl
      );
      return extractResourcesFromSpecJson(filteredSpec ?? spec);
    } catch (err) {
      logger.error('Failed to parse OpenAPI spec:', err);
      return [];
    }
  }
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

export default function ServiceProviderGuardrailsTab() {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  const isReadOnlyProvider = Boolean(provider?.readOnly);
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
    source: 'global' | 'operation' | 'legacy';
  } | null>(null);
  const [selectedCategories, setSelectedCategories] = useState<string[]>(['AI']);
  const [drawerGuardrails, setDrawerGuardrails] = useState<typeof availableGuardrails>([]);
  const [drawerGuardrailsLoading, setDrawerGuardrailsLoading] = useState(false);
  const [customPolicies, setCustomPolicies] = useState<GatewayCustomPolicy[]>([]);
  const [customPoliciesLoading, setCustomPoliciesLoading] = useState(false);
  const showSnackbar = useAIWorkspaceSnackbar();

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
    void fetchDrawerGuardrails(selectedCategories);
  }, [selectedCategories, fetchDrawerGuardrails]);

  useEffect(() => {
    void fetchCustomPolicies();
  }, [fetchCustomPolicies]);

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

  // Custom policies applied to the provider are indistinguishable from hub
  // policies once saved, so pill-edit needs its own name-keyed lookup.
  const customPolicyByName = useMemo(() => {
    const map = new Map<string, GatewayCustomPolicy>();
    customPolicies.forEach((p) => map.set(p.name, p));
    return map;
  }, [customPolicies]);

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

  const policies = provider?.policies ?? [];

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

  const globalGuardrails = useMemo(() => {
    const items: Array<{
      id: string;
      name: string;
      displayName: string;
      version: string;
      source: 'global' | 'legacy';
      policyIndex: number;
      pathIndex: number | null;
    }> = [];

    // New globalPolicies list
    (provider?.globalPolicies ?? []).forEach((policy, policyIndex) => {
      const version = policy.version
        ? `v${policy.version.replace(/^v/, '').split('.')[0]}`
        : 'v0';
      items.push({
        id: `global-${policy.name}-${version}-${policyIndex}`,
        name: policy.name,
        displayName: getDisplayName(policy.name),
        version,
        source: 'global',
        policyIndex,
        pathIndex: null,
      });
    });

    // Legacy policies with path === '/*'
    policies.forEach((policy, policyIndex) => {
      const version = policy.version
        ? `v${policy.version.replace(/^v/, '').split('.')[0]}`
        : 'v0';
      const displayName = getDisplayName(policy.name);
      if (!policy.paths || policy.paths.length === 0) {
        items.push({
          id: `legacy-${policy.name}-${version}-${policyIndex}-default`,
          name: policy.name,
          displayName,
          version,
          source: 'legacy',
          policyIndex,
          pathIndex: null,
        });
      } else {
        policy.paths.forEach((pathConfig, pathIndex) => {
          if (pathConfig.path === '/*') {
            items.push({
              id: `legacy-${policy.name}-${version}-${policyIndex}-${pathIndex}`,
              name: policy.name,
              displayName,
              version,
              source: 'legacy',
              policyIndex,
              pathIndex,
            });
          }
        });
      }
    });

    return items;
  }, [provider?.globalPolicies, policies, displayNameMap]);

  const resources = useMemo(() => {
    const specResources = parseOpenApiText(
      provider?.openapi || '',
      provider?.accessControl
    );
    // A gateway-pushed provider has no OpenAPI spec, so specResources is empty.
    // Derive a resource row for every concrete (method, path) referenced by an
    // operation/legacy policy so its guardrails are still shown and viewable in
    // the resource-wise section. Wildcard method ('*') / path ('/*') entries are
    // provider-wide (handled elsewhere) and are skipped here.
    const seen = new Set(
      specResources.map((r) => `${r.method.toUpperCase()} ${r.path}`)
    );
    const derived: ResourceItem[] = [];
    const addPaths = (
      paths?: Array<{ methods?: string[]; path: string }>
    ) => {
      (paths ?? []).forEach((pc) => {
        if (!pc?.path || pc.path === '/*') return;
        (pc.methods ?? []).forEach((m) => {
          const method = (m || '').toUpperCase();
          if (!method || method === '*') return;
          const dedupeKey = `${method} ${pc.path}`;
          if (seen.has(dedupeKey)) return;
          seen.add(dedupeKey);
          derived.push({ method, path: pc.path });
        });
      });
    };
    (provider?.operationPolicies ?? []).forEach((p) => addPaths(p.paths));
    policies.forEach((p) => addPaths(p.paths));
    return [...specResources, ...derived];
  }, [
    provider?.openapi,
    provider?.accessControl,
    provider?.operationPolicies,
    policies,
  ]);

  const selectedGuardrailPolicy =
    drawerItems.find((policy) => policy.name === selectedGuardrail) ??
    availableGuardrails.find((policy) => policy.name === selectedGuardrail);
  const selectedGuardrailVersion = selectedGuardrailPolicy?.version
    ? `v${selectedGuardrailPolicy.version.replace(/^v/i, '').split('.')[0]}`
    : 'v0';

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
    scope: DrawerContext,
    source: 'global' | 'operation' | 'legacy'
  ) => {
    let policyName: string;
    let policyVersion: string | undefined;
    let existingParams: ParameterValues = {};

    if (source === 'global') {
      const policy = (provider?.globalPolicies ?? [])[policyIndex];
      if (!policy) return;
      policyName = policy.name;
      policyVersion = policy.version;
      existingParams = (policy.params as ParameterValues) ?? {};
    } else if (source === 'operation') {
      const policy = (provider?.operationPolicies ?? [])[policyIndex];
      if (!policy) return;
      policyName = policy.name;
      policyVersion = policy.version;
      existingParams =
        pathIndex !== null
          ? ((policy.paths[pathIndex]?.params as ParameterValues) ?? {})
          : {};
    } else {
      const policy = policies[policyIndex];
      if (!policy) return;
      policyName = policy.name;
      policyVersion = policy.version;
      existingParams =
        pathIndex !== null
          ? ((policy.paths?.[pathIndex]?.params as ParameterValues) ?? {})
          : {};
    }

    setEditingTarget({ policyIndex, pathIndex, source });
    setDrawerContext(scope);
    setSelectedGuardrail(policyName);
    setGuardrailSettings(existingParams);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setDefinitionError(null);
    setDrawerOpen(true);

    // Custom policies carry their definition inline — no policy-hub lookup.
    const customPolicy = customPolicyByName.get(policyName);
    if (customPolicy) {
      setPolicyDefinition(buildPolicyDefinitionFromCustomPolicy(customPolicy));
      return;
    }

    // Prefer the policy-hub version, but fall back to the applied policy's own
    // version so non-guardrail policies shown as pills (e.g. the llm-cost tracker,
    // which the guardrails hub list doesn't include) can still load their
    // definition. getGuardrailDefinition hits the generic policy-definition
    // endpoint, so it resolves any policy by name+version.
    const guardrailMeta = availableGuardrails.find(
      (g) => g.name === policyName
    );
    const definitionVersion = guardrailMeta?.version ?? policyVersion;
    if (!definitionVersion) {
      setDefinitionError('No version available for this policy.');
      return;
    }

    setDefinitionLoading(true);
    getGuardrailDefinition(policyName, definitionVersion)
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

  const handleGuardrailClick = async (guardrail: DrawerGuardrailItem) => {
    setSelectedGuardrail(guardrail.name);
    setIsDetailView(true);
    setPolicyDefinition(null);
    setGuardrailSettings({});
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
    } catch (e) {
      logger.error('Failed to load guardrail definition:', e);
      setDefinitionError('Failed to load guardrail definition.');
    } finally {
      setDefinitionLoading(false);
    }
  };

  const handleRetryDefinition = () => {
    if (!selectedGuardrailPolicy) return;

    void handleGuardrailClick(selectedGuardrailPolicy);
  };

  const buildPolicyPath = (params: ParameterValues) => {
    if (drawerContext.scope === 'resource') {
      return {
        path: drawerContext.path,
        methods: [drawerContext.method],
        params,
      };
    }
    return {
      path: '/*',
      methods: ['*'],
      params,
    };
  };

  const handlePolicySubmit = async (params: ParameterValues) => {
    if (!provider || !selectedGuardrailPolicy || isLoading || error) return;

    const {
      status,
      createdAt,
      createdBy,
      updatedAt,
      updatedBy,
      lastUpdated,
      ...updatePayload
    } = provider;

    const isEditing = !!editingTarget;

    try {
      if (editingTarget) {
        const { policyIndex, pathIndex, source } = editingTarget;
        if (source === 'global') {
          const globalPolicies = [...(provider.globalPolicies ?? [])];
          globalPolicies[policyIndex] = { ...globalPolicies[policyIndex], params };
          await updateProvider({ ...updatePayload, globalPolicies });
        } else if (source === 'operation') {
          const operationPolicies = [...(provider.operationPolicies ?? [])];
          const existing = operationPolicies[policyIndex];
          const paths = [...existing.paths];
          if (pathIndex !== null) {
            paths[pathIndex] = { ...paths[pathIndex], params };
          }
          operationPolicies[policyIndex] = { ...existing, paths };
          await updateProvider({ ...updatePayload, operationPolicies });
        } else {
          // legacy
          const updated = [...policies];
          const existing = updated[policyIndex];
          if (!existing) return;
          if (pathIndex !== null) {
            const existingPaths = [...(existing.paths ?? [])];
            existingPaths[pathIndex] = { ...existingPaths[pathIndex], params };
            updated[policyIndex] = { ...existing, paths: existingPaths };
          }
          await updateProvider({ ...updatePayload, policies: updated });
        }
      } else if (drawerContext.scope === 'global') {
        // Add mode — global scope → globalPolicies
        const globalPolicies = [...(provider.globalPolicies ?? [])];
        const existingIndex = globalPolicies.findIndex(
          (p) =>
            p.name === selectedGuardrailPolicy.name &&
            p.version === selectedGuardrailVersion
        );
        if (existingIndex === -1) {
          globalPolicies.push({
            name: selectedGuardrailPolicy.name,
            version: selectedGuardrailVersion,
            params,
          });
        } else {
          globalPolicies[existingIndex] = {
            ...globalPolicies[existingIndex],
            params,
          };
        }
        await updateProvider({ ...updatePayload, globalPolicies });
      } else {
        // Add mode — resource scope → operationPolicies
        const operationPolicies = [...(provider.operationPolicies ?? [])];
        const resourcePath = drawerContext.path;
        const resourceMethod = drawerContext.method;
        const newPathEntry = { path: resourcePath, methods: [resourceMethod], params };
        const existingPolicyIndex = operationPolicies.findIndex(
          (p) =>
            p.name === selectedGuardrailPolicy.name &&
            p.version === selectedGuardrailVersion
        );
        if (existingPolicyIndex === -1) {
          operationPolicies.push({
            name: selectedGuardrailPolicy.name,
            version: selectedGuardrailVersion,
            paths: [newPathEntry],
          });
        } else {
          const existing = operationPolicies[existingPolicyIndex];
          const alreadyHasPath = existing.paths.some(
            (p) =>
              p.path === resourcePath &&
              p.methods
                .map((m) => m.toUpperCase())
                .includes(resourceMethod.toUpperCase())
          );
          if (!alreadyHasPath) {
            operationPolicies[existingPolicyIndex] = {
              ...existing,
              paths: [...existing.paths, newPathEntry],
            };
          }
        }
        await updateProvider({ ...updatePayload, operationPolicies });
      }

      setGuardrailSettings(params);
      if (!isDraftMode) {
        showSnackbar(
          isEditing
            ? 'Guardrail updated successfully.'
            : 'Guardrail added successfully.',
          'success'
        );
      }
      setDrawerOpen(false);
      setIsDetailView(false);
    } catch (e) {
      logger.error('Failed to update guardrails:', e);
      if (!isDraftMode) {
        showSnackbar(
          isEditing
            ? 'Failed to update guardrail.'
            : 'Failed to add guardrail.',
          'error'
        );
      }
    }
  };

  const handleRemoveAppliedGuardrail = async (
    policyIndex: number,
    pathIndex: number | null,
    source: 'global' | 'operation' | 'legacy'
  ) => {
    if (!provider || isLoading || error || isReadOnlyProvider) return;

    const {
      status,
      createdAt,
      createdBy,
      updatedAt,
      updatedBy,
      lastUpdated,
      ...updatePayload
    } = provider;

    try {
      if (source === 'global') {
        const globalPolicies = (provider.globalPolicies ?? []).filter(
          (_, idx) => idx !== policyIndex
        );
        await updateProvider({ ...updatePayload, globalPolicies });
      } else if (source === 'operation') {
        const operationPolicies = [...(provider.operationPolicies ?? [])];
        const existing = operationPolicies[policyIndex];
        if (pathIndex !== null) {
          const paths = existing.paths.filter((_, idx) => idx !== pathIndex);
          if (paths.length === 0) {
            operationPolicies.splice(policyIndex, 1);
          } else {
            operationPolicies[policyIndex] = { ...existing, paths };
          }
        } else {
          operationPolicies.splice(policyIndex, 1);
        }
        await updateProvider({ ...updatePayload, operationPolicies });
      } else {
        // legacy
        const updatedPolicies = policies.flatMap((policy, index) => {
          if (index !== policyIndex) return [policy];
          if (pathIndex === null) return [];
          const existingPaths = policy.paths ?? [];
          const nextPaths = existingPaths.filter((_, idx) => idx !== pathIndex);
          if (nextPaths.length === 0) return [];
          return [{ ...policy, paths: nextPaths }];
        });
        await updateProvider({ ...updatePayload, policies: updatedPolicies });
      }

      if (!isDraftMode) {
        showSnackbar('Guardrail removed successfully.', 'success');
      }
    } catch (e) {
      logger.error('Failed to remove guardrail:', e);
      if (!isDraftMode) {
        showSnackbar('Failed to remove guardrail.', 'error');
      }
    }
  };

  return (
    <>
      <Grid container spacing={3}>
        {/* Global guardrails */}
        <Grid size={{ xs: 12 }}>
          <Grid
            container
            spacing={2}
            sx={{ alignItems: 'center' }}
          >
            <Grid size={{ xs: 12, sm: 'grow' }}>
              <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.apply.global.guardrails"
                  defaultMessage="Global Guardrails & Policies"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.applies.for.all.resources"
                  defaultMessage={'Applies for all resources'}
                />
              </Typography>
            </Grid>

            <Grid size={{ xs: 12, sm: 'auto' }}>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<Plus size={16} />}
                  onClick={openAddDrawerForGlobal}
                  disabled={isReadOnlyProvider}
                  sx={{ whiteSpace: 'nowrap' }}
                >
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.add.guardrail"
                    defaultMessage={'Add'}
                  />
                </Button>
              </DisabledActionTooltip>
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
                label={`${g.displayName} (${g.version})`}
                onClick={() =>
                  handleEditGuardrailPill(g.policyIndex, g.pathIndex, {
                    scope: 'global',
                  }, g.source)
                }
                onRemove={
                  isReadOnlyProvider
                    ? undefined
                    : () =>
                      void handleRemoveAppliedGuardrail(
                          g.policyIndex,
                          g.pathIndex, g.source
                      )
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
          <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.apply.guardrails.resource.wise"
              defaultMessage="Resource-wise Guardrails"
            />
          </Typography>

          {resources.length > 0 && (
            <Grid
              container
              spacing={1}
              sx={{ mb: 1.5, alignItems: 'center' }}
            >
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

          <Box
            sx={{
              mt: 1,
              maxHeight: { xs: 360, sm: 460, md: 560 },
              overflowY: 'auto',
              pr: 0.5,
            }}
          >
            <Stack spacing={1.25}>
              {(() => {
                const hasResourcePolicy = (resource: ResourceItem) =>
                  (provider?.operationPolicies ?? []).some((policy) =>
                    policy.paths.some((pc) => matchesResource(pc, resource))
                  ) ||
                  policies.some((policy) =>
                    (policy.paths ?? []).some(
                      (pc) => pc.path !== '/*' && matchesResource(pc, resource)
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

                if (filteredResources.length === 0 && resources.length > 0) {
                  return (
                    <Stack alignItems="center" spacing={1} sx={{ py: 4 }}>
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.no.matching.resources"
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
                      source: 'operation' | 'legacy';
                      policyIndex: number;
                      pathIndex: number;
                    }> = [];
                    // New operationPolicies
                    (provider?.operationPolicies ?? []).forEach(
                      (policy, policyIndex) => {
                        policy.paths.forEach((pathConfig, pathIndex) => {
                          if (!matchesResource(pathConfig, resource)) return;
                          const version = policy.version
                            ? `v${policy.version.replace(/^v/, '').split('.')[0]}`
                            : 'v0';
                          items.push({
                            id: `op-${policy.name}-${version}-${policyIndex}-${pathIndex}`,
                            name: policy.name,
                            displayName: getDisplayName(policy.name),
                            version,
                            source: 'operation',
                            policyIndex,
                            pathIndex,
                          });
                        });
                      }
                    );
                    // Legacy policies
                    policies.forEach((policy, policyIndex) => {
                      (policy.paths ?? []).forEach((pathConfig, pathIndex) => {
                        if (pathConfig.path === '/*') return;
                        if (!matchesResource(pathConfig, resource)) return;
                        const version = policy.version
                          ? `v${policy.version.replace(/^v/, '').split('.')[0]}`
                          : 'v0';
                        items.push({
                          id: `legacy-${policy.name}-${version}-${policyIndex}-${pathIndex}`,
                          name: policy.name,
                          displayName: getDisplayName(policy.name),
                          version,
                          source: 'legacy',
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
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                            {resourceGuardrails.length > 0 && (
                              <Tooltip title="Guardrails applied" arrow>
                                <Box component="span" sx={{ display: 'inline-flex', color: 'success.main' }}>
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
                            <Grid container spacing={1} sx={{ alignItems: 'center' }}>
                              <Grid size={{ xs: 12, sm: 'grow' }}>
                                <Typography variant="subtitle2">
                                  <FormattedMessage
                                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.guardrails"
                                    defaultMessage={'Guardrails & Policies'}
                                  />
                                </Typography>
                              </Grid>

                              <Grid size={{ xs: 12, sm: 'auto' }}>
                                <DisabledActionTooltip
                                  disabled={isReadOnlyProvider}
                                >
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
                                    disabled={isReadOnlyProvider}
                                  >
                                    <FormattedMessage
                                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.add.guardrail"
                                      defaultMessage={'Add'}
                                    />
                                  </Button>
                                </DisabledActionTooltip>
                              </Grid>

                              <Grid size={{ xs: 12 }}>
                                <Stack
                                  direction="row"
                                  spacing={1}
                                  useFlexGap
                                  sx={{ flexWrap: 'wrap' }}
                                >
                                  {resourceGuardrails.length > 0 ? (
                                    resourceGuardrails.map((guardrail) => (
                                      <GuardrailPill
                                        key={guardrail.id}
                                        label={`${
                                          guardrail.displayName
                                        } (v${guardrail.version.replace(
                                          /^v/,
                                          ''
                                        )})`}
                                        onClick={() =>
                                          handleEditGuardrailPill(
                                            guardrail.policyIndex,
                                            guardrail.pathIndex,
                                            {
                                              scope: 'resource',
                                              method,
                                              path: resource.path,
                                            },
                                            guardrail.source
                                          )
                                        }
                                        onRemove={
                                          isReadOnlyProvider
                                            ? undefined
                                            : () =>
                                              void handleRemoveAppliedGuardrail(
                                                guardrail.policyIndex,
                                                guardrail.pathIndex,
                                                guardrail.source
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
                                        id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.no.guardrails.added.yet"
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
        </Grid>
      </Grid>

      <Drawer
        anchor="right"
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      >
        <Box sx={{ width: { xs: '100vw', sm: 450, md: 600 }, maxWidth: '100vw', p: 2 }}>
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
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.guardrail.policies"
                  defaultMessage="Guardrails"
                />
              </Typography>
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.choose.a.guardrail.to.configure.advanced.options"
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
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.loading.guardrails"
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
                    <Stack spacing={1.25} sx={{ mt: 1 }}>
                      {drawerItemsLoading ? (
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, py: 2 }}>
                          <CircularProgress size={16} />
                          <Typography variant="body2" color="text.secondary">Loading...</Typography>
                        </Box>
                      ) : drawerItems
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
                                      {guardrail.displayName || guardrail.name}
                                    </Typography>
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                      {guardrail.provider && (
                                        <Chip
                                          label={guardrail.provider}
                                          size="small"
                                          variant="outlined"
                                          color="default"
                                        />
                                      )}
                                      <Chip
                                        label={guardrail.version || 'v0'}
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
                        })}
                    </Stack>
                    </>
                  ) : (
                    <Stack spacing={1.5} sx={{ mt: 1 }}>
                      <Box>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                          <Typography variant="subtitle2">
                            {selectedGuardrailPolicy?.displayName ||
                              selectedGuardrailPolicy?.name}
                          </Typography>
                          {Boolean(selectedGuardrailPolicy?.isCustomPolicy) && (
                            <Chip label="Custom" size="small" variant="outlined" color="primary" />
                          )}
                        </Box>
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
                                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.loading.definition"
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
                              existingValues={editingTarget ? guardrailSettings : undefined}
                              onCancel={() => setIsDetailView(false)}
                              onSubmit={handlePolicySubmit}
                              readOnly={isReadOnlyProvider}
                            />
                          ) : (
                            <Typography variant="body2" color="text.secondary">
                              <FormattedMessage
                                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.no.definition.available"
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
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderGuardrailsTab.back"
                      defaultMessage={'Back'}
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
