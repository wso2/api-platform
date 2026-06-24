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
  Box,
  Button,
  CircularProgress,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronLeft } from '@wso2/oxygen-ui-icons-react';
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
  isValidHttpUrl,
  toTokenConfig,
  TOKEN_FIELDS,
  TOKEN_LOCATIONS,
  type TokenConfig,
  type TokenFieldKey,
} from '../../../../utils/providerTemplateFields';

const MAX_DESCRIPTION_LENGTH = 200;

const VERSION_PATTERN = /^[vV]\d+\.\d+$/;

function suggestNextVersion(current?: string): string {
  const match = /^[vV](\d+)\.\d+$/.exec((current ?? '').trim());
  if (!match) return 'v2.0';
  const major = parseInt(match[1], 10);
  return `v${major + 1}.0`;
}

function toTemplateId(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
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

// True if the text parses (JSON or YAML) into a plausible OpenAPI document.
// Used to reject obviously-invalid specs on upload/fetch.
function isParseableSpec(text: string): boolean {
  if (!text.trim()) return false;
  let spec: unknown = null;
  try {
    spec = JSON.parse(text);
  } catch {
    try {
      spec = YAML.parse(text);
    } catch {
      return false;
    }
  }
  return (
    !!spec &&
    typeof spec === 'object' &&
    ('openapi' in spec || 'swagger' in spec || 'paths' in spec)
  );
}

function CreateProviderTemplateVersionForm({
  template,
}: {
  template: ProviderTemplate;
}) {
  const templateId = template.id;
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { refreshTemplates } = useProviderTemplates();
  const showSnackbar = useAIWorkspaceSnackbar();

  const overviewPath = `${buildOrgPath(
    currentOrganization,
    '/settings/llm-provider-templates'
  )}/${templateId}`;

  const currentVersion = template.version;
  const [version, setVersion] = useState(() => suggestNextVersion(template.version));

// Description is user-entered (prefilled from the source version) and optional,
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
  const [endpointUrlTouched, setEndpointUrlTouched] = useState(false);
  const [specUrlTouched, setSpecUrlTouched] = useState(false);
  const hasInheritedSpec = !specFileName && Boolean(template.openapi?.trim());
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
    if (!isValidHttpUrl(url)) {
      showSnackbar('Enter a valid http or https URL.', 'error');
      return;
    }
    setIsFetchingSpec(true);
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`Failed to fetch: ${res.statusText}`);
      const text = await res.text();
      if (!isParseableSpec(text)) {
        showSnackbar('That URL did not return a valid OpenAPI specification.', 'error');
        return;
      }
      const serverUrl = parseSpecServerUrl(text);
      setSpecFileName('');
      setSpecContent('');
      if (serverUrl) {
        setEndpointUrl(serverUrl);
        showSnackbar('Specification fetched and endpoint URL added.', 'success');
      } else {
        showSnackbar('Specification fetched. Add the endpoint URL manually.', 'info');
      }
    } catch {
      showSnackbar('Could not fetch a specification from that URL.', 'error');
    } finally {
      setIsFetchingSpec(false);
    }
  };

  const handleSpecFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      if (!isParseableSpec(text)) {
        showSnackbar('That file is not a valid OpenAPI specification (JSON or YAML).', 'error');
        return;
      }
      const serverUrl = parseSpecServerUrl(text);
      setSpecFileName(file.name);
      setSpecContent(text);
      setOpenapiSpecUrl('');
      if (serverUrl) {
        setEndpointUrl(serverUrl);
        showSnackbar('Specification uploaded and endpoint URL added.', 'success');
      } else {
        showSnackbar('Specification uploaded. Add the endpoint URL manually.', 'info');
      }
    } catch {
      showSnackbar('Could not read that specification file.', 'error');
    } finally {
      e.target.value = '';
    }
  };

  const isDescriptionValid = description.length <= MAX_DESCRIPTION_LENGTH;
  const isEndpointValid =
    endpointUrl.trim().length > 0 && isValidHttpUrl(endpointUrl);
  const specUrlEntered = openapiSpecUrl.trim().length > 0;
  const isSpecUrlValid = isValidHttpUrl(openapiSpecUrl);
  const isVersionValid = VERSION_PATTERN.test(version.trim());
  const isFormValid =
    isDescriptionValid &&
    isEndpointValid &&
    isVersionValid &&
    (!specUrlEntered || isSpecUrlValid);

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    const organizationId = currentOrganization?.uuid;
    if (!templateId || !organizationId || !isFormValid || isSubmitting) return;

    const tokenFields = fromTokenConfig(tokenConfig);

    const metadata: TemplateMetadata = { ...(template.metadata ?? {}) };
    if (endpointUrl.trim()) metadata.endpointUrl = endpointUrl.trim();
    else delete metadata.endpointUrl;
    if (openapiSpecUrl.trim()) metadata.openapiSpecUrl = openapiSpecUrl.trim();
    else delete metadata.openapiSpecUrl;

    const payload: ProviderTemplate = {
      id: toTemplateId(`${template.name ?? ''} ${version.trim()}`),
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
      const created = await providerTemplateApis.createProviderTemplateVersion(
        templateId,
        payload,
        organizationId,
        PLATFORM_API_BASE_URL
      );
      await refreshTemplates();
      showSnackbar(`New version ${version.trim()} created successfully.`, 'success');
      const newVersionPath = created.id
        ? `${buildOrgPath(currentOrganization, '/settings/llm-provider-templates')}/${created.id}`
        : overviewPath;
      navigate(newVersionPath);
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
                'Create a new version using an updated OpenAPI specification'
              }
            />
          </PageTitle.SubHeader>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
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
                  helperText={
                    version.trim().length > 0 && !isVersionValid
                      ? 'Expected: v<major>.<minor>'
                      : ''
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
                <Stack spacing={1.5} sx={{ mt: 1 }}>
                  <Stack direction="row" spacing={1.5} alignItems="center">
                    <TextField
                      size="small"
                      fullWidth
                      value={openapiSpecUrl}
                      onChange={(e) => {
                        setOpenapiSpecUrl(e.target.value);
                        setSpecUrlTouched(false);
                      }}
                      onBlur={() => setSpecUrlTouched(true)}
                      placeholder="https://api.openai.com/openapi.json"
                      error={specUrlTouched && specUrlEntered && !isSpecUrlValid}
                      helperText={
                        specUrlTouched && specUrlEntered && !isSpecUrlValid
                          ? 'Enter a valid URL.'
                          : ''
                      }
                    />
                    <Button
                      variant="outlined"
                      size="small"
                      disabled={isFetchingSpec || !openapiSpecUrl.trim() || !isSpecUrlValid}
                      onClick={() => void fetchSpecFromUrl()}
                      sx={{ whiteSpace: 'nowrap', flexShrink: 0 }}
                    >
                      {isFetchingSpec ? 'Fetching…' : 'Fetch specification'}
                    </Button>
                  </Stack>
                  <Divider>Or</Divider>
                  <Button
                    variant="outlined"
                    fullWidth
                    onClick={() => fileInputRef.current?.click()}
                  >
                    {specFileName
                      ? `Uploaded: ${specFileName}`
                      : specContent.trim()
                        ? 'Uploaded Specification'
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
                  onChange={(e) => {
                    setEndpointUrl(e.target.value);
                    setEndpointUrlTouched(false);
                  }}
                  onBlur={() => setEndpointUrlTouched(true)}
                  placeholder="https://api.openai.com"
                  error={
                    endpointUrlTouched &&
                    endpointUrl.trim().length > 0 &&
                    !isValidHttpUrl(endpointUrl)
                  }
                  helperText={
                    endpointUrlTouched &&
                    endpointUrl.trim().length > 0 &&
                    !isValidHttpUrl(endpointUrl)
                      ? 'Enter a valid URL.'
                      : ''
                  }
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
                  Token and model mapping, copied from {currentVersion}.
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
