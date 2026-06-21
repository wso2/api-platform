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

import React, { useEffect, useMemo, useState } from 'react';
import { Link as RouterLink, useParams } from 'react-router-dom';
import YAML from 'yaml';
import {
  Avatar,
  Box,
  Button,
  Card,
  CircularProgress,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  Menu,
  MenuItem,
  PageContent,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, ChevronDown, ChevronLeft, Clock, Download, Edit, GitBranch } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { buildOrgPath } from '../../../../utils/projectRouting';
import {
  DEFAULT_AUTH_CONFIG,
  fromTokenConfig,
  toTokenConfigWithDefaults,
  type TokenConfig,
  type TokenFieldKey,
} from '../../../../utils/providerTemplateFields';
import type {
  ProviderTemplate,
  ResourceMapping,
  TemplateMetadata,
  TemplateMetadataAuth,
  UpdateProviderTemplateRequest,
} from '../../../../utils/types';
import { downloadTemplateYaml } from '../../../../utils/providerTemplateManifest';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';
import TemplateTokenMapping from './TemplateTokenMapping';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0 || words[0] === '') return '??';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

type TabPanelProps = {
  value: number;
  index: number;
  children: React.ReactNode;
};

function TabPanel({ value, index, children }: TabPanelProps) {
  return (
    <Box role="tabpanel" hidden={value !== index} sx={{ pt: 2 }}>
      {value === index ? children : null}
    </Box>
  );
}

const tabs = ['Overview', 'Connection', 'Token Mapping'];

function parseOpenApiSpec(text: string): Record<string, unknown> | null {
  if (!text.trim()) return null;
  try {
    const json = JSON.parse(text);
    return json && typeof json === 'object' ? (json as Record<string, unknown>) : null;
  } catch {
    try {
      const yaml = YAML.parse(text);
      return yaml && typeof yaml === 'object' ? (yaml as Record<string, unknown>) : null;
    } catch {
      return null;
    }
  }
}

function DetailRow({ label, value }: { label: string; value?: string }) {
  return (
    <Stack direction="row" spacing={2} sx={{ py: 0.75 }}>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ minWidth: 180, flexShrink: 0 }}
      >
        {label}
      </Typography>
      <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
        {value?.trim() ? value : '—'}
      </Typography>
    </Stack>
  );
}

export default function ProviderTemplateOverview() {
  const { templateId } = useParams<{ templateId: string }>();
  const { currentOrganization } = useAppShell();
  const { updateTemplate } = useProviderTemplates();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [tabIndex, setTabIndex] = useState(0);

  const [template, setTemplate] = useState<ProviderTemplate | undefined>();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [versions, setVersions] = useState<ProviderTemplate[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<string>('');
  const [versionMenuAnchor, setVersionMenuAnchor] = useState<null | HTMLElement>(
    null
  );

  const [endpointUrl, setEndpointUrl] = useState('');
  const [openapiSpecUrl, setOpenapiSpecUrl] = useState('');
  const [logoUrlField, setLogoUrlField] = useState('');
  const [authType, setAuthType] = useState('');
  const [authHeader, setAuthHeader] = useState('');
  const [valuePrefix, setValuePrefix] = useState('');
  const [defaultTokens, setDefaultTokens] = useState<TokenConfig>(() =>
    toTokenConfigWithDefaults(undefined)
  );

  const [resourceMappings, setResourceMappings] = useState<ResourceMapping[]>([]);
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [urlSpecText, setUrlSpecText] = useState('');
  const [isSpecLoading, setIsSpecLoading] = useState(false);

  const listPath = buildOrgPath(currentOrganization, '/settings/llm-provider-templates');

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId) return;

    let isMounted = true;
    setIsLoading(true);
    setError(null);
    providerTemplateApis
      .getProviderTemplate(templateId, organizationId, PLATFORM_API_BASE_URL)
      .then((full) => {
        if (isMounted) setTemplate(full);
      })
      .catch((err: unknown) => {
        if (isMounted) {
          setError(err instanceof Error ? err : new Error('Failed to load template'));
        }
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [templateId, currentOrganization?.uuid]);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId) return;

    let isMounted = true;
    providerTemplateApis
      .getProviderTemplateVersions(templateId, organizationId, PLATFORM_API_BASE_URL)
      .then((list) => {
        if (isMounted && list.length) setVersions(list);
      })
      .catch(() => {
        /* switcher gracefully degrades to the single current version */
      });

    return () => {
      isMounted = false;
    };
  }, [templateId, currentOrganization?.uuid]);

  useEffect(() => {
    if (template?.version) setSelectedVersion(template.version);
  }, [template?.version]);

  const handleSwitchVersion = async (version: string) => {
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId || version === selectedVersion) return;
    setSelectedVersion(version);
    setIsLoading(true);
    try {
      const isLatest = versions.find((v) => v.version === version)?.isLatest;
      const full = isLatest
        ? await providerTemplateApis.getProviderTemplate(
            templateId,
            organizationId,
            PLATFORM_API_BASE_URL
          )
        : await providerTemplateApis.getProviderTemplateVersion(
            templateId,
            version,
            organizationId,
            PLATFORM_API_BASE_URL
          );
      setTemplate(full);
    } catch (err) {
      showSnackbar(
        err instanceof Error ? err.message : 'Failed to load version.',
        'error'
      );
    } finally {
      setIsLoading(false);
    }
  };

  // Seed (and reset) the editable drafts whenever the loaded template changes.
  const seedDrafts = React.useCallback((t: ProviderTemplate) => {
    setEndpointUrl(t.metadata?.endpointUrl ?? '');
    setOpenapiSpecUrl(t.metadata?.openapiSpecUrl ?? '');
    setLogoUrlField(t.metadata?.logoUrl ?? '');
    setAuthType(t.metadata?.auth?.type ?? DEFAULT_AUTH_CONFIG.type);
    setAuthHeader(t.metadata?.auth?.header ?? DEFAULT_AUTH_CONFIG.header);
    setValuePrefix(t.metadata?.auth?.valuePrefix ?? DEFAULT_AUTH_CONFIG.valuePrefix);
    setDefaultTokens(toTokenConfigWithDefaults(t));
    setResourceMappings(t.resourceMappings?.resources ?? []);
    setIsDirty(false);
  }, []);

  useEffect(() => {
    if (template) seedDrafts(template);
  }, [template, seedDrafts]);

  useEffect(() => {
    const inline = template?.openapi?.trim();
    const specUrl = template?.metadata?.openapiSpecUrl?.trim();
    if (inline || !specUrl) {
      setUrlSpecText('');
      setIsSpecLoading(false);
      return;
    }
    let isMounted = true;
    setIsSpecLoading(true);
    fetch(specUrl)
      .then((res) => (res.ok ? res.text() : Promise.reject(new Error(`${res.status}`))))
      .then((text) => {
        if (isMounted) setUrlSpecText(text);
      })
      .catch(() => {
        if (isMounted) setUrlSpecText('');
      })
      .finally(() => {
        if (isMounted) setIsSpecLoading(false);
      });
    return () => {
      isMounted = false;
    };
  }, [template?.openapi, template?.metadata?.openapiSpecUrl]);

  const updateToken = (
    field: TokenFieldKey,
    key: 'identifier' | 'location',
    value: string
  ) => {
    setDefaultTokens((prev) => ({
      ...prev,
      [field]: { ...prev[field], [key]: value },
    }));
    setIsDirty(true);
  };

  const handleSaveChanges = async () => {
    if (!template?.id || isSaving) return;
    if (!endpointUrl.trim()) {
      showSnackbar('Endpoint URL is required.', 'error');
      return;
    }

    const metadata: TemplateMetadata = {};
    if (endpointUrl.trim()) metadata.endpointUrl = endpointUrl.trim();
    if (logoUrlField.trim()) metadata.logoUrl = logoUrlField.trim();
    if (openapiSpecUrl.trim()) metadata.openapiSpecUrl = openapiSpecUrl.trim();
    const authObj: TemplateMetadataAuth = {} as TemplateMetadataAuth;
    if (authType.trim()) authObj.type = authType.trim();
    if (authHeader.trim()) authObj.header = authHeader.trim();
    if (valuePrefix.trim()) authObj.valuePrefix = valuePrefix.trim();
    if (Object.keys(authObj).length) metadata.auth = authObj;

    const payload: UpdateProviderTemplateRequest = {
      id: template.id,
      name: template.name,
      description: template.description,
      ...fromTokenConfig(defaultTokens),
      metadata: Object.keys(metadata).length ? metadata : undefined,
      resourceMappings: resourceMappings.length
        ? { resources: resourceMappings }
        : undefined,
      openapi: template.openapi,
    };

    setIsSaving(true);
    try {
      const updated = await updateTemplate(template.id, payload);
      setTemplate(updated);
      setIsDirty(false);
      showSnackbar('Template updated successfully.', 'success');
    } catch (err) {
      showSnackbar(
        err instanceof Error ? err.message : 'Failed to update template.',
        'error'
      );
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancelChanges = () => {
    if (template) seedDrafts(template);
  };

  const parsedSpec = useMemo(
    () =>
      parseOpenApiSpec(
        template?.openapi?.trim() ? template.openapi : urlSpecText
      ),
    [template?.openapi, urlSpecText]
  );

  const backButton = (
    <Button
      component={RouterLink}
      to={listPath}
      size="small"
      startIcon={<ChevronLeft size={24} />}
    >
      <FormattedMessage
        id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplateOverview.back"
        defaultMessage={'Back to list'}
      />
    </Button>
  );

  if (isLoading) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 300,
          }}
        >
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth>
        {backButton}
        <Stack spacing={1} sx={{ mt: 2 }}>
          <Typography variant="h6" color="error">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplateOverview.error"
              defaultMessage={'Error loading template'}
            />
          </Typography>
          <Typography variant="body2">{error.message}</Typography>
        </Stack>
      </PageContent>
    );
  }

  if (!template) {
    return (
      <PageContent fullWidth>
        {backButton}
        <Typography variant="h6" sx={{ mt: 2 }}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplateOverview.notFound"
            defaultMessage={'Template not found'}
          />
        </Typography>
      </PageContent>
    );
  }

  const metadata = template.metadata;
  const logoUrl = metadata?.logoUrl?.trim();
  const hasLogo = Boolean(logoUrl);
  const description = template.description?.trim() || 'No description';
  const createdTime = template.createdAt ?? template.updatedAt;

  return (
    <PageContent fullWidth>
      {backButton}

      <Stack spacing={3} sx={{ mt: 2 }}>
        {/* Header card */}
        <Card>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 2,
              padding: 2,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
              <Avatar
                src={hasLogo ? logoUrl : undefined}
                sx={{
                  width: 72,
                  height: 72,
                  fontWeight: 600,
                  fontSize: 28,
                  bgcolor: hasLogo ? 'common.white' : 'primary.light',
                  color: hasLogo ? 'text.primary' : 'primary.contrastText',
                  border: hasLogo ? '1px solid' : 'none',
                  borderColor: 'divider',
                  p: hasLogo ? 0.5 : 0,
                  '& img': { objectFit: 'contain' },
                }}
              >
                {!hasLogo ? getInitials(template.name) : null}
              </Avatar>
              <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
                  <Typography variant="h3">{template.name}</Typography>
                  <Tooltip title="Edit template">
                    <IconButton component={RouterLink} to="edit" size="small">
                      <Edit size={16} />
                    </IconButton>
                  </Tooltip>
                  {/* Version switcher — a pill that opens the versions menu. */}
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={(e) => setVersionMenuAnchor(e.currentTarget)}
                    endIcon={<ChevronDown size={16} />}
                    sx={{ borderRadius: 5, px: 1.5 }}
                  >
                    {selectedVersion || template.version || 'v1'}
                  </Button>
                  <Menu
                    anchorEl={versionMenuAnchor}
                    open={Boolean(versionMenuAnchor)}
                    onClose={() => setVersionMenuAnchor(null)}
                    slotProps={{ paper: { sx: { minWidth: 260 } } }}
                  >
                    <Typography
                      variant="overline"
                      sx={{
                        px: 2,
                        py: 0.5,
                        display: 'block',
                        color: 'text.secondary',
                      }}
                    >
                      Versions
                    </Typography>
                    {(versions.length ? versions : [template]).map((v) => {
                      const ver = v.version || 'v1';
                      const isSelected =
                        ver === (selectedVersion || template.version || 'v1');
                      return (
                        <MenuItem
                          key={ver}
                          selected={isSelected}
                          onClick={() => {
                            setVersionMenuAnchor(null);
                            void handleSwitchVersion(ver);
                          }}
                        >
                          <Stack sx={{ flexGrow: 1 }}>
                            <Typography variant="body2" sx={{ fontWeight: 600 }}>
                              {ver}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              {formatRelativeTime(v.createdAt ?? v.updatedAt)}
                            </Typography>
                          </Stack>
                          {isSelected ? <Check size={16} /> : null}
                        </MenuItem>
                      );
                    })}
                    <Divider />
                    <MenuItem
                      component={RouterLink}
                      to="new-version"
                      onClick={() => setVersionMenuAnchor(null)}
                      sx={{ color: 'primary.main', gap: 1 }}
                    >
                      <GitBranch size={16} />
                      Create new version
                    </MenuItem>
                  </Menu>
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {description}
                </Typography>
                <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap">
                  {template.createdBy ? (
                    <Typography variant="caption" color="text.secondary">
                      <FormattedMessage
                        id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplateOverview.createdBy"
                        defaultMessage={'Created by {user}'}
                        values={{ user: template.createdBy }}
                      />
                    </Typography>
                  ) : null}
                  <Stack direction="row" spacing={0.5} alignItems="center">
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary">
                      {formatRelativeTime(createdTime)}
                    </Typography>
                  </Stack>
                </Stack>
              </Stack>
            </Box>

            <Button
              variant="contained"
              onClick={() => {
                const name = downloadTemplateYaml(template);
                showSnackbar(`${name} downloaded`, 'success');
              }}
              startIcon={<Download size={16} />}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.ProviderTemplateOverview.downloadYaml"
                defaultMessage={'Download YAML'}
              />
            </Button>
          </Box>
        </Card>

        {/* Tabbed card */}
        <Card>
          <Tabs
            value={tabIndex}
            onChange={(_, value) => setTabIndex(value)}
            variant="scrollable"
            allowScrollButtonsMobile
          >
            {tabs.map((label) => (
              <Tab key={label} label={label} />
            ))}
          </Tabs>
          <Divider />
          <Box padding={2}>
            {/* Overview */}
            <TabPanel value={tabIndex} index={0}>
              <Stack divider={<Divider flexItem />} spacing={0.5}>
                <DetailRow label="Template ID" value={template.id} />
                <DetailRow
                  label="Version"
                  value={selectedVersion || template.version || 'v1'}
                />
                <DetailRow label="Endpoint URL" value={metadata?.endpointUrl} />
                <DetailRow
                  label="Total Versions"
                  value={String(versions.length || 1)}
                />
              </Stack>

              <Box sx={{ mt: 3 }}>
                <Typography variant="h6" sx={{ mb: 1, fontWeight: 600 }}>
                  OpenAPI Resources
                </Typography>
                {!template.openapi?.trim() &&
                !template.metadata?.openapiSpecUrl?.trim() ? (
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                    No available resources. Add an OpenAPI specification (content
                    or URL) on the Connection tab to see resources.
                  </Typography>
                ) : isSpecLoading ? (
                  <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
                    <CircularProgress size={24} />
                  </Box>
                ) : !parsedSpec ? (
                  <Typography variant="body2" color="error" sx={{ py: 2 }}>
                    Failed to load the OpenAPI specification
                    {template.metadata?.openapiSpecUrl?.trim()
                      ? ' from the configured URL.'
                      : '.'}
                  </Typography>
                ) : (
                  <Box
                    sx={{
                      maxHeight: 350,
                      overflowY: 'auto',
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      bgcolor: 'background.paper',
                      px: 2,
                      pt: 2,
                    }}
                  >
                    <SwaggerSpecViewer
                      spec={parsedSpec}
                      disableTryOutBtn
                      disableNetworkExecution
                      hideInfoSection
                      hideServers
                      hideAuthorizeButton
                      hideTagHeaders
                      docExpansion="list"
                      defaultModelsExpandDepth={-1}
                    />
                  </Box>
                )}
              </Box>
            </TabPanel>

            <TabPanel value={tabIndex} index={1}>
              <Grid container spacing={2}>
                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel required>Endpoint URL</FormLabel>
                    <TextField
                      fullWidth
                      required
                      value={endpointUrl}
                      onChange={(e) => {
                        setEndpointUrl(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="https://api.openai.com"
                      error={!endpointUrl.trim()}
                      helperText={
                        !endpointUrl.trim() ? 'Endpoint URL is required.' : ''
                      }
                    />
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel>OpenAPI Spec URL</FormLabel>
                    <TextField
                      fullWidth
                      value={openapiSpecUrl}
                      onChange={(e) => {
                        setOpenapiSpecUrl(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="https://api.openai.com/openapi.json"
                    />
                    {template.openapi?.trim() ? (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ mt: 0.5 }}
                      >
                        An OpenAPI spec is stored inline (uploaded) —{' '}
                        {(template.openapi.length / 1024).toFixed(1)} KB. It powers
                        the resources above. Setting a URL here references a spec by
                        link instead.
                      </Typography>
                    ) : null}
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel>Logo URL</FormLabel>
                    <TextField
                      fullWidth
                      value={logoUrlField}
                      onChange={(e) => {
                        setLogoUrlField(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="https://cdn.example.com/logos/openai.svg"
                    />
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <FormControl fullWidth>
                    <FormLabel>Auth Type</FormLabel>
                    <TextField
                      fullWidth
                      value={authType}
                      onChange={(e) => {
                        setAuthType(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="bearer"
                    />
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <FormControl fullWidth>
                    <FormLabel>Auth Header</FormLabel>
                    <TextField
                      fullWidth
                      value={authHeader}
                      onChange={(e) => {
                        setAuthHeader(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="Authorization"
                    />
                  </FormControl>
                </Grid>
                <Grid size={{ xs: 12, sm: 4 }}>
                  <FormControl fullWidth>
                    <FormLabel>Value Prefix</FormLabel>
                    <TextField
                      fullWidth
                      value={valuePrefix}
                      onChange={(e) => {
                        setValuePrefix(e.target.value);
                        setIsDirty(true);
                      }}
                      placeholder="Bearer "
                    />
                  </FormControl>
                </Grid>
              </Grid>
            </TabPanel>

            <TabPanel value={tabIndex} index={2}>
              <TemplateTokenMapping
                defaultTokens={defaultTokens}
                onChangeDefaultToken={updateToken}
                resourceMappings={resourceMappings}
                onChangeResourceMappings={(next) => {
                  setResourceMappings(next);
                  setIsDirty(true);
                }}
                spec={parsedSpec}
              />
            </TabPanel>
          </Box>
        </Card>

        <Box sx={{ position: 'sticky', bottom: 0, zIndex: 10 }}>
          <Card>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              spacing={1}
              alignItems={{ xs: 'flex-start', sm: 'center' }}
              justifyContent="space-between"
              sx={{ p: 2 }}
            >
              <Typography
                variant="body2"
                color={isDirty ? 'warning.main' : 'text.secondary'}
              >
                {isDirty ? 'You have unsaved changes.' : ''}
              </Typography>
              <Stack direction="row" spacing={1}>
                <Button
                  variant="outlined"
                  color="secondary"
                  disabled={!isDirty || isSaving}
                  onClick={handleCancelChanges}
                >
                  Cancel
                </Button>
                <Button
                  variant="contained"
                  disabled={!isDirty || isSaving || !endpointUrl.trim()}
                  onClick={() => void handleSaveChanges()}
                >
                  {isSaving ? 'Updating...' : 'Update'}
                </Button>
              </Stack>
            </Stack>
          </Card>
        </Box>
      </Stack>
    </PageContent>
  );
}
