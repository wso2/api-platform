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
import { useNavigate, Link as RouterLink } from 'react-router-dom';
import YAML from 'yaml';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
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
import { ChevronDown, ChevronLeft, Tag } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useProviderTemplates } from '../../../../contexts/llmProvider/providerTemplate';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import type {
  CreateProviderTemplateRequest,
  TemplateMetadata,
} from '../../../../utils/types';
import {
  DEFAULT_AUTH_CONFIG,
  DEFAULT_TOKEN_CONFIG,
  fromTokenConfig,
  isValidHttpUrl,
  TOKEN_FIELDS,
  TOKEN_LOCATIONS,
  type TokenConfig,
  type TokenFieldKey,
} from '../../../../utils/providerTemplateFields';

const MAX_NAME_LENGTH = 80;
const MAX_DESCRIPTION_LENGTH = 200;
const INITIAL_VERSION = 'v1.0';

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

export default function CreateProviderTemplate() {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { createTemplate } = useProviderTemplates();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [endpointUrl, setEndpointUrl] = useState('');
  const [openapiSpecUrl, setOpenapiSpecUrl] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [specFileName, setSpecFileName] = useState('');
  const [specContent, setSpecContent] = useState('');
  // Whether the entered spec URL has been fetched (and validated). Required
  // before create so a URL can't be saved without confirming it resolves.
  const [specFetched, setSpecFetched] = useState(false);

  const [tokenConfig, setTokenConfig] = useState<TokenConfig>(() => ({
    ...DEFAULT_TOKEN_CONFIG,
  }));
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

  const listPath = buildOrgPath(currentOrganization, '/settings');

  const fetchSpecFromUrl = async () => {
    const url = openapiSpecUrl.trim();
    if (!url) return;
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
      setSpecFetched(true);
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
      if (!isParseableSpec(text)) {
        showSnackbar('That file is not a valid OpenAPI specification (JSON or YAML).', 'error');
        return;
      }
      const serverUrl = parseSpecServerUrl(text);
      setSpecFileName(file.name);
      setSpecContent(text);
      setOpenapiSpecUrl('');
      setSpecFetched(true);
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

  const toTemplateId = (value: string): string =>
    value
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '');

  const isNameValid = name.trim().length > 0 && name.length <= MAX_NAME_LENGTH;
  const isDescriptionValid = description.length <= MAX_DESCRIPTION_LENGTH;
  const isEndpointValid =
    endpointUrl.trim().length > 0 && isValidHttpUrl(endpointUrl);
  const specUrlEntered = openapiSpecUrl.trim().length > 0;
  const isSpecUrlValid = isValidHttpUrl(openapiSpecUrl);
  // A spec URL, if provided, must be a valid URL AND fetched before creating.
  const isSpecReady = !specUrlEntered || (isSpecUrlValid && specFetched);
  const isFormValid =
    isNameValid && isDescriptionValid && isEndpointValid && isSpecReady;

  const handleSubmit = async (event?: React.FormEvent) => {
    if (event) event.preventDefault();
    if (!isFormValid || isSubmitting) return;

    const tokenFields = fromTokenConfig(tokenConfig);
    const metadata: TemplateMetadata = {};

    if (endpointUrl.trim()) metadata.endpointUrl = endpointUrl.trim();
    if (openapiSpecUrl.trim()) metadata.openapiSpecUrl = openapiSpecUrl.trim();

    metadata.auth = { ...DEFAULT_AUTH_CONFIG };

    const payload: CreateProviderTemplateRequest = {
      id: toTemplateId(name),
      name: name.trim(),
      description: description.trim() || undefined,
      ...tokenFields,
      metadata: Object.keys(metadata).length ? metadata : undefined,
      // Uploaded spec content (empty when a spec URL is used instead).
      openapi: specContent.trim() ? specContent : undefined,
    };

    setIsSubmitting(true);
    try {
      await createTemplate(payload);
      showSnackbar('Template created successfully.', 'success');
      navigate(listPath); // Go back to the list, where the new template appears.
    } catch (err: any) {
      showSnackbar(err?.message || 'Failed to create template.', 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={listPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.back"
          defaultMessage={'Back to list'}
        />
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.title"
              defaultMessage={'Add LLM Provider Template'}
            />
          </PageTitle.Header>
        </PageTitle>
      </Stack>

      {/* component="form" makes Enter submit and groups inputs semantically. */}
      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Box component="form" onSubmit={handleSubmit} noValidate>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 8 }}>
              <FormControl fullWidth>
                <FormLabel required>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.name"
                    defaultMessage={'Name'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Enter template name"
                  error={name.length > MAX_NAME_LENGTH}
                  helperText={
                    name.length > MAX_NAME_LENGTH
                      ? `Name must not exceed ${MAX_NAME_LENGTH} characters (${name.length}/${MAX_NAME_LENGTH})`
                      : ''
                  }
                />
              </FormControl>
            </Grid>

            {/* A new template always starts at v1 and read-only*/}
            <Grid size={{ xs: 12, sm: 4 }}>
              <FormControl fullWidth>
                <FormLabel required>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.version"
                    defaultMessage={'Version'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  value={INITIAL_VERSION}
                  disabled
                  slotProps={{
                    input: {
                      startAdornment: (
                        <InputAdornment position="start">
                          <Tag size={16} />
                        </InputAdornment>
                      ),
                    },
                  }}
                  helperText="Initial version"
                />
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12 }}>
              <FormControl fullWidth>
                <FormLabel>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.description"
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
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.importSpec"
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
                      setSpecFetched(false);
                    }}
                    placeholder="https://api.openai.com/openapi.json"
                    error={specUrlEntered && (!isSpecUrlValid || !specFetched)}
                  />
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={
                      isFetchingSpec || !openapiSpecUrl.trim() || !isSpecUrlValid
                    }
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
                {/* Reserved fixed-height line so the message doesn't shift the
                    row above it. Shown in red since fetching is mandatory. */}
                <Box sx={{ minHeight: 20, mt: 0.5 }}>
                  {specUrlEntered && !isSpecUrlValid ? (
                    <Typography variant="caption" color="error">
                      Enter a valid URL.
                    </Typography>
                  ) : specUrlEntered && !specFetched ? (
                    <Typography variant="caption" color="error">
                      Click &apos;Fetch specification&apos; to validate the URL
                      before creating.
                    </Typography>
                  ) : null}
                </Box>
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
                    id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.endpointUrl"
                    defaultMessage={'Endpoint URL'}
                  />
                </FormLabel>
                <TextField
                  fullWidth
                  required
                  value={endpointUrl}
                  onChange={(e) => setEndpointUrl(e.target.value)}
                  placeholder="https://api.openai.com"
                  error={endpointUrl.trim().length > 0 && !isValidHttpUrl(endpointUrl)}
                  helperText={
                    endpointUrl.trim().length > 0 && !isValidHttpUrl(endpointUrl)
                      ? 'Enter a valid http(s) URL.'
                      : ''
                  }
                />
              </FormControl>
            </Grid>

          </Grid>

          {/* Advanced: token & model extraction mapping (collapsed by default,
              pre-filled with the OpenAI defaults). */}
          <Accordion
            disableGutters
            sx={{ mt: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1, '&:before': { display: 'none' } }}
          >
            <AccordionSummary expandIcon={<ChevronDown size={18} />}>
              <Box>
                <Typography variant="subtitle2">Advanced</Typography>
                <Typography variant="caption" color="text.secondary">
                  Token &amp; model mapping — defaults to OpenAI values; change if
                  your provider differs.
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
              to={listPath}
              color="secondary"
              type="button"
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.cancel"
                defaultMessage={'Cancel'}
              />
            </Button>
            <Button
              variant="contained"
              type="submit"
              disabled={isSubmitting || !isFormValid}
              data-cyid="create-provider-template-submit"
            >
              {isSubmitting ? (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.creating"
                  defaultMessage={'Creating...'}
                />
              ) : (
                <FormattedMessage
                  id="aiWorkspace.pages.appShell.appShellPages.providerTemplate.CreateProviderTemplate.create"
                  defaultMessage={'Create Template'}
                />
              )}
            </Button>
          </Box>
        </Box>
      </Box>
    </PageContent>
  );
}
