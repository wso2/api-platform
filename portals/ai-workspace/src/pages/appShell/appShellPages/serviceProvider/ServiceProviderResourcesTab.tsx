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

import React, { useMemo, useRef, useState, useEffect } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { HelpCircle, PenLine, Upload } from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { useLLMProvider } from '../../../../contexts/llmProvider';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { logger } from '../../../../utils/logger';
import {
  DisabledActionTooltip,
} from '../../../../utils/readOnlyArtifacts';
import NoData from '../../../../assets/images/NoData.svg';
import { ExpandableResourceRow } from '../../../../Components/ResourceView';
import { FormattedMessage } from 'react-intl';

type ResourceItem = {
  method: string;
  path: string;
  summary?: string;
  secured?: boolean;
  disabled?: boolean;
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

function getResourceKey(resource: ResourceItem) {
  return `${resource.method.toUpperCase()}::${resource.path}`;
}

function parseOpenApiSpec(text: string): Record<string, unknown> | null {
  if (!text.trim()) return null;
  try {
    const spec = JSON.parse(text);
    return spec && typeof spec === 'object'
      ? (spec as Record<string, unknown>)
      : null;
  } catch {
    try {
      const spec = YAML.parse(text);
      return spec && typeof spec === 'object'
        ? (spec as Record<string, unknown>)
        : null;
    } catch (err) {
      logger.error('Failed to parse OpenAPI spec:', err);
      return null;
    }
  }
}

function buildOperationSpec(
  rootSpec: Record<string, unknown>,
  method: string,
  path: string
): Record<string, unknown> | null {
  const methodKey = method.toLowerCase();
  const paths = rootSpec.paths as Record<string, unknown> | undefined;
  const pathEntry = paths?.[path] as Record<string, unknown> | undefined;
  const operation = pathEntry?.[methodKey] as
    | Record<string, unknown>
    | undefined;
  if (!operation) return null;

  const operationPathItem: Record<string, unknown> = {
    [methodKey]: operation,
  };

  const commonPathKeys = ['parameters', 'servers', 'summary', 'description'];
  commonPathKeys.forEach((key) => {
    if (pathEntry?.[key] !== undefined) {
      operationPathItem[key] = pathEntry[key];
    }
  });

  return {
    ...(rootSpec.openapi ? { openapi: rootSpec.openapi } : {}),
    ...(rootSpec.swagger ? { swagger: rootSpec.swagger } : {}),
    ...(rootSpec.info ? { info: rootSpec.info } : {}),
    ...(rootSpec.servers ? { servers: rootSpec.servers } : {}),
    ...(rootSpec.components ? { components: rootSpec.components } : {}),
    ...(rootSpec.security ? { security: rootSpec.security } : {}),
    ...(rootSpec.tags ? { tags: rootSpec.tags } : {}),
    ...(rootSpec.basePath ? { basePath: rootSpec.basePath } : {}),
    ...(rootSpec.host ? { host: rootSpec.host } : {}),
    ...(rootSpec.schemes ? { schemes: rootSpec.schemes } : {}),
    ...(rootSpec.consumes ? { consumes: rootSpec.consumes } : {}),
    ...(rootSpec.produces ? { produces: rootSpec.produces } : {}),
    ...(rootSpec.definitions ? { definitions: rootSpec.definitions } : {}),
    ...(rootSpec.securityDefinitions
      ? { securityDefinitions: rootSpec.securityDefinitions }
      : {}),
    paths: {
      [path]: operationPathItem,
    },
  };
}

export default function ServiceProviderResourcesTab() {
  const { provider, isLoading, error, updateProvider, isDraftMode } =
    useLLMProvider();
  const isReadOnlyProvider = Boolean(provider?.readOnly);
  const [resourceMode, setResourceMode] = useState<'allow' | 'deny'>('allow');
  const [pendingResourceMode, setPendingResourceMode] = useState<
    'allow' | 'deny' | null
  >(null);
  const [resourceModeConfirmOpen, setResourceModeConfirmOpen] = useState(false);
  const [exceptionResources, setExceptionResources] = useState<ResourceItem[]>(
    []
  );
  const [availableSearch, setAvailableSearch] = useState('');
  const [exceptionSearch, setExceptionSearch] = useState('');
  const [openKey, setOpenKey] = useState<string | null>(null);
  const [selectedAvailableKeys, setSelectedAvailableKeys] = useState<string[]>(
    []
  );
  const [selectedExceptionKeys, setSelectedExceptionKeys] = useState<string[]>(
    []
  );
  const [openapiText, setOpenapiText] = useState('');
  const { hasPermission } = useAppAuth();
  const { currentProject, currentOrganization } = useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const isAdminOrgLevel = hasPermission(SCOPES.LLM_PROVIDER_MANAGE) && !isProjectLevel;
  const showSnackbar = useAIWorkspaceSnackbar();

  useEffect(() => {
    if (!provider) return;
    const mode = String(provider.accessControl?.mode || '')
      .toLowerCase()
      .replace(/-/g, '_');
    setResourceMode(mode === 'deny_all' ? 'deny' : 'allow');
    setOpenapiText(provider.openapi || '');
    const exceptions = provider.accessControl?.exceptions || [];
    setExceptionResources(
      exceptions.map((exception) => ({
        method: exception.methods?.[0] || 'GET',
        path: exception.path,
      }))
    );
  }, [provider]);

  const parseOpenApiText = (text: string): ResourceItem[] => {
    if (!text.trim()) return [];
    try {
      const spec = JSON.parse(text);
      return extractResourcesFromSpecJson(spec);
    } catch {
      try {
        const spec = YAML.parse(text);
        return extractResourcesFromSpecJson(spec);
      } catch (err) {
        logger.error('Failed to parse OpenAPI spec:', err);
        return [];
      }
    }
  };

  const updateAccessControl = async (
    mode: 'allow' | 'deny',
    exceptions: ResourceItem[],
    nextOpenapi: string
  ) => {
    if (!provider || isLoading || error || isReadOnlyProvider) return;
    const {
      status,
      createdAt,
      createdBy,
      updatedAt,
      lastUpdated,
      ...updatePayload
    } = provider;
    const accessControlMode = mode === 'allow' ? 'allow_all' : 'deny_all';
    const exceptionPayload = exceptions.map((resource) => ({
      path: resource.path,
      methods: [resource.method.toUpperCase()],
    }));

    try {
      await updateProvider({
        ...updatePayload,
        openapi: nextOpenapi,
        accessControl: {
          mode: accessControlMode,
          exceptions: exceptionPayload,
        },
      });
      if (!isDraftMode) {
        showSnackbar('Resources updated successfully.', 'success');
      }
    } catch (err) {
      if (!isDraftMode) {
        showSnackbar('Failed to update resources.', 'error');
      }
      logger.error('Failed to update resources:', err);
    }
  };

  const handleResourceModeChange = (
    _event: React.MouseEvent<HTMLElement>,
    newMode: 'allow' | 'deny' | null
  ) => {
    if (isReadOnlyProvider) return;
    if (!newMode || newMode === resourceMode) return;
    setPendingResourceMode(newMode);
    setResourceModeConfirmOpen(true);
  };

  const handleCancelResourceModeChange = () => {
    setResourceModeConfirmOpen(false);
    setPendingResourceMode(null);
  };

  const handleApplyResourceModeChange = () => {
    if (isReadOnlyProvider) {
      handleCancelResourceModeChange();
      return;
    }
    if (!pendingResourceMode || pendingResourceMode === resourceMode) {
      handleCancelResourceModeChange();
      return;
    }
    setResourceMode(pendingResourceMode);
    setResourceModeConfirmOpen(false);
    void updateAccessControl(
      pendingResourceMode,
      exceptionResources,
      openapiText
    );
    setPendingResourceMode(null);
  };

  const [updateSpecModalOpen, setUpdateSpecModalOpen] = useState(false);
  const [specUrl, setSpecUrl] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const originalSpecUrlRef = useRef('');

  useEffect(() => {
    const templateId = provider?.template;
    const organizationId = currentOrganization?.uuid;

    if (!templateId || !organizationId) {
      originalSpecUrlRef.current = '';
      setSpecUrl('');
      return;
    }

    let isMounted = true;
    void (async () => {
      try {
        const template = await providerTemplateApis.getProviderTemplate(
          templateId,
          PLATFORM_API_BASE_URL
        );
        if (!isMounted) return;
        const url = template.metadata?.openapiSpecUrl || '';
        originalSpecUrlRef.current = url;
        setSpecUrl(url);
      } catch (fetchError) {
        if (!isMounted) return;
        logger.error(
          `Failed to fetch provider template ${templateId}:`,
          fetchError
        );
        originalSpecUrlRef.current = '';
        setSpecUrl('');
      }
    })();

    return () => {
      isMounted = false;
    };
  }, [PLATFORM_API_BASE_URL, currentOrganization?.uuid, provider?.template]);
  const updateSpecFileInputRef = useRef<HTMLInputElement | null>(null);

  const handleUpdateSpecUploadClick = () => {
    if (isReadOnlyProvider) return;
    updateSpecFileInputRef.current?.click();
  };

  const handleUpdateSpecFileChange = async (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    if (isReadOnlyProvider) return;
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      const text = await file.text();
      const imported = parseOpenApiText(text);
      if (!imported.length) {
        throw new Error('No resources found in specification.');
      }
      setOpenapiText(text);
      await updateAccessControl(resourceMode, exceptionResources, text);
      setUpdateSpecModalOpen(false);
      setSpecUrl(originalSpecUrlRef.current);
      if (!isDraftMode) {
        showSnackbar('OpenAPI definition updated successfully.', 'success');
      }
    } catch (err) {
      showSnackbar('Failed to import specification.', 'error');
      logger.error('Failed to import spec file:', err);
    } finally {
      e.target.value = '';
    }
  };

  const applySpecificationFromUrl = async (url: string) => {
    if (isReadOnlyProvider) return;
    if (!url.trim()) return;
    setIsFetchingSpec(true);
    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`Failed to fetch: ${response.statusText}`);
      }
      const text = await response.text();
      const imported = parseOpenApiText(text);
      if (!imported.length) {
        throw new Error('No resources found in specification.');
      }
      setOpenapiText(text);
      await updateAccessControl(resourceMode, exceptionResources, text);
      setUpdateSpecModalOpen(false);
      setSpecUrl(originalSpecUrlRef.current);
      if (!isDraftMode) {
        showSnackbar('OpenAPI definition updated successfully.', 'success');
      }
    } catch (err) {
      showSnackbar('Failed to fetch specification from URL.', 'error');
      logger.error('Failed to fetch spec from URL:', err);
    } finally {
      setIsFetchingSpec(false);
    }
  };

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const handleUploadClick = () => {
    if (isReadOnlyProvider) return;
    fileInputRef.current?.click();
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (isReadOnlyProvider) return;
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      const text = await file.text();
      const imported = parseOpenApiText(text);
      if (!imported.length) {
        throw new Error('No resources found in specification.');
      }
      setOpenapiText(text);
      await updateAccessControl(resourceMode, exceptionResources, text);
      if (!isDraftMode) {
        showSnackbar('Resources imported successfully.', 'success');
      }
    } catch (err) {
      showSnackbar('Failed to import specification.', 'error');
      logger.error('Failed to import spec file:', err);
    } finally {
      e.target.value = '';
    }
  };

  const normalized = useMemo(() => {
    const base = parseOpenApiText(openapiText);
    return base.map((r) => ({
      ...r,
      method: r.method.toUpperCase(),
    }));
  }, [openapiText]);
  const parsedOpenApiSpec = useMemo(
    () => parseOpenApiSpec(openapiText),
    [openapiText]
  );
  const operationSpecByKey = useMemo(() => {
    const map = new Map<string, Record<string, unknown>>();
    if (!parsedOpenApiSpec) return map;

    normalized.forEach((resource) => {
      const operationSpec = buildOperationSpec(
        parsedOpenApiSpec,
        resource.method,
        resource.path
      );
      if (operationSpec) {
        map.set(getResourceKey(resource), operationSpec);
      }
    });

    return map;
  }, [normalized, parsedOpenApiSpec]);

  const resourceSummaryByKey = useMemo(() => {
    const map = new Map<string, string>();
    normalized.forEach((resource) => {
      if (resource.summary) {
        map.set(getResourceKey(resource), resource.summary);
      }
    });
    return map;
  }, [normalized]);

  const exceptionKeySet = useMemo(() => {
    const set = new Set<string>();
    exceptionResources.forEach((resource) => {
      set.add(getResourceKey(resource));
    });
    return set;
  }, [exceptionResources]);

  const availableResources = useMemo(
    () =>
      normalized.filter(
        (resource) => !exceptionKeySet.has(getResourceKey(resource))
      ),
    [exceptionKeySet, normalized]
  );

  const filteredAvailableResources = useMemo(() => {
    const query = availableSearch.trim().toLowerCase();
    if (!query) return availableResources;
    return availableResources.filter((resource) => {
      const haystack = `${resource.method} ${resource.path} ${
        resource.summary ?? ''
      }`.toLowerCase();
      return haystack.includes(query);
    });
  }, [availableResources, availableSearch]);

  const filteredExceptionResources = useMemo(() => {
    const query = exceptionSearch.trim().toLowerCase();
    if (!query) return exceptionResources;
    return exceptionResources.filter((resource) => {
      const summary =
        resource.summary || resourceSummaryByKey.get(getResourceKey(resource));
      const haystack = `${resource.method} ${resource.path} ${
        summary ?? ''
      }`.toLowerCase();
      return haystack.includes(query);
    });
  }, [exceptionResources, exceptionSearch, resourceSummaryByKey]);

  const selectedAvailableKeySet = useMemo(
    () => new Set(selectedAvailableKeys),
    [selectedAvailableKeys]
  );
  const selectedExceptionKeySet = useMemo(
    () => new Set(selectedExceptionKeys),
    [selectedExceptionKeys]
  );

  const toggleAvailableSelection = (resource: ResourceItem) => {
    const key = getResourceKey(resource);
    setSelectedAvailableKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]
    );
  };

  const toggleExceptionSelection = (resource: ResourceItem) => {
    const key = getResourceKey(resource);
    setSelectedExceptionKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]
    );
  };

  const moveSelectedToExceptions = () => {
    if (isReadOnlyProvider) return;
    if (!selectedAvailableKeys.length) return;
    const selected = availableResources.filter((resource) =>
      selectedAvailableKeySet.has(getResourceKey(resource))
    );
    if (!selected.length) return;
    const nextExceptions = [...exceptionResources, ...selected];
    setExceptionResources(nextExceptions);
    setSelectedAvailableKeys([]);
    void updateAccessControl(resourceMode, nextExceptions, openapiText);
  };

  const moveSelectedToAvailable = () => {
    if (isReadOnlyProvider) return;
    if (!selectedExceptionKeys.length) return;
    const nextExceptions = exceptionResources.filter(
      (resource) => !selectedExceptionKeySet.has(getResourceKey(resource))
    );
    setExceptionResources(nextExceptions);
    setSelectedExceptionKeys([]);
    void updateAccessControl(resourceMode, nextExceptions, openapiText);
  };

  const moveAllToExceptions = () => {
    if (isReadOnlyProvider) return;
    const nextExceptions = normalized;
    setExceptionResources(nextExceptions);
    setSelectedAvailableKeys([]);
    void updateAccessControl(resourceMode, nextExceptions, openapiText);
  };

  const moveAllToAvailable = () => {
    if (isReadOnlyProvider) return;
    setExceptionResources([]);
    setSelectedExceptionKeys([]);
    void updateAccessControl(resourceMode, [], openapiText);
  };

  if (!isAdminOrgLevel) {
    return (
      <Stack spacing={2}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1.5,
            flexWrap: 'wrap',
          }}
        >
          {/* <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Button variant="outlined" size="small" onClick={handleUploadClick}>
              Import resources from specification
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              hidden
              accept=".json,.yaml,.yml"
              onChange={handleFileChange}
            />
          </Box> */}
        </Box>

        <Stack spacing={1.25}>
          {normalized.map((resource) => {
            const key = getResourceKey(resource);
            const isOpen = openKey === key;

            return (
              <ExpandableResourceRow
                key={key}
                resource={resource}
                isOpen={isOpen}
                operationSpec={operationSpecByKey.get(key)}
                onRowClick={() => setOpenKey(isOpen ? null : key)}
                onToggleOpen={() => setOpenKey(isOpen ? null : key)}
              />
            );
          })}

          {!normalized.length && (
            <Stack
              spacing={1}
              alignItems="center"
              justifyContent="center"
              sx={{ py: 2, textAlign: 'center' }}
            >
              <Box
                component="img"
                src={NoData}
                alt="No available resources"
                sx={{ width: 80, maxWidth: '80%' }}
              />
              <Typography variant="body2" color="text.secondary">
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.no.available.resources"
                  defaultMessage={'No available resources.'}
                />
              </Typography>
            </Stack>
          )}
        </Stack>
      </Stack>
    );
  }

  return (
    <Box>
      <Stack spacing={2}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 2,
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.default.mode"
                defaultMessage="Mode"
              />
            </Typography>

            <ToggleButtonGroup
              size="small"
              value={resourceMode}
              exclusive
              onChange={handleResourceModeChange}
            >
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Box component="span">
                  <ToggleButton
                    value="allow"
                    disabled={isReadOnlyProvider}
                    sx={{ textTransform: 'none' }}
                  >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.allow.all"
                  defaultMessage={'Allow all'}
                />{' '}
                <Tooltip
                  arrow
                  title={
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.allow.all.tooltip"
                      defaultMessage="Allow all exposes every available resource. Any exception you add will be hidden."
                    />
                  }
                  >
                    <IconButton size="small">
                      <HelpCircle size={16} />
                    </IconButton>
                  </Tooltip>
                  </ToggleButton>
                </Box>
              </DisabledActionTooltip>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Box component="span">
                  <ToggleButton
                    value="deny"
                    disabled={isReadOnlyProvider}
                    sx={{ textTransform: 'none' }}
                  >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.deny.all"
                  defaultMessage={'Deny all'}
                />{' '}
                <Tooltip
                  arrow
                  title={
                    <FormattedMessage
                      id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.deny.all.tooltip"
                      defaultMessage="Deny all hides every available resource. Any exception you add will be exposed."
                    />
                  }
                  >
                    <IconButton size="small">
                      <HelpCircle size={16} />
                    </IconButton>
                  </Tooltip>
                  </ToggleButton>
                </Box>
              </DisabledActionTooltip>
            </ToggleButtonGroup>
          </Box>

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            {/* <Button
              variant="outlined"
              size="small"
              startIcon={<Upload size={16} />}
              onClick={handleUploadClick}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.import.resources.from.specification"
                defaultMessage={'Import resources from specification'}
              />
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              hidden
              accept=".json,.yaml,.yml"
              onChange={handleFileChange}
            /> */}
            <DisabledActionTooltip disabled={isReadOnlyProvider}>
              <Button
                variant="outlined"
                size="small"
                startIcon={<PenLine size={16} />}
                onClick={() => setUpdateSpecModalOpen(true)}
                disabled={isReadOnlyProvider}
              >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.update.openapi.definition"
                  defaultMessage={'Update OpenAPI Definition'}
                />
              </Button>
            </DisabledActionTooltip>
          </Box>
        </Box>

        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '1fr auto 1fr' },
            gap: 2,
          }}
        >
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              p: 2,
              minHeight: 360,
              maxHeight: 560,
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
          >
            <Stack spacing={1.5} sx={{ flex: 1, minHeight: 0 }}>
              <Box>
                <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                  {resourceMode === 'allow'
                    ? 'Allowed Resources'
                    : 'Denied Resources'}
                </Typography>
                {/* <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.show.all.available.resources"
                    defaultMessage={'Show all available resources'}
                  />
                </Typography> */}
              </Box>

              <TextField
                size="small"
                placeholder="Search resources"
                value={availableSearch}
                fullWidth
                onChange={(event) => setAvailableSearch(event.target.value)}
              />

              <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.5 }}>
                <Stack spacing={1.25}>
                  {filteredAvailableResources.map((resource) => {
                    const key = getResourceKey(resource);
                    const isOpen = openKey === key;

                    return (
                      <ExpandableResourceRow
                        key={key}
                        resource={resource}
                        isOpen={isOpen}
                        operationSpec={operationSpecByKey.get(key)}
                        selected={selectedAvailableKeySet.has(key)}
                        onRowClick={() => toggleAvailableSelection(resource)}
                        onToggleOpen={() => setOpenKey(isOpen ? null : key)}
                      />
                    );
                  })}

                  {!filteredAvailableResources.length && (
                    <Stack
                      spacing={1}
                      alignItems="center"
                      justifyContent="center"
                      sx={{ py: 4, textAlign: 'center' }}
                    >
                      <Box
                        component="img"
                        src={NoData}
                        alt="No available resources"
                        sx={{ width: 80, maxWidth: '80%' }}
                      />
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.no.available.resources"
                          defaultMessage={'No available resources.'}
                        />
                      </Typography>
                    </Stack>
                  )}
                </Stack>
              </Box>
            </Stack>
          </Box>

          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'center',
              marginTop: 15,
            }}
          >
            <Stack spacing={1}>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={moveAllToExceptions}
                  disabled={!availableResources.length || isReadOnlyProvider}
                >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.message"
                  defaultMessage={'>>'}
                />
                </Button>
              </DisabledActionTooltip>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={moveSelectedToExceptions}
                  disabled={!selectedAvailableKeys.length || isReadOnlyProvider}
                >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.message.2"
                  defaultMessage={'>'}
                />
                </Button>
              </DisabledActionTooltip>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={moveSelectedToAvailable}
                  disabled={!selectedExceptionKeys.length || isReadOnlyProvider}
                >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.message.3"
                  defaultMessage={'<'}
                />
                </Button>
              </DisabledActionTooltip>
              <DisabledActionTooltip disabled={isReadOnlyProvider}>
                <Button
                  variant="outlined"
                  size="small"
                  onClick={moveAllToAvailable}
                  disabled={!exceptionResources.length || isReadOnlyProvider}
                >
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.message.4"
                  defaultMessage={'<<'}
                />
                </Button>
              </DisabledActionTooltip>
            </Stack>
          </Box>

          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 2,
              p: 2,
              minHeight: 360,
              maxHeight: 560,
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
          >
            <Stack spacing={1.5} sx={{ flex: 1, minHeight: 0 }}>
              <Box>
                <Typography variant="h6" sx={{ mb: 0.5, fontWeight: 600 }}>
                  {resourceMode === 'allow'
                    ? 'Denied Resources'
                    : 'Allowed Resources'}
                </Typography>
                {/* <Typography variant="caption" color="text.secondary">
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.selected.resources"
                    defaultMessage={'Selected resources'}
                  />
                </Typography> */}
              </Box>

              <TextField
                size="small"
                placeholder="Search resources"
                value={exceptionSearch}
                fullWidth
                onChange={(event) => setExceptionSearch(event.target.value)}
              />

              <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.5 }}>
                <Stack spacing={1.25}>
                  {filteredExceptionResources.map((resource) => {
                    const key = getResourceKey(resource);
                    const isOpen = openKey === key;
                    const resourceWithSummary = {
                      ...resource,
                      summary:
                        resource.summary || resourceSummaryByKey.get(key) || '',
                    };

                    return (
                      <ExpandableResourceRow
                        key={key}
                        resource={resourceWithSummary}
                        isOpen={isOpen}
                        operationSpec={operationSpecByKey.get(key)}
                        selected={selectedExceptionKeySet.has(key)}
                        onRowClick={() => toggleExceptionSelection(resource)}
                        onToggleOpen={() => setOpenKey(isOpen ? null : key)}
                      />
                    );
                  })}

                  {!filteredExceptionResources.length && (
                    <Stack
                      spacing={1}
                      alignItems="center"
                      justifyContent="center"
                      sx={{ py: 4, textAlign: 'center' }}
                    >
                      <Box
                        component="img"
                        src={NoData}
                        alt="No available selected resources"
                        sx={{ width: 80, maxWidth: '80%' }}
                      />
                      <Typography variant="body2" color="text.secondary">
                        <FormattedMessage
                          id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderResourcesTab.no.available.selected.resources"
                          defaultMessage={'No available selected resources.'}
                        />
                      </Typography>
                    </Stack>
                  )}
                </Stack>
              </Box>
            </Stack>
          </Box>
        </Box>
      </Stack>

      <Dialog
        open={resourceModeConfirmOpen}
        onClose={handleCancelResourceModeChange}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Confirm Resource Mode Change</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Change resource mode from{' '}
            {resourceMode === 'allow' ? 'Allow all' : 'Deny all'} to{' '}
            {pendingResourceMode === 'allow' ? 'Allow all' : 'Deny all'}?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleCancelResourceModeChange}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleApplyResourceModeChange}
            disabled={!pendingResourceMode || isLoading}
          >
            Apply
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={updateSpecModalOpen}
        onClose={() => {
          setUpdateSpecModalOpen(false);
          setSpecUrl(originalSpecUrlRef.current);
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Update OpenAPI Definition</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>Import Specification</FormLabel>
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{ mt: 1 }}
              >
                <TextField
                  size="small"
                  fullWidth
                  value={specUrl}
                  onChange={(e) => {
                    setSpecUrl(e.target.value);
                  }}
                  placeholder="Paste OpenAPI URL or upload a file"
                />
                <Box minWidth={152}>
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={isFetchingSpec || isReadOnlyProvider}
                    onClick={() => {
                      void applySpecificationFromUrl(specUrl);
                    }}
                  >
                    {isFetchingSpec ? 'Fetching....' : 'Fetch specification'}
                  </Button>
                </Box>
              </Stack>
            </FormControl>
            <Divider sx={{ my: 2 }}>Or</Divider>
            <Button
              variant="outlined"
              fullWidth
              onClick={handleUpdateSpecUploadClick}
              disabled={isReadOnlyProvider}
            >
              Upload Your Specification
            </Button>
            <input
              ref={updateSpecFileInputRef}
              type="file"
              hidden
              accept=".json,.yaml,.yml"
              onChange={handleUpdateSpecFileChange}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => {
              setUpdateSpecModalOpen(false);
              setSpecUrl(originalSpecUrlRef.current);
            }}
          >
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
