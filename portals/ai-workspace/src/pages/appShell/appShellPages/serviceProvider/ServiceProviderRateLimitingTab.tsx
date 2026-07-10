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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Collapse,
  Divider,
  FormControl,
  FormControlLabel,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Tooltip,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowUpDown,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  Clock2,
  Layers,
  ShieldCheck,
} from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import { filterOpenApiSpecByAccessControl } from '../../../../utils/openApiAccessControl';
import { ResourceRow } from '../../../../Components/ResourceView';
import { FormattedMessage } from 'react-intl';
import type { AccessControl } from '../../../../utils/types';
import { GATEWAY_MANAGED_ARTIFACT_TOOLTIP } from '../../../../utils/readOnlyArtifacts';

type ResourceItem = {
  method: string;
  path: string;
  summary?: string;
  secured?: boolean;
  disabled?: boolean;
};

type RateLimitCriteria = {
  label: string;
  quota: string;
  resetValue: string;
  resetUnit: string;
  enabled?: boolean;
};

type CriteriaMap = Record<string, RateLimitCriteria[]>;

type CriteriaRowsProps = {
  criteria: RateLimitCriteria[];
  onChange: (next: RateLimitCriteria[]) => void;
  disabled?: boolean;
};

const UNITS = ['hour', 'day', 'week', 'month'] as const;

const BACKEND_CRITERIA: RateLimitCriteria[] = [
  { label: 'Request Count', quota: '', resetValue: '', resetUnit: 'hour' },
  { label: 'Token Count', quota: '', resetValue: '', resetUnit: 'hour' },
  { label: 'Cost', quota: '', resetValue: '', resetUnit: 'hour' },
];

const CONSUMER_CRITERIA: RateLimitCriteria[] = [
  { label: 'Request Count', quota: '', resetValue: '', resetUnit: 'hour' },
  { label: 'Token Count', quota: '', resetValue: '', resetUnit: 'hour' },
  { label: 'Cost', quota: '', resetValue: '', resetUnit: 'hour' },
];

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
      const secured =
        !!op?.security ||
        (!!spec?.security &&
          Array.isArray(spec.security) &&
          spec.security.length > 0);

      extracted.push({
        method: methodKey.toUpperCase(),
        path,
        summary: op?.summary || op?.description || undefined,
        secured,
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

function CriteriaRows({
  criteria,
  onChange,
  disabled = false,
}: CriteriaRowsProps) {
  const [rows, setRows] = useState<RateLimitCriteria[]>(criteria);
  const [openMap, setOpenMap] = useState<Record<string, boolean>>({});

  useEffect(() => {
    setRows(criteria);
    setOpenMap((prev) =>
      criteria.reduce<Record<string, boolean>>((acc, item) => {
        acc[item.label] = prev[item.label] ?? Boolean(item.enabled);
        return acc;
      }, {})
    );
  }, [criteria]);

  const updateRow = (label: string, patch: Partial<RateLimitCriteria>) => {
    setRows((prev) => {
      const next = prev.map((r) =>
        r.label === label ? { ...r, ...patch } : r
      );
      onChange(next);
      return next;
    });
  };

  const toggleEnabled = (label: string) => {
    setRows((prev) => {
      let nextEnabled = false;
      const next = prev.map((r) => {
        if (r.label !== label) return r;
        nextEnabled = !r.enabled;
        return { ...r, enabled: nextEnabled };
      });
      onChange(next);
      setOpenMap((prevOpen) => ({ ...prevOpen, [label]: nextEnabled }));
      return next;
    });
  };

  const toggleOpen = (label: string) => {
    setOpenMap((prev) => ({ ...prev, [label]: !prev[label] }));
  };

  return (
    <Stack spacing={1.5}>
      {rows.map((item) => {
        return (
          <Box
            key={item.label}
            sx={{
              borderRadius: 1,
              border: '1px solid',
              borderColor: 'divider',
              backgroundColor: 'background.paper',
              overflow: 'hidden',
            }}
          >
            <Box
              sx={{
                px: 1.5,
                py: 1.25,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 1.5,
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {item.label}
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <FormControlLabel
                  label="Enable"
                  labelPlacement="start"
                  control={
                    <Switch
                      size="small"
                      checked={Boolean(item.enabled)}
                      disabled={disabled}
                      onChange={() => toggleEnabled(item.label)}
                    />
                  }
                />
                <IconButton
                  size="small"
                  disabled={!item.enabled}
                  onClick={() => toggleOpen(item.label)}
                >
                  {openMap[item.label] ? (
                    <ChevronUp size={18} />
                  ) : (
                    <ChevronDown size={18} />
                  )}
                </IconButton>
              </Box>
            </Box>

            <Collapse in={Boolean(openMap[item.label])} timeout="auto">
              <Divider />
              <Box sx={{ px: 1.5, py: 1.25 }}>
                <Grid container spacing={2} alignItems="center">
                  <Grid size={{ xs: 12, md: 4 }}>
                    <FormControl fullWidth>
                      <FormLabel>Quota</FormLabel>
                      <TextField
                        size="small"
                        value={item.quota}
                        disabled={disabled}
                        onChange={(e) =>
                          updateRow(item.label, { quota: e.target.value })
                        }
                        slotProps={
                          item.label === 'Cost'
                            ? {
                                input: {
                                  endAdornment: (
                                    <InputAdornment position="end">$</InputAdornment>
                                  ),
                                },
                              }
                            : undefined
                        }
                      />
                    </FormControl>
                  </Grid>

                  <Grid size={{ xs: 12, md: 8 }}>
                    <FormControl fullWidth>
                      <FormLabel>Reset Duration</FormLabel>

                      <Box sx={{ display: 'flex', gap: 1 }}>
                        <TextField
                          size="small"
                          type="number"
                          value={item.resetValue}
                          disabled={disabled}
                          onChange={(e) =>
                            updateRow(item.label, { resetValue: e.target.value })
                          }
                          sx={{ flex: 1 }}
                        />

                        <Select
                          size="small"
                          value={item.resetUnit || 'hour'}
                          disabled={disabled}
                          onChange={(e) => {
                            updateRow(item.label, {
                              resetUnit: String(e.target.value),
                            });
                          }}
                          sx={{ minWidth: 120 }}
                        >
                          {UNITS.map((u) => (
                            <MenuItem key={u} value={u}>
                              {u.charAt(0).toUpperCase() + u.slice(1)}
                            </MenuItem>
                          ))}
                        </Select>
                      </Box>
                    </FormControl>
                  </Grid>
                </Grid>
              </Box>
            </Collapse>
          </Box>
        );
      })}
    </Stack>
  );
}

function ModeToggle({
  value,
  onChange,
  disableGlobal,
  disableResource,
  disableGlobalReason,
  disableResourceReason,
  disabled = false,
  disabledReason,
}: {
  value: 'global' | 'resource';
  onChange: (v: 'global' | 'resource') => void;
  disableGlobal?: boolean;
  disableResource?: boolean;
  disableGlobalReason?: string;
  disableResourceReason?: string;
  disabled?: boolean;
  disabledReason?: string;
}) {
  const handleChange = (
    _e: React.MouseEvent<HTMLElement>,
    newValue: 'global' | 'resource' | null
  ) => {
    if (newValue) onChange(newValue);
  };

  return (
    <ToggleButtonGroup
      size="small"
      value={value}
      exclusive
      onChange={handleChange}
    >
      <Tooltip
        title={
          disabled
            ? disabledReason || ''
            : disableGlobal
              ? disableGlobalReason || ''
              : ''
        }
        placement="top"
      >
        <Box component="span">
          <ToggleButton
            value="global"
            disabled={disabled || Boolean(disableGlobal)}
            sx={{ textTransform: 'none' }}
          >
            Provider-wide
          </ToggleButton>
        </Box>
      </Tooltip>
      <Tooltip
        title={
          disabled
            ? disabledReason || ''
            : disableResource
              ? disableResourceReason || ''
              : ''
        }
        placement="top"
      >
        <Box component="span">
          <ToggleButton
            value="resource"
            disabled={disabled || Boolean(disableResource)}
            sx={{ textTransform: 'none' }}
          >
            Per Resource
          </ToggleButton>
        </Box>
      </Tooltip>
    </ToggleButtonGroup>
  );
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
      const filteredSpec = filterOpenApiSpecByAccessControl(spec, accessControl);
      return extractResourcesFromSpecJson(filteredSpec ?? spec);
    } catch (err) {
      logger.error('Failed to parse OpenAPI spec:', err);
      return [];
    }
  }
};

const parseLimitToCriteria = (
  limitObj: Record<string, any> | undefined
): RateLimitCriteria[] => {
  const base: RateLimitCriteria[] = [
    { label: 'Request Count', quota: '', resetValue: '', resetUnit: 'hour' },
    { label: 'Token Count', quota: '', resetValue: '', resetUnit: 'hour' },
    { label: 'Cost', quota: '', resetValue: '', resetUnit: 'hour' },
  ];
  if (!limitObj || typeof limitObj !== 'object') return base;

  const mapping: Record<string, { key: string; valueField: string }> = {
    'Request Count': { key: 'request', valueField: 'count' },
    'Token Count': { key: 'token', valueField: 'count' },
    Cost: { key: 'cost', valueField: 'amount' },
  };

  return base.map((item) => {
    const m = mapping[item.label];
    if (!m) return item;
    const entry = limitObj[m.key];
    if (!entry || typeof entry !== 'object') return item;
    return {
      ...item,
      enabled: Boolean(entry.enabled),
      quota: entry[m.valueField] != null ? String(entry[m.valueField]) : '',
      resetValue:
        entry.reset?.duration != null ? String(entry.reset.duration) : '',
      resetUnit: entry.reset?.unit || 'hour',
    };
  });
};

const parsePositiveNumber = (value: string) => {
  if (!value.trim()) return null;
  const num = Number(value);
  if (!Number.isFinite(num) || num < 1) return null;
  return num;
};

const parsePositiveDecimal = (value: string) => {
  if (!value.trim()) return null;
  const num = Number(value);
  if (!Number.isFinite(num) || num <= 0) return null;
  return num;
};

const hasConfiguredCriteria = (criteria: RateLimitCriteria[]) =>
  criteria.some((item) => Boolean(item.enabled));

const hasConfiguredResourceWise = (
  defaultCriteria: RateLimitCriteria[],
  criteriaMap: CriteriaMap
) =>
  hasConfiguredCriteria(defaultCriteria) ||
  Object.values(criteriaMap).some((criteria) => hasConfiguredCriteria(criteria));

const validateCriteria = (criteria: RateLimitCriteria[]) => {
  const limit: Record<string, any> = {};
  const errors: string[] = [];
  let hasEnabled = false;

  criteria.forEach((item) => {
    if (!item.enabled) return;
    hasEnabled = true;
    const quota = parsePositiveNumber(item.quota);
    const resetValue = parsePositiveNumber(item.resetValue);
    const resetUnit = item.resetUnit || 'hour';

    if (item.label === 'Request Count') {
      if (quota === null) {
        errors.push('Request Count quota must be >= 1');
      }
      if (resetValue === null) {
        errors.push('Request Count reset duration must be >= 1');
      }
      limit.request = {
        enabled: true,
        count: quota ?? 0,
        reset: { duration: resetValue ?? 0, unit: resetUnit },
      };
    }

    if (item.label === 'Token Count') {
      if (quota === null) {
        errors.push('Token Count quota must be >= 1');
      }
      if (resetValue === null) {
        errors.push('Token Count reset duration must be >= 1');
      }
      limit.token = {
        enabled: true,
        count: quota ?? 0,
        reset: { duration: resetValue ?? 0, unit: resetUnit },
      };
    }

    if (item.label === 'Cost') {
      const costQuota = parsePositiveDecimal(item.quota);
      if (costQuota === null) {
        errors.push('Cost quota must be > 0');
      }
      if (resetValue === null) {
        errors.push('Cost reset duration must be >= 1');
      }
      limit.cost = {
        enabled: true,
        amount: costQuota ?? 0,
        reset: { duration: resetValue ?? 0, unit: resetUnit },
      };
    }
  });

  return { limit, errors, hasEnabled };
};

const cloneCriteria = (criteria: RateLimitCriteria[]) =>
  criteria.map((item) => ({ ...item }));

type ServiceProviderRateLimitingTabProps = {
  onDirtyChange?: (dirty: boolean) => void;
  onActionsChange?: (
    actions: {
      saveDraftChanges: () => Promise<boolean>;
      discardDraftChanges: () => void;
    } | null
  ) => void;
};

export default function ServiceProviderRateLimitingTab({
  onDirtyChange,
  onActionsChange,
}: ServiceProviderRateLimitingTabProps) {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  const isReadOnlyProvider = Boolean(provider?.readOnly);
  const [backendRateMode, setBackendRateMode] = useState<'global' | 'resource'>(
    'global'
  );
  const [consumerRateMode, setConsumerRateMode] = useState<
    'global' | 'resource'
  >('global');
  const [openBackendResourceKey, setOpenBackendResourceKey] = useState<
    string | null
  >(null);
  const [openConsumerResourceKey, setOpenConsumerResourceKey] = useState<
    string | null
  >(null);
  const [backendSearch, setBackendSearch] = useState('');
  const [sortByRateLimit, setSortByRateLimit] = useState(false);
  const [consumerSearch, setConsumerSearch] = useState('');
  const [sortByConsumerRateLimit, setSortByConsumerRateLimit] = useState(false);
  const [openBackendDefaultRL, setOpenBackendDefaultRL] = useState(false);
  const [openConsumerDefaultRL, setOpenConsumerDefaultRL] = useState(false);
  const [backendDefaultCriteria, setBackendDefaultCriteria] =
    useState<RateLimitCriteria[]>(cloneCriteria(BACKEND_CRITERIA));
  const [consumerDefaultCriteria, setConsumerDefaultCriteria] =
    useState<RateLimitCriteria[]>(cloneCriteria(CONSUMER_CRITERIA));
  const [backendGlobalCriteria, setBackendGlobalCriteria] =
    useState<RateLimitCriteria[]>(cloneCriteria(BACKEND_CRITERIA));
  const [consumerGlobalCriteria, setConsumerGlobalCriteria] =
    useState<RateLimitCriteria[]>(cloneCriteria(CONSUMER_CRITERIA));
  const [backendResourceCriteriaMap, setBackendResourceCriteriaMap] =
    useState<CriteriaMap>({});
  const [consumerResourceCriteriaMap, setConsumerResourceCriteriaMap] =
    useState<CriteriaMap>({});
  const [openapiText, setOpenapiText] = useState('');
  const [isDirty, setIsDirty] = useState(false);
  const showSnackbar = useAIWorkspaceSnackbar();
  const saveDraftChangesRef = useRef<() => Promise<boolean>>(async () => false);
  const discardDraftChangesRef = useRef<() => void>(() => {});

  const loadDraftFromProvider = (providerValue: typeof provider) => {
    const openapi = providerValue?.openapi || '';
    setOpenapiText(openapi);

    setBackendRateMode('global');
    setConsumerRateMode('global');
    setBackendDefaultCriteria(cloneCriteria(BACKEND_CRITERIA));
    setConsumerDefaultCriteria(cloneCriteria(CONSUMER_CRITERIA));
    setBackendGlobalCriteria(cloneCriteria(BACKEND_CRITERIA));
    setConsumerGlobalCriteria(cloneCriteria(CONSUMER_CRITERIA));
    setBackendResourceCriteriaMap({});
    setConsumerResourceCriteriaMap({});

    const rl = providerValue?.rateLimiting;
    if (!rl) return;
    const base = parseOpenApiText(openapi, providerValue?.accessControl);
    const pl = rl.providerLevel;
    const cl = rl.consumerLevel;

    if (pl?.resourceWise) {
      setBackendRateMode('resource');
      if (pl.resourceWise.default) {
        setBackendDefaultCriteria(parseLimitToCriteria(pl.resourceWise.default));
      }
      if (Array.isArray(pl.resourceWise.resources)) {
        const map: CriteriaMap = {};
        pl.resourceWise.resources.forEach((r: any) => {
          if (!r?.resource || !r?.limit) return;
          const parsed = parseLimitToCriteria(r.limit);
          base.forEach((res) => {
            const key = `${res.method.toUpperCase()}-${res.path}`;
            if (res.path === r.resource) {
              map[key] = parsed;
            }
          });
        });
        setBackendResourceCriteriaMap(map);
      }
    } else if (pl?.global) {
      setBackendRateMode('global');
      setBackendGlobalCriteria(parseLimitToCriteria(pl.global));
    }

    if (cl?.resourceWise) {
      setConsumerRateMode('resource');
      if (cl.resourceWise.default) {
        setConsumerDefaultCriteria(parseLimitToCriteria(cl.resourceWise.default));
      }
      if (Array.isArray(cl.resourceWise.resources)) {
        const map: CriteriaMap = {};
        cl.resourceWise.resources.forEach((r: any) => {
          if (!r?.resource || !r?.limit) return;
          const parsed = parseLimitToCriteria(r.limit);
          base.forEach((res) => {
            const key = `${res.method.toUpperCase()}-${res.path}`;
            if (res.path === r.resource) {
              map[key] = parsed;
            }
          });
        });
        setConsumerResourceCriteriaMap(map);
      }
    } else if (cl?.global) {
      setConsumerRateMode('global');
      setConsumerGlobalCriteria(parseLimitToCriteria(cl.global));
    }
  };

  useEffect(() => {
    if (!provider) return;
    loadDraftFromProvider(provider);
    setIsDirty(false);
  }, [provider]);

  useEffect(() => {
    onDirtyChange?.(isDirty);
  }, [isDirty, onDirtyChange]);

  useEffect(() => () => onDirtyChange?.(false), [onDirtyChange]);

  const normalizedResources = useMemo(() => {
    const base = parseOpenApiText(openapiText, provider?.accessControl);
    return base.map((r) => ({ ...r, method: r.method.toUpperCase() }));
  }, [openapiText, provider?.accessControl]);

  const hasResourceRateLimit = (resource: ResourceItem) => {
    const key = `${resource.method}-${resource.path}`;
    const criteria = backendResourceCriteriaMap[key];
    if (!criteria) return false;
    return criteria.some((c) => c.enabled || c.quota.trim() !== '');
  };

  const filteredBackendResources = useMemo(() => {
    const query = backendSearch.trim().toLowerCase();
    const filtered = query
      ? normalizedResources.filter((resource) => {
          const haystack = `${resource.method} ${resource.path} ${
            resource.summary ?? ''
          }`.toLowerCase();
          return haystack.includes(query);
        })
      : [...normalizedResources];

    if (sortByRateLimit) {
      filtered.sort((a, b) => {
        const aHas = hasResourceRateLimit(a) ? 0 : 1;
        const bHas = hasResourceRateLimit(b) ? 0 : 1;
        return aHas - bHas;
      });
    }

    return filtered;
  }, [backendSearch, normalizedResources, sortByRateLimit, backendResourceCriteriaMap]);

  const hasConsumerRateLimit = (resource: ResourceItem) => {
    const key = `${resource.method}-${resource.path}`;
    const criteria = consumerResourceCriteriaMap[key];
    if (!criteria) return false;
    return criteria.some((c) => c.enabled || c.quota.trim() !== '');
  };

  const filteredConsumerResources = useMemo(() => {
    const query = consumerSearch.trim().toLowerCase();
    const filtered = query
      ? normalizedResources.filter((resource) => {
          const haystack = `${resource.method} ${resource.path} ${
            resource.summary ?? ''
          }`.toLowerCase();
          return haystack.includes(query);
        })
      : [...normalizedResources];

    if (sortByConsumerRateLimit) {
      filtered.sort((a, b) => {
        const aHas = hasConsumerRateLimit(a) ? 0 : 1;
        const bHas = hasConsumerRateLimit(b) ? 0 : 1;
        return aHas - bHas;
      });
    }

    return filtered;
  }, [consumerSearch, normalizedResources, sortByConsumerRateLimit, consumerResourceCriteriaMap]);

  const backendHasGlobalConfig = hasConfiguredCriteria(backendGlobalCriteria);
  const backendHasResourceConfig = hasConfiguredResourceWise(
    backendDefaultCriteria,
    backendResourceCriteriaMap
  );
  const consumerHasGlobalConfig = hasConfiguredCriteria(consumerGlobalCriteria);
  const consumerHasResourceConfig = hasConfiguredResourceWise(
    consumerDefaultCriteria,
    consumerResourceCriteriaMap
  );

  const buildRateLimitingPayload = () => {
    const providerLevel: Record<string, any> = {};
    const consumerLevel: Record<string, any> = {};
    const errors: string[] = [];

    if (backendRateMode === 'global') {
      const { limit, errors: limitErrors } = validateCriteria(
        backendGlobalCriteria
      );
      errors.push(...limitErrors);
      providerLevel.global = limit;
    } else {
      const {
        limit: defaultLimit,
        errors: defaultErrors,
        hasEnabled: defaultEnabled,
      } = validateCriteria(backendDefaultCriteria);
      if (defaultEnabled) {
        errors.push(...defaultErrors);
      }
      const resources = normalizedResources
        .map((resource) => {
          const key = `${resource.method}-${resource.path}`;
          const criteria =
            backendResourceCriteriaMap[key] ??
            BACKEND_CRITERIA.map((c) => ({ ...c, enabled: false }));
          const {
            limit,
            errors: resErrors,
            hasEnabled: resEnabled,
          } = validateCriteria(criteria);
          if (!resEnabled) return null;
          errors.push(...resErrors.map((e) => `${resource.path}: ${e}`));
          return {
            resource: resource.path,
            limit,
          };
        })
        .filter(Boolean);
      providerLevel.resourceWise = {
        default: {
          request: { enabled: false },
          ...defaultLimit,
        },
        resources: resources.length ? resources : [],
      };
    }

    if (consumerRateMode === 'global') {
      const { limit, errors: limitErrors } = validateCriteria(
        consumerGlobalCriteria
      );
      errors.push(...limitErrors);
      consumerLevel.global = limit;
    } else {
      const {
        limit: defaultLimit,
        errors: defaultErrors,
        hasEnabled: defaultEnabled,
      } = validateCriteria(consumerDefaultCriteria);
      if (defaultEnabled) {
        errors.push(...defaultErrors);
      }
      const resources = normalizedResources
        .map((resource) => {
          const key = `${resource.method}-${resource.path}`;
          const criteria =
            consumerResourceCriteriaMap[key] ??
            CONSUMER_CRITERIA.map((c) => ({ ...c, enabled: false }));
          const {
            limit,
            errors: resErrors,
            hasEnabled: resEnabled,
          } = validateCriteria(criteria);
          if (!resEnabled) return null;
          errors.push(...resErrors.map((e) => `${resource.path}: ${e}`));
          return {
            resource: resource.path,
            limit,
          };
        })
        .filter(Boolean);
      consumerLevel.resourceWise = {
        default: {
          request: { enabled: false },
          ...defaultLimit,
        },
        resources: resources.length ? resources : [],
      };
    }

    return {
      payload: {
        providerLevel,
        consumerLevel,
      },
      errors,
    };
  };

  const handleSaveRateLimiting = async (): Promise<boolean> => {
    if (!provider || isLoading || error || isReadOnlyProvider) return false;
    if (backendHasGlobalConfig && backendHasResourceConfig) {
      showSnackbar(
        'Backend cannot have both Provider-wide and Per Resource values. Remove one side and try again.',
        'error'
      );
      return false;
    }
    if (consumerHasGlobalConfig && consumerHasResourceConfig) {
      showSnackbar(
        'Per Consumer cannot have both Provider-wide and Per Resource values. Remove one side and try again.',
        'error'
      );
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
    const { payload, errors: validationErrors } = buildRateLimitingPayload();

    if (validationErrors.length) {
      showSnackbar(validationErrors[0], 'error');
      return false;
    }

    const fullPayload = {
      ...updatePayload,
      rateLimiting: payload,
    };

    try {
      await updateProvider(fullPayload);
      if (!isDraftMode) {
        showSnackbar('Rate limits updated successfully.', 'success');
      }
      setIsDirty(false);
      return true;
    } catch (err) {
      if (!isDraftMode) {
        showSnackbar('Failed to update rate limits.', 'error');
      }
      logger.error('Failed to update rate limits:', err);
      return false;
    }
  };

  const handleCancelRateLimitingChanges = () => {
    if (!provider) return;
    loadDraftFromProvider(provider);
    setIsDirty(false);
  };

  useEffect(() => {
    saveDraftChangesRef.current = handleSaveRateLimiting;
    discardDraftChangesRef.current = handleCancelRateLimitingChanges;
  });

  useEffect(() => {
    if (!onActionsChange) return undefined;
    onActionsChange({
      saveDraftChanges: () => saveDraftChangesRef.current(),
      discardDraftChanges: () => discardDraftChangesRef.current(),
    });
    return () => onActionsChange(null);
  }, [onActionsChange]);

  const handleBackendModeChange = (value: 'global' | 'resource') => {
    setBackendRateMode(value);
  };

  const handleConsumerModeChange = (value: 'global' | 'resource') => {
    setConsumerRateMode(value);
  };

  const updateBackendResourceCriteria = (
    key: string,
    next: RateLimitCriteria[]
  ) => {
    setBackendResourceCriteriaMap((prev) => ({ ...prev, [key]: next }));
    setIsDirty(true);
  };

  const updateConsumerResourceCriteria = (
    key: string,
    next: RateLimitCriteria[]
  ) => {
    setConsumerResourceCriteriaMap((prev) => ({ ...prev, [key]: next }));
    setIsDirty(true);
  };

  return (
    <>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Stack spacing={2}>
                <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderRateLimitingTab.backend.rate.limits"
                    defaultMessage='Backend'
                  />
                </Typography>

                <ModeToggle
                  value={backendRateMode}
                  onChange={handleBackendModeChange}
                  disabled={isReadOnlyProvider}
                  disabledReason={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                  disableGlobal={
                    backendRateMode !== 'global' && backendHasResourceConfig
                  }
                  disableResource={
                    backendRateMode !== 'resource' && backendHasGlobalConfig
                  }
                  disableGlobalReason='If you need Provider-wide, remove Per Resource values first.'
                  disableResourceReason='If you need Per Resource, remove Provider-wide values first.'
                />

                {backendRateMode === 'resource' && (
                  <Stack spacing={1}>
                    <Box>
                      <Box
                        onClick={() =>
                          setOpenBackendDefaultRL(!openBackendDefaultRL)
                        }
                        sx={{
                          cursor: 'pointer',
                          userSelect: 'none',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          px: 1.5,
                          py: 1.25,
                          borderRadius: 1,
                          border: '1px solid',
                          borderColor: 'divider',
                          backgroundColor: 'action.hover',
                        }}
                      >
                        <Stack direction="row" spacing={1} alignItems="center">
                          <Layers size={16} />
                          <Typography variant="subtitle2">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderRateLimitingTab.apply.default.rate.limiting"
                              defaultMessage='Limit per Resource'
                            />
                          </Typography>
                        </Stack>
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            setOpenBackendDefaultRL(!openBackendDefaultRL);
                          }}
                        >
                          {openBackendDefaultRL ? (
                            <ChevronUp size={18} />
                          ) : (
                            <ChevronDown size={18} />
                          )}
                        </IconButton>
                      </Box>
                      <Collapse
                        in={openBackendDefaultRL}
                        timeout="auto"
                        unmountOnExit
                      >
                        <Box
                          sx={{
                            mt: 1,
                            px: 1.5,
                            py: 1.25,
                            borderRadius: 1,
                            border: '1px solid',
                            borderColor: 'divider',
                            backgroundColor: 'background.paper',
                          }}
                        >
                          <CriteriaRows
                            criteria={backendDefaultCriteria}
                            disabled={isReadOnlyProvider}
                            onChange={(next) => {
                              setBackendDefaultCriteria(next);
                              setIsDirty(true);
                            }}
                          />
                        </Box>
                      </Collapse>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <TextField
                        size="small"
                        fullWidth
                        placeholder="Search resources..."
                        value={backendSearch}
                        onChange={(event) => setBackendSearch(event.target.value)}
                      />
                      <Tooltip
                        title={
                          sortByRateLimit
                            ? 'Show default order'
                            : 'Show resources with rate limits first'
                        }
                        arrow
                      >
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<ArrowUpDown size={16} />}
                          onClick={() => setSortByRateLimit((prev) => !prev)}
                          sx={{
                            whiteSpace: 'nowrap',
                            flexShrink: 0,
                            ...(sortByRateLimit && {
                              borderColor: 'success.main',
                              color: 'success.main',
                              backgroundColor: 'rgba(46, 125, 50, 0.08)',
                            }),
                          }}
                        >
                          Rate limited
                        </Button>
                      </Tooltip>
                    </Box>
                    <Box
                      sx={{
                        minHeight: 220,
                        maxHeight: 420,
                        overflowY: 'auto',
                        pr: 0.5,
                      }}
                    >
                      {filteredBackendResources.map((resource) => {
                        const key = `${resource.method}-${resource.path}`;
                        const isOpen = openBackendResourceKey === key;
                        const criteria =
                          backendResourceCriteriaMap[key] ?? BACKEND_CRITERIA;

                        return (
                          <Box key={key} sx={{ minHeight: 44, mb: 0.8 }}>
                            <ResourceRow
                              resource={resource}
                              onClick={() =>
                                setOpenBackendResourceKey(isOpen ? null : key)
                              }
                              trailing={
                                <Box
                                  sx={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 0.5,
                                  }}
                                >
                                  {hasResourceRateLimit(resource) && (
                                    <Tooltip title="Rate limits configured" arrow>
                                      <Box
                                        component="span"
                                        sx={{
                                          display: 'inline-flex',
                                          color: 'success.main',
                                        }}
                                      >
                                        <Clock2 size={18} />
                                      </Box>
                                    </Tooltip>
                                  )}
                                  <IconButton
                                    size="small"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setOpenBackendResourceKey(
                                        isOpen ? null : key
                                      );
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
                              <Box
                                sx={{
                                  mt: 1,
                                  px: 1.5,
                                  py: 1.25,
                                  borderRadius: 1,
                                  border: '1px solid',
                                  borderColor: 'divider',
                                  backgroundColor: 'background.paper',
                                }}
                              >
                                <CriteriaRows
                                  criteria={criteria}
                                  disabled={isReadOnlyProvider}
                                  onChange={(next) =>
                                    updateBackendResourceCriteria(key, next)
                                  }
                                />
                              </Box>
                            </Collapse>
                          </Box>
                        );
                      })}
                    </Box>
                  </Stack>
                )}
                {backendRateMode === 'global' && (
                  <>
                    <Divider />
                    <CriteriaRows
                      criteria={backendGlobalCriteria}
                      disabled={isReadOnlyProvider}
                      onChange={(next) => {
                        setBackendGlobalCriteria(next);
                        setIsDirty(true);
                      }}
                    />
                  </>
                )}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card
            variant="outlined"
          >
            <CardContent>
              <Stack spacing={2}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderRateLimitingTab.per.consumer.rate.limits"
                      defaultMessage='Per Consumer'
                    />
                  </Typography>
                </Stack>

                <ModeToggle
                  value={consumerRateMode}
                  onChange={handleConsumerModeChange}
                  disabled={isReadOnlyProvider}
                  disabledReason={GATEWAY_MANAGED_ARTIFACT_TOOLTIP}
                  disableGlobal={consumerRateMode !== 'global' && consumerHasResourceConfig}
                  disableResource={consumerRateMode !== 'resource' && consumerHasGlobalConfig}
                  disableGlobalReason='If you need Provider-wide, remove Per Resource values first.'
                  disableResourceReason='If you need Per Resource, remove Provider-wide values first.'
                />
                {consumerRateMode === 'resource' && (
                  <Stack spacing={1}>
                    <Box>
                      <Box
                        onClick={() =>
                          setOpenConsumerDefaultRL(!openConsumerDefaultRL)
                        }
                        sx={{
                          cursor: 'pointer',
                          userSelect: 'none',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          px: 1.5,
                          py: 1.25,
                          borderRadius: 1,
                          border: '1px solid',
                          borderColor: 'divider',
                          backgroundColor: 'action.hover',
                        }}
                      >
                        <Stack direction="row" spacing={1} alignItems="center">
                          <Layers size={16} />
                          <Typography variant="subtitle2">
                            <FormattedMessage
                              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderRateLimitingTab.apply.default.rate.limiting"
                              defaultMessage='Limit per Resource'
                            />
                          </Typography>
                        </Stack>
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            setOpenConsumerDefaultRL(!openConsumerDefaultRL);
                          }}
                        >
                          {openConsumerDefaultRL ? (
                            <ChevronUp size={18} />
                          ) : (
                            <ChevronDown size={18} />
                          )}
                        </IconButton>
                      </Box>
                      <Collapse
                        in={openConsumerDefaultRL}
                        timeout="auto"
                        unmountOnExit
                      >
                        <Box
                          sx={{
                            mt: 1,
                            px: 1.5,
                            py: 1.25,
                            borderRadius: 1,
                            border: '1px solid',
                            borderColor: 'divider',
                            backgroundColor: 'background.paper',
                          }}
                        >
                          <CriteriaRows
                            criteria={consumerDefaultCriteria}
                            disabled={isReadOnlyProvider}
                            onChange={(next) => {
                              setConsumerDefaultCriteria(next);
                              setIsDirty(true);
                            }}
                          />
                        </Box>
                      </Collapse>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <TextField
                        size="small"
                        fullWidth
                        placeholder="Search resources..."
                        value={consumerSearch}
                        onChange={(event) =>
                          setConsumerSearch(event.target.value)
                        }
                      />
                      <Tooltip
                        title={
                          sortByConsumerRateLimit
                            ? 'Show default order'
                            : 'Show resources with rate limits first'
                        }
                        arrow
                      >
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<ArrowUpDown size={16} />}
                          onClick={() => setSortByConsumerRateLimit((prev) => !prev)}
                          sx={{
                            whiteSpace: 'nowrap',
                            flexShrink: 0,
                            ...(sortByConsumerRateLimit && {
                              borderColor: 'success.main',
                              color: 'success.main',
                              backgroundColor: 'rgba(46, 125, 50, 0.08)',
                            }),
                          }}
                        >
                          Rate limited
                        </Button>
                      </Tooltip>
                    </Box>
                    <Box
                      sx={{
                        minHeight: 220,
                        maxHeight: 420,
                        overflowY: 'auto',
                        pr: 0.5,
                      }}
                    >
                      {filteredConsumerResources.map((resource) => {
                        const key = `${resource.method}-${resource.path}`;
                        const isOpen = openConsumerResourceKey === key;
                        const criteria =
                          consumerResourceCriteriaMap[key] ?? CONSUMER_CRITERIA;

                        return (
                          <Box key={key} sx={{ minHeight: 44, mb: 0.8 }}>
                            <ResourceRow
                              resource={resource}
                              onClick={() =>
                                setOpenConsumerResourceKey(isOpen ? null : key)
                              }
                              trailing={
                                <Box
                                  sx={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    gap: 0.5,
                                  }}
                                >
                                  {hasConsumerRateLimit(resource) && (
                                    <Tooltip title="Rate limits configured" arrow>
                                      <Box
                                        component="span"
                                        sx={{
                                          display: 'inline-flex',
                                          color: 'success.main',
                                        }}
                                      >
                                        <Clock2 size={18} />
                                      </Box>
                                    </Tooltip>
                                  )}
                                  <IconButton
                                    size="small"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setOpenConsumerResourceKey(
                                        isOpen ? null : key
                                      );
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
                              <Box
                                sx={{
                                  mt: 1,
                                  px: 1.5,
                                  py: 1.25,
                                  borderRadius: 1,
                                  border: '1px solid',
                                  borderColor: 'divider',
                                  backgroundColor: 'background.paper',
                                }}
                              >
                                <CriteriaRows
                                  criteria={criteria}
                                  disabled={isReadOnlyProvider}
                                  onChange={(next) =>
                                    updateConsumerResourceCriteria(key, next)
                                  }
                                />
                              </Box>
                            </Collapse>
                          </Box>
                        );
                      })}
                    </Box>
                  </Stack>
                )}
                {consumerRateMode === 'global' && (
                  <>
                    <Divider />
                    <CriteriaRows
                      criteria={consumerGlobalCriteria}
                      disabled={isReadOnlyProvider}
                      onChange={(next) => {
                        setConsumerGlobalCriteria(next);
                        setIsDirty(true);
                      }}
                    />
                  </>
                )}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </>
  );
}
