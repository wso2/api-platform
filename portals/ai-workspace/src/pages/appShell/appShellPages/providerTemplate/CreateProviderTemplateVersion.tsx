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

import React, { useRef, useState } from 'react';
import { useNavigate, useParams, Link as RouterLink } from 'react-router-dom';
import YAML from 'yaml';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  CircularProgress,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  InputAdornment,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronLeft,
  GitBranch,
  Tag,
} from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import * as providerTemplateApis from '../../../../apis/providerTemplateApis';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { buildOrgPath } from '../../../../utils/projectRouting';
import type {
  ProviderTemplate,
  TemplateMetadata,
} from '../../../../utils/types';
import {
  fromTokenConfig,
  toTokenConfig,
  TOKEN_FIELDS,
  TOKEN_LOCATIONS,
  type TokenConfig,
  type TokenFieldKey,
} from '../../../../utils/providerTemplateFields';

const MAX_DESCRIPTION_LENGTH = 200;

// Versions follow the v<major>.<minor> pattern (e.g. v1.0).
const VERSION_PATTERN = /^[vV]\d+\.\d+$/;

// Suggest the next version by bumping the major of the current version
// (v1.0 -> v2.0). Falls back to v2.0 when the current can't be parsed. The
// suggestion is only a default — the field is editable.
function suggestNextVersion(current?: string): string {
  const match = /^[vV](\d+)\.\d+$/.exec((current ?? '').trim());
  if (!match) return 'v2.0';
  const major = parseInt(match[1], 10);
  return `v${major + 1}.0`;
}

// Parse an OpenAPI spec (JSON or YAML) and return its first server URL.
function parseSpecServerUrl(text: string): string | null {
  if (!text.trim()) return null;
  let spec: { servers?: Array<{ url?: string }> } | null = null;
  try {
    spec = JSON.parse(text);
  } catch {
    try {
      spec = YAML.parse(text) as { servers?: Array<{ url?: string }> };
    } catch {
      return null;
    }
  }
  const url = spec?.servers?.[0]?.url;
  return typeof url === 'string' && url.trim() ? url.trim() : null;
}

function CreateProviderTemplateVersionForm({
  template,
}: {
  template: ProviderTemplate;
}) {
  const templateId = template.id ?? '';
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { refreshTemplates } = useProviderTemplates();
  const showSnackbar = useAIWorkspaceSnackbar();

  const overviewPath = `${buildOrgPath(
    currentOrganization,
    '/settings/llm-provider-templates'
  )}/${templateId}`;

  // The current latest version; the new version is user-entered (prefilled with
  // a suggested bump) and must be unique.
  const currentVersion = template.version ?? 'v1.0';
  const [version, setVersion] = useState(() => suggestNextVersion(template.version));

  // Description and the OpenAPI spec are the things a new version typically
  // changes; pre-fill from the source version.
  const [description, setDescription] = useState(template.description ?? '');
  const [openapiSpecUrl, setOpenapiSpecUrl] = useState(
    template.metadata?.openapiSpecUrl ?? ''
  );
  const [endpointUrl, setEndpointUrl] = useState(
    template.metadata?.endpointUrl ?? ''
  );
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [specFileName, setSpecFileName] = useState('');
  const [specContent, setSpecContent] = useState(template.openapi ?? '');
  // Token & model mappings copied from the source version (adjust as needed).
  const [tokenConfig, setTokenConfig] = useState<TokenConfig>(() =>
    toTokenConfig(template)
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const updateToken = (
    field: TokenFieldKey,
    key: 'identifier' | 'location',
    value: string
  ) => {
    setTokenConfig((prev) => ({
      ...prev,
      [field]: { ...prev[field], [key]: value },
    }));
  };

  const fetchSpecFromUrl = async () => {
    const url = openapiSpecUrl.trim();
    if (!url) return;
    setIsFetchingSpec(true);
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`Failed to fetch: ${res.statusText}`);
      const text = await res.text();
      const serverUrl = parseSpecServerUrl(text);
      setSpecFileName('');
      setSpecContent('');
      if (serverUrl) {
        setEndpointUrl(serverUrl);
        showSnackbar('Specification fetched. Endpoint URL filled from servers.', 'success');
      } else {
        showSnackbar('Fetched the spec, but no server URL was found — enter the endpoint manually.', 'info');
      }
    } catch {
      showSnackbar('Failed to fetch specification from that URL.', 'error');
    } finally {
      setIsFetchingSpec(false);
    }
  };

  const handleSpecFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const serverUrl = parseSpecServerUrl(text);
      setSpecFileName(file.name);
      setSpecContent(text);
      setOpenapiSpecUrl('');
      if (serverUrl) {
        setEndpointUrl(serverUrl);
        showSnackbar('Specification uploaded. Endpoint URL filled from servers.', 'success');
      } else {
        showSnackbar('Read the spec, but no server URL was found — enter the endpoint manually.', 'info');
      }
    } catch {
      showSnackbar('Failed to read the specification file.', 'error');
    } finally {
      e.target.value = '';
    }
  };

  const isDescriptionValid = description.length <= MAX_DESCRIPTION_LENGTH;
  const isEndpointValid = endpointUrl.trim().length > 0;
  const isVersionValid = VERSION_PATTERN.test(version.trim());
  const isFormValid = isDescriptionValid && isEndpointValid && isVersionValid;

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId || !isFormValid || isSubmitting) return;

    const tokenFields = fromTokenConfig(tokenConfig);

    // Preserve connection metadata (endpoint + spec URL); the new version keeps
    // the source's auth/logo and updates endpoint/spec from this form.
    const metadata: TemplateMetadata = { ...(template.metadata ?? {}) };
    if (endpointUrl.trim()) metadata.endpointUrl = endpointUrl.trim();
    else delete metadata.endpointUrl;
    if (openapiSpecUrl.trim()) metadata.openapiSpecUrl = openapiSpecUrl.trim();
    else delete metadata.openapiSpecUrl;

    // POST /{id}/versions creates a NEW version with the supplied version
    // string. Carry forward resource mappings; override the fields edited here.
    const payload: ProviderTemplate = {
      id: templateId,
      name: template.name,
      version: version.trim(),
      description: description.trim() || undefined,
      resourceMappings: template.resourceMappings,
      ...tokenFields,
      metadata: Object.keys(metadata).length ? metadata : undefined,
      openapi: specContent.trim() ? specContent : undefined,
    };

    setIsSubmitting(true);
    try {
      await providerTemplateApis.createProviderTemplateVersion(
        templateId,
        payload,
        organizationId,
        PLATFORM_API_BASE_URL
      );
      await refreshTemplates();
      showSnackbar(`New version ${version.trim()} created successfully.`, 'success');
      navigate(overviewPath);
    } catch (err) {
      showSnackbar(
        err instanceof Error ? err.message : 'Failed to create new version.',
        'error'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={overviewPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.back"
          defaultMessage={'Back to list'}
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.title"
              defaultMessage={'Create New Version'}
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.subtitle"
              defaultMessage={
                'Spin a new version with an updated OpenAPI spec. Token & resource mappings are copied from the source version — adjust as needed.'
              }
            />
          </PageTitle.SubHeader>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Alert severity="info" icon={<GitBranch size={18} />} sx={{ mb: 2 }}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.info"
            defaultMessage={
              'The new version is added to {name} and becomes the latest. Older versions stay available in the version switcher and listing.'
            }
            values={{ name: <strong>{template.name}</strong> }}
          />
        </Alert>

        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Grid container spacing={2}>
            {/* Name is fixed — a version belongs to the same template. */}
            <Grid size={{ xs: 12, sm: 8 }}>
              <FormControl fullWidth>
                <FormLabel required>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.name"
                    defaultMessage={'Name'}
                  />
                </FormLabel>
                <TextField fullWidth value={template.name} disabled />
              </FormControl>
            </Grid>

            {/* Version is user-entered (prefilled with a suggested bump) and
                must be unique, matching the v<major>.<minor> pattern. */}
            <Grid size={{ xs: 12, sm: 4 }}>
              <FormControl fullWidth>
                <FormLabel required>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.version"
                    defaultMessage={'Version'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  placeholder="v2.0"
                  error={version.trim().length > 0 && !isVersionValid}
                  slotProps={{
                    input: {
                      startAdornment: (
                        <InputAdornment position="start">
                          <Tag size={16} />
                        </InputAdornment>
                      ),
                    },
                  }}
                  helperText={
                    version.trim().length > 0 && !isVersionValid
                      ? 'Use the v<major>.<minor> format, e.g. v2.0'
                      : `Latest is ${currentVersion}`
                  }
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.description"
                    defaultMessage={'Description'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Enter description"
                  multiline
                  minRows={3}
                  error={description.length > MAX_DESCRIPTION_LENGTH}
                  helperText={
                    description.length > MAX_DESCRIPTION_LENGTH
                      ? `Description must not exceed ${MAX_DESCRIPTION_LENGTH} characters (${description.length}/${MAX_DESCRIPTION_LENGTH})`
                      : ''
                  }
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.importSpec"
                    defaultMessage={'OpenAPI Specification'}
                  />
                </FormLabel>
                <Stack
                  direction="row"
                  spacing={1.5}
                  alignItems="center"
                  sx={{ mt: 1 }}
                >
                  <TextField
                    size="small"
                    fullWidth
                    value={openapiSpecUrl}
                    onChange={(e) => {
                      setOpenapiSpecUrl(e.target.value);
                      setSpecFileName('');
                      setSpecContent('');
                    }}
                    placeholder="https://api.openai.com/openapi.json"
                  />
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={isFetchingSpec || !openapiSpecUrl.trim()}
                    onClick={() => void fetchSpecFromUrl()}
                    sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
                  >
                    {isFetchingSpec ? 'Fetching…' : 'Fetch specification'}
                  </Button>
                  <Divider orientation="vertical" flexItem>
                    Or
                  </Divider>
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={() => fileInputRef.current?.click()}
                    sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
                  >
                    {specFileName
                      ? `Uploaded: ${specFileName}`
                      : 'Upload Your Specification'}
                  </Button>
                </Stack>
                <input
                  ref={fileInputRef}
                  type="file"
                  hidden
                  accept=".json,.yaml,.yml"
                  onChange={handleSpecFileChange}
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel required>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.endpointUrl"
                    defaultMessage={'Endpoint URL'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  required
                  value={endpointUrl}
                  onChange={(e) => setEndpointUrl(e.target.value)}
                  placeholder="https://api.openai.com"
                />
              </FormControl>
            </Grid>
          </Grid>

          {/* Advanced: token & model extraction mapping copied from the source
              version (collapsed by default). */}
          <Accordion
            disableGutters
            sx={{ mt: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1, '&:before': { display: 'none' } }}
          >
            <AccordionSummary expandIcon={<ChevronDown size={18} />}>
              <Box>
                <Typography variant="subtitle2">Advanced</Typography>
                <Typography variant="caption" color="text.secondary">
                  Token &amp; model mapping — copied from {currentVersion}; change
                  if this version differs.
                </Typography>
              </Box>
            </AccordionSummary>
            <AccordionDetails>
              <Grid container spacing={2}>
                {TOKEN_FIELDS.map(({ key, label }) => (
                  <React.Fragment key={key}>
                    <Grid size={{ xs: 12, sm: 8 }}>
                      <FormControl fullWidth>
                        <FormLabel>{`${label} Identifier`}</FormLabel>
                        <TextField
                          fullWidth
                          value={tokenConfig[key].identifier}
                          onChange={(e) =>
                            updateToken(key, 'identifier', e.target.value)
                          }
                        />
                      </FormControl>
                    </Grid>
                    <Grid size={{ xs: 12, sm: 4 }}>
                      <FormControl fullWidth>
                        <FormLabel>{`${label} Location`}</FormLabel>
                        <Select
                          value={tokenConfig[key].location}
                          onChange={(e) =>
                            updateToken(key, 'location', e.target.value as string)
                          }
                        >
                          {TOKEN_LOCATIONS.map((option) => (
                            <MenuItem key={option.value} value={option.value}>
                              {option.label}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    </Grid>
                  </React.Fragment>
                ))}
              </Grid>
            </AccordionDetails>
          </Accordion>

          <Box sx={{ mt: 3, display: 'flex', justifyContent: 'flex-start', gap: 1 }}>
            <Button
              variant="outlined"
              component={RouterLink}
              to={overviewPath}
              color="secondary"
              type="button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.cancel"
                defaultMessage={'Cancel'}
              />
            </Button>
            <Button
              variant="contained"
              type="submit"
              disabled={isSubmitting || !isFormValid}
              data-cyid="create-provider-template-version-submit"
            >
              {isSubmitting ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.creating"
                  defaultMessage={'Creating...'}
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.create"
                  defaultMessage={'Create Version'}
                />
              )}
            </Button>
          </Box>
        </Box>
      </Box>
    </PageContent>
  );
}

// Container: fetch the full (latest) template so token/resource mappings and
// metadata are available to copy into the new version.
export default function CreateProviderTemplateVersion() {
  const { templateId } = useParams<{ templateId: string }>();
  const { currentOrganization } = useAppShell();
  const [template, setTemplate] = useState<ProviderTemplate | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const listPath = buildOrgPath(currentOrganization, '/settings');

  React.useEffect(() => {
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
          setError(err instanceof Error ? err.message : 'Failed to load template');
        }
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [templateId, currentOrganization?.uuid]);

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

  if (error || !template) {
    return (
      <PageContent fullWidth>
        <Button
          component={RouterLink}
          to={listPath}
          size="small"
          startIcon={<ChevronLeft size={24} />}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.back"
            defaultMessage={'Back to list'}
          />
        </Button>
        <Typography variant="h6" sx={{ mt: 2 }}>
          {error ? (
            error
          ) : (
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplateVersion.notFound"
              defaultMessage={'Template not found'}
            />
          )}
        </Typography>
      </PageContent>
    );
  }

  return (
    <CreateProviderTemplateVersionForm key={template.id} template={template} />
  );
}
