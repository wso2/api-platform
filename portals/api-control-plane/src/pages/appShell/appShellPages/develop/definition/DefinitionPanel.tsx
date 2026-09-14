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

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Editor from '@monaco-editor/react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Drawer,
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
import { Download, Maximize2, Minimize2, Pencil, Sparkles, Upload } from '@wso2/oxygen-ui-icons-react';
import yaml from 'js-yaml';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ApiError } from '@/api/core/errors';
import {
  usePutRestApiOpenApi,
  useRestApiOpenApi,
  useValidateOpenApiSpec,
  type OpenAPIContent,
} from '@/api/resources/restApis';
import SwaggerSpecViewer from '@/components/SwaggerSpecViewer';
import { MonitorIllustration } from '@/components/illustrations/MonitorIllustration';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';

const messages = defineMessages({
  loading: {
    id: 'develop.definition.DefinitionPanel.loading',
    defaultMessage: 'Loading API definition',
  },
  loadError: {
    id: 'develop.definition.DefinitionPanel.loadError',
    defaultMessage: 'Unable to load the API definition.',
  },
  emptyTitle: {
    id: 'develop.definition.DefinitionPanel.emptyTitle',
    defaultMessage: 'No API definition',
  },
  emptyDescription: {
    id: 'develop.definition.DefinitionPanel.emptyDescription',
    defaultMessage:
      'Upload an OpenAPI or Swagger spec to document your API contract, or generate one from the operations you have already configured.',
  },
  addDefinition: {
    id: 'develop.definition.DefinitionPanel.addDefinition',
    defaultMessage: 'Add definition',
  },
  downloadLabel: {
    id: 'develop.definition.DefinitionPanel.downloadLabel',
    defaultMessage: 'Download Definition',
  },
  generateLabel: {
    id: 'develop.definition.DefinitionPanel.generateLabel',
    defaultMessage: 'Generate from operations',
  },
  updateOpenApi: {
    id: 'develop.definition.DefinitionPanel.updateOpenApi',
    defaultMessage: 'Import Definition',
  },
  editorHeading: {
    id: 'develop.definition.DefinitionPanel.editorHeading',
    defaultMessage: 'API Definition',
  },
  operationsHeading: {
    id: 'develop.definition.DefinitionPanel.operationsHeading',
    defaultMessage: 'API Resources',
  },
  edit: {
    id: 'develop.definition.DefinitionPanel.edit',
    defaultMessage: 'Edit',
  },
  expand: {
    id: 'develop.definition.DefinitionPanel.expand',
    defaultMessage: 'Expand editor',
  },
  collapse: {
    id: 'develop.definition.DefinitionPanel.collapse',
    defaultMessage: 'Collapse editor',
  },
  expandedTitle: {
    id: 'develop.definition.DefinitionPanel.expandedTitle',
    defaultMessage: 'API Definition',
  },
  save: {
    id: 'develop.definition.DefinitionPanel.save',
    defaultMessage: 'Save',
  },
  reset: {
    id: 'develop.definition.DefinitionPanel.reset',
    defaultMessage: 'Reset',
  },
  dialogTitle: {
    id: 'develop.definition.DefinitionPanel.dialogTitle',
    defaultMessage: 'Update OpenAPI Definition',
  },
  dialogImportLabel: {
    id: 'develop.definition.DefinitionPanel.dialogImportLabel',
    defaultMessage: 'Import Specification',
  },
  dialogUrlPlaceholder: {
    id: 'develop.definition.DefinitionPanel.dialogUrlPlaceholder',
    defaultMessage: 'Paste OpenAPI URL or upload a file',
  },
  dialogFetch: {
    id: 'develop.definition.DefinitionPanel.dialogFetch',
    defaultMessage: 'Fetch specification',
  },
  dialogFetching: {
    id: 'develop.definition.DefinitionPanel.dialogFetching',
    defaultMessage: 'Fetching…',
  },
  dialogUpload: {
    id: 'develop.definition.DefinitionPanel.dialogUpload',
    defaultMessage: 'Upload Specification',
  },
  dialogCancel: {
    id: 'develop.definition.DefinitionPanel.dialogCancel',
    defaultMessage: 'Cancel',
  },
  dialogFetchError: {
    id: 'develop.definition.DefinitionPanel.dialogFetchError',
    defaultMessage: 'Failed to fetch specification from the provided URL.',
  },
  dialogParseError: {
    id: 'develop.definition.DefinitionPanel.dialogParseError',
    defaultMessage: 'The fetched content is not a valid OpenAPI/Swagger spec.',
  },
  formatLabel: {
    id: 'develop.definition.DefinitionPanel.formatLabel',
    defaultMessage: 'Source format',
    description: 'Accessible name for the YAML / JSON toggle buttons.',
  },
  operationsParseError: {
    id: 'develop.definition.DefinitionPanel.operationsParseError',
    defaultMessage: 'The current spec cannot be parsed. Fix any syntax errors to preview operations.',
  },
  saveSpecInvalid: {
    id: 'develop.definition.DefinitionPanel.saveSpecInvalid',
    defaultMessage: 'Fix the following issues before saving:',
    description: 'Heading above spec validation errors shown when Save is clicked.',
  },
  saveValidationUnavailable: {
    id: 'develop.definition.DefinitionPanel.saveValidationUnavailable',
    defaultMessage: 'Spec validation is currently unavailable. Please try again.',
    description: 'Error shown when the validation service itself fails (network/auth error).',
  },
});

/** Width of the expanded editor Drawer. */
const EXPANDED_WIDTH = { md: 'min(1000px, 92vw)', xs: '100%' };

type OpenApiSpec = Record<string, unknown>;

function parseSpec(text: string): OpenApiSpec | null {
  const trimmed = text.trimStart();
  if (!trimmed) return null;
  try {
    return JSON.parse(trimmed) as OpenApiSpec;
  } catch {
    try {
      const doc = yaml.load(trimmed);
      return doc && typeof doc === 'object' && !Array.isArray(doc) ? (doc as OpenApiSpec) : null;
    } catch {
      return null;
    }
  }
}

function filenameFromUrl(urlStr: string): string {
  try {
    const { pathname } = new URL(urlStr);
    const last = pathname.split('/').filter(Boolean).pop();
    return last ?? 'openapi.yaml';
  } catch {
    return 'openapi.yaml';
  }
}

export function DefinitionPanel() {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const restApiId = params.apiHandler;

  const fileInputRef = useRef<HTMLInputElement>(null);

  const openApiQuery = useRestApiOpenApi(restApiId);
  const putOpenApi = usePutRestApiOpenApi();
  const validateSpec = useValidateOpenApiSpec();

  const openApiError = openApiQuery.error as ApiError | null;
  const openApiData = openApiQuery.data as OpenAPIContent | undefined;

  // savedContent is always YAML (server always stores YAML).
  const savedContent = openApiData?.content ?? '';
  const [editorText, setEditorText] = useState(savedContent);
  // 'yaml' | 'json' — controls Monaco language and the format sent on Save.
  const [format, setFormat] = useState<'yaml' | 'json'>('yaml');
  // Tracks the filename for the next Save (set when user picks a file or fetches from URL).
  const [pendingFileName, setPendingFileName] = useState<string | null>(null);

  // Validation errors from the backend spec validator. Cleared when edit mode is exited.
  const [saveValidationErrors, setSaveValidationErrors] = useState<string[] | null>(null);
  const [isValidating, setIsValidating] = useState(false);

  // Whether the editor is expanded into a full-width right Drawer.
  const [expanded, setExpanded] = useState(false);

  // Whether the editor is in edit mode (writable). Read-only by default.
  const [isEditing, setIsEditing] = useState(false);

  // Dialog state — import / update dialog
  const [dialogOpen, setDialogOpen] = useState(false);
  const [specUrl, setSpecUrl] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Sync editor when the stored spec (re-)loads; always reset to YAML view.
  useEffect(() => {
    setEditorText(openApiData?.content ?? '');
    setPendingFileName(null);
    setFormat('yaml');
  }, [openApiData?.content]);

  // Clear save-time validation errors whenever the editor content changes.
  useEffect(() => {
    setSaveValidationErrors(null);
  }, [editorText]);

  // Dirty check compares parsed semantic content, not raw strings.
  // Format-toggling (YAML ↔ JSON) never affects dirty state.
  const isDirty = useMemo(() => {
    const savedParsed = parseSpec(savedContent);
    const editorParsed = parseSpec(editorText);
    if (!savedParsed && !editorParsed) return false;
    if (!savedParsed || !editorParsed) return true;
    return JSON.stringify(savedParsed) !== JSON.stringify(editorParsed);
  }, [savedContent, editorText]);

  // Parsed spec for the operations panel — derived live from editor text.
  const parsedSpecForOperations = useMemo(() => parseSpec(editorText), [editorText]);

  // savedContent in the current display format — used by the Reset button.
  const savedInCurrentFormat = useMemo(() => {
    if (!savedContent || format === 'yaml') return savedContent;
    try {
      const parsed = yaml.load(savedContent) as Record<string, unknown>;
      return JSON.stringify(parsed, null, 2);
    } catch {
      return savedContent;
    }
  }, [savedContent, format]);

  const isSaving = isValidating || putOpenApi.isPending;

  const handleFormatToggle = useCallback(
    (newFormat: 'yaml' | 'json') => {
      if (newFormat === format) return;
      try {
        if (newFormat === 'json') {
          const parsed = yaml.load(editorText) as Record<string, unknown>;
          setEditorText(JSON.stringify(parsed, null, 2));
        } else {
          const parsed = JSON.parse(editorText) as Record<string, unknown>;
          setEditorText(yaml.dump(parsed));
        }
      } catch {
        // If conversion fails (invalid content), just switch the display language.
      }
      setFormat(newFormat);
    },
    [format, editorText],
  );

  const closeDialog = () => {
    setDialogOpen(false);
    setSpecUrl('');
    setFetchError(null);
  };

  // Load content into the editor from a file — does NOT immediately PUT to backend.
  // Always loads as YAML; converts JSON uploads automatically.
  const applyFileContent = (file: File) => {
    void file.text().then((text) => {
      let yamlText = text;
      try {
        const parsed = JSON.parse(text) as Record<string, unknown>;
        yamlText = yaml.dump(parsed);
      } catch {
        // Already YAML, use as-is.
      }
      setEditorText(yamlText);
      setPendingFileName(file.name.replace(/\.json$/i, '.yaml'));
      setFormat('yaml');
    });
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    applyFileContent(file);
    event.target.value = '';
    closeDialog();
  };

  // Fetch spec from URL and load into the editor — does NOT immediately PUT.
  const handleFetchSpec = async () => {
    const url = specUrl.trim();
    if (!url) return;
    setIsFetchingSpec(true);
    setFetchError(null);
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error('fetch failed');
      const text = await response.text();
      if (!parseSpec(text)) {
        setFetchError(intl.formatMessage(messages.dialogParseError));
        return;
      }
      let yamlText = text;
      try {
        const parsed = JSON.parse(text) as Record<string, unknown>;
        yamlText = yaml.dump(parsed);
      } catch {
        // Already YAML.
      }
      setEditorText(yamlText);
      setPendingFileName(filenameFromUrl(url).replace(/\.json$/i, '.yaml'));
      setFormat('yaml');
      closeDialog();
    } catch {
      setFetchError(intl.formatMessage(messages.dialogFetchError));
    } finally {
      setIsFetchingSpec(false);
    }
  };

  const handleDownload = () => {
    let content = savedContent;
    let filename = 'openapi.yaml';
    let mimeType = 'application/x-yaml';

    if (format === 'json') {
      try {
        const parsed = yaml.load(savedContent) as Record<string, unknown>;
        content = JSON.stringify(parsed, null, 2);
        filename = 'openapi.json';
        mimeType = 'application/json';
      } catch {
        // Fall back to YAML if conversion fails.
      }
    }

    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleSave = async () => {
    if (!restApiId || isSaving) return;
    // Always persist as YAML. Convert from JSON if the editor is in JSON mode.
    let yamlContent = editorText;
    if (format === 'json') {
      try {
        const parsed = JSON.parse(editorText) as Record<string, unknown>;
        yamlContent = yaml.dump(parsed);
      } catch {
        // Invalid JSON — let the server reject it.
      }
    }

    const isEmpty = !yamlContent.trim();

    if (!isEmpty) {
      setIsValidating(true);
      setSaveValidationErrors(null);
      try {
        const validation = await validateSpec.mutateAsync(yamlContent);
        if (!validation.isValid) {
          setSaveValidationErrors(validation.errors.map((e) => e.message));
          return;
        }
      } catch {
        setSaveValidationErrors([intl.formatMessage(messages.saveValidationUnavailable)]);
        return;
      } finally {
        setIsValidating(false);
      }
    }

    const fileName = pendingFileName ?? 'openapi.yaml';
    const blob = new Blob([yamlContent], { type: 'application/x-yaml' });
    const file = new File([blob], fileName, { type: 'application/x-yaml' });
    const formData = new FormData();
    formData.append('file', file);
    putOpenApi.mutate({ restApiId, formData }, { onSuccess: () => setIsEditing(false) });
  };

  if (openApiQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  if (openApiError && openApiError.status !== 404) {
    return <ErrorState title={intl.formatMessage(messages.loadError)} />;
  }

  const hasSpec = Boolean(openApiData);

  return (
    <>
      {/* Hidden file input — triggered from inside the dialog. */}
      <input
        accept=".json,.yaml,.yml"
        onChange={handleFileChange}
        ref={fileInputRef}
        style={{ display: 'none' }}
        type="file"
      />

      {/* Update OpenAPI Definition dialog */}
      <Dialog fullWidth maxWidth="sm" onClose={closeDialog} open={dialogOpen}>
        <DialogTitle>{intl.formatMessage(messages.dialogTitle)}</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 1 }}>
            <FormControl fullWidth>
              <FormLabel>{intl.formatMessage(messages.dialogImportLabel)}</FormLabel>
              <Stack alignItems="center" direction="row" spacing={1.5} sx={{ mt: 1 }}>
                <TextField
                  fullWidth
                  onChange={(e) => {
                    setSpecUrl(e.target.value);
                    setFetchError(null);
                  }}
                  placeholder={intl.formatMessage(messages.dialogUrlPlaceholder)}
                  size="small"
                  value={specUrl}
                />
                <Button
                  disabled={isFetchingSpec || !specUrl.trim()}
                  onClick={() => void handleFetchSpec()}
                  size="small"
                  sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                  variant="outlined"
                >
                  {isFetchingSpec
                    ? intl.formatMessage(messages.dialogFetching)
                    : intl.formatMessage(messages.dialogFetch)}
                </Button>
                <Divider flexItem orientation="vertical">
                  Or
                </Divider>
                <Button
                  onClick={() => fileInputRef.current?.click()}
                  size="small"
                  sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                  variant="outlined"
                >
                  {intl.formatMessage(messages.dialogUpload)}
                </Button>
              </Stack>
              {fetchError && (
                <Typography color="error" sx={{ mt: 1 }} variant="caption">
                  {fetchError}
                </Typography>
              )}
            </FormControl>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button color="secondary" onClick={closeDialog} variant="outlined">
            {intl.formatMessage(messages.dialogCancel)}
          </Button>
        </DialogActions>
      </Dialog>

      {hasSpec || editorText ? (
        <Stack spacing={2}>
          {/* Action bar */}
          <Stack direction="row" spacing={1} sx={{ justifyContent: 'flex-end' }}>
            <Button
              onClick={() => setDialogOpen(true)}
              startIcon={<Upload size={16} />}
              variant="outlined"
            >
              {intl.formatMessage(messages.updateOpenApi)}
            </Button>
            {hasSpec && (
              <Button
                onClick={handleDownload}
                startIcon={<Download size={16} />}
                variant="outlined"
              >
                {intl.formatMessage(messages.downloadLabel)}
              </Button>
            )}
          </Stack>

          {/* Split view: left = spec editor, right = live resources.
              Both panels carry an explicit height so Monaco's height="100%"
              resolves correctly through the flex chain. */}
          <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: '3fr 2fr' }}>
            {/* Left panel — spec editor (blurred static preview when the Drawer is open) */}
            {expanded ? (
              <Box
                sx={{
                  bgcolor: 'background.paper',
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  filter: 'blur(2px)',
                  opacity: 0.45,
                  overflow: 'hidden',
                  pointerEvents: 'none',
                  userSelect: 'none',
                }}
              >
                <Box
                  component="pre"
                  sx={{
                    bgcolor: '#1e1e1e',
                    color: '#d4d4d4',
                    fontFamily: "'Menlo', 'Monaco', 'Courier New', monospace",
                    fontSize: 12,
                    height: '100%',
                    lineHeight: '20px',
                    m: 0,
                    overflow: 'hidden',
                    p: 2,
                    whiteSpace: 'pre',
                  }}
                >
                  {editorText}
                </Box>
              </Box>
            ) : (
              <Box
                sx={{
                  bgcolor: 'background.paper',
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  display: 'flex',
                  flexDirection: 'column',
                  overflow: 'hidden',
                }}
              >
                {/* Editor toolbar: title left | format toggle + expand right */}
                <Box
                  sx={{
                    alignItems: 'center',
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                    display: 'flex',
                    flexShrink: 0,
                    justifyContent: 'space-between',
                    px: 2,
                    py: 1,
                  }}
                >
                  <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
                    <FormattedMessage {...messages.editorHeading} />
                  </Typography>
                  <Stack alignItems="center" direction="row" spacing={1}>
                    {!isEditing && (
                      <Button
                        onClick={() => setIsEditing(true)}
                        size="small"
                        startIcon={<Pencil size={16} />}
                        variant="outlined"
                      >
                        <FormattedMessage {...messages.edit} />
                      </Button>
                    )}
                    <ToggleButtonGroup
                      aria-label={intl.formatMessage(messages.formatLabel)}
                      color="primary"
                      exclusive
                      onChange={(_event, next: 'yaml' | 'json' | null) => {
                        if (next !== null) handleFormatToggle(next);
                      }}
                      size="small"
                      value={format}
                    >
                      <ToggleButton value="yaml">YAML</ToggleButton>
                      <ToggleButton value="json">JSON</ToggleButton>
                    </ToggleButtonGroup>
                    <Tooltip title={intl.formatMessage(messages.expand)}>
                      <IconButton
                        aria-label={intl.formatMessage(messages.expand)}
                        onClick={() => setExpanded(true)}
                        size="small"
                      >
                        <Maximize2 size={16} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                </Box>

                {/* Monaco editor — fills remaining height */}
                <Box sx={{ flex: 1, minHeight: 0, p: 1 }}>
                  <Editor
                    height="100%"
                    language={format}
                    loading={
                      <Box sx={{ bgcolor: '#1e1e1e', height: '100%', p: 2 }}>
                        <Typography color="text.disabled" variant="body2">
                          Loading editor…
                        </Typography>
                      </Box>
                    }
                    onChange={(value) => setEditorText(value ?? '')}
                    options={{
                      automaticLayout: true,
                      fontSize: 12,
                      lineHeight: 20,
                      minimap: { enabled: false },
                      readOnly: !isEditing,
                      scrollBeyondLastLine: false,
                      wordWrap: 'on',
                    }}
                    theme="vs-dark"
                    value={editorText}
                  />
                </Box>

                {/* Save / Reset bar — shown whenever the editor is in edit mode */}
                {isEditing && (
                  <Box sx={{ borderColor: 'divider', borderTop: '1px solid', flexShrink: 0 }}>
                    {saveValidationErrors !== null && saveValidationErrors.length > 0 && (
                      <Alert severity="error" sx={{ borderRadius: 0, m: 0 }}>
                        <FormattedMessage {...messages.saveSpecInvalid} />
                        <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
                          {saveValidationErrors.map((msg, i) => (
                            <Typography component="li" key={i} variant="body2">
                              {msg}
                            </Typography>
                          ))}
                        </Box>
                      </Alert>
                    )}
                    <Box
                      sx={{
                        alignItems: 'center',
                        display: 'flex',
                        gap: 1,
                        justifyContent: 'flex-end',
                        px: 2,
                        py: 1.5,
                      }}
                    >
                      <Button
                        color="secondary"
                        disabled={isSaving}
                        onClick={() => {
                          setEditorText(savedInCurrentFormat);
                          setPendingFileName(null);
                          setIsEditing(false);
                          setSaveValidationErrors(null);
                        }}
                        size="small"
                        variant="outlined"
                      >
                        {intl.formatMessage(messages.reset)}
                      </Button>
                      <Button
                        disabled={!isDirty || !editorText.trim()}
                        loading={isSaving}
                        onClick={() => void handleSave()}
                        size="small"
                        variant="contained"
                      >
                        {intl.formatMessage(messages.save)}
                      </Button>
                    </Box>
                  </Box>
                )}
              </Box>
            )}

            {/* Right panel — live resources derived from spec */}
            <Box
              sx={{
                bgcolor: 'background.paper',
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                display: 'flex',
                flexDirection: 'column',
                overflow: 'hidden',
              }}
            >
              {/* Resources panel header */}
              <Box
                sx={{
                  borderBottom: '1px solid',
                  borderColor: 'divider',
                  flexShrink: 0,
                  px: 2,
                  py: 1,
                }}
              >
                <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
                  <FormattedMessage {...messages.operationsHeading} />
                </Typography>
              </Box>

              {/* Resources viewer — updates in real time as the editor text changes */}
              <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 1, py: 0.5 }}>
                {parsedSpecForOperations ? (
                  <Box
                    sx={{
                      '& .swagger-ui': { bgcolor: 'transparent' },
                      '& .swagger-ui .opblock-tag': { position: 'relative' },
                      '& .swagger-ui .opblock-tag a, & .swagger-ui .opblock-tag a.nostyle': {
                        fontSize: '1rem',
                        fontWeight: 600,
                      },
                      '& .swagger-ui .opblock-tag small': { fontSize: '0.8125rem' },
                      '& .swagger-ui .opblock-summary-method': { fontSize: '0.8125rem' },
                      '& .swagger-ui .opblock-summary-path, & .swagger-ui .opblock-summary-path span': {
                        fontSize: '0.9375rem',
                      },
                      '& .swagger-ui .opblock-summary-description': { fontSize: '0.8125rem' },
                      '& .swagger-ui .opblock-tag::before': {
                        color: 'currentColor',
                        content: '"›"',
                        display: 'inline-block',
                        fontSize: '1.1rem',
                        fontWeight: 700,
                        marginRight: '6px',
                        opacity: 0.6,
                        transform: 'rotate(0deg)',
                        transition: 'transform 0.15s ease',
                      },
                      '& .swagger-ui .opblock-tag[data-is-open="true"]::before': {
                        transform: 'rotate(90deg)',
                      },
                    }}
                  >
                    <SwaggerSpecViewer
                      disableResponseSection
                      disableTryOutBtn
                      hideInfoSection
                      hideServers
                      spec={parsedSpecForOperations}
                    />
                  </Box>
                ) : (
                  <Box
                    sx={{
                      alignItems: 'center',
                      display: 'flex',
                      height: '100%',
                      justifyContent: 'center',
                      px: 3,
                    }}
                  >
                    <Typography color="text.secondary" textAlign="center" variant="body2">
                      {intl.formatMessage(messages.operationsParseError)}
                    </Typography>
                  </Box>
                )}
              </Box>
            </Box>
          </Box>

          {/* Expanded editor Drawer — only one Monaco instance is ever mounted; text
              survives the open/close transition as React state. */}
          <Drawer
            anchor="right"
            onClose={() => setExpanded(false)}
            open={expanded}
            slotProps={{ paper: { sx: { width: EXPANDED_WIDTH } } }}
          >
            {expanded && (
              <Stack spacing={0} sx={{ height: '100%', minHeight: 0 }}>
                {/* Drawer toolbar */}
                <Box
                  sx={{
                    alignItems: 'center',
                    bgcolor: 'background.paper',
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                    display: 'flex',
                    flexShrink: 0,
                    justifyContent: 'space-between',
                    px: 3,
                    py: 1.5,
                  }}
                >
                  <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
                    {intl.formatMessage(messages.expandedTitle)}
                  </Typography>
                  <Stack alignItems="center" direction="row" spacing={1}>
                    {!isEditing && (
                      <Button
                        onClick={() => setIsEditing(true)}
                        size="small"
                        startIcon={<Pencil size={16} />}
                        variant="outlined"
                      >
                        <FormattedMessage {...messages.edit} />
                      </Button>
                    )}
                    <ToggleButtonGroup
                      aria-label={intl.formatMessage(messages.formatLabel)}
                      color="primary"
                      exclusive
                      onChange={(_event, next: 'yaml' | 'json' | null) => {
                        if (next !== null) handleFormatToggle(next);
                      }}
                      size="small"
                      value={format}
                    >
                      <ToggleButton value="yaml">YAML</ToggleButton>
                      <ToggleButton value="json">JSON</ToggleButton>
                    </ToggleButtonGroup>
                    <Tooltip title={intl.formatMessage(messages.collapse)}>
                      <IconButton
                        aria-label={intl.formatMessage(messages.collapse)}
                        onClick={() => setExpanded(false)}
                        size="small"
                      >
                        <Minimize2 size={16} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                </Box>

                {/* Monaco fills the remaining Drawer height */}
                <Box sx={{ flex: 1, minHeight: 0 }}>
                  <Editor
                    height="100%"
                    language={format}
                    loading={
                      <Box sx={{ bgcolor: '#1e1e1e', height: '100%', p: 2 }}>
                        <Typography color="text.disabled" variant="body2">
                          Loading editor…
                        </Typography>
                      </Box>
                    }
                    onChange={(value) => setEditorText(value ?? '')}
                    options={{
                      automaticLayout: true,
                      fontSize: 12,
                      lineHeight: 20,
                      minimap: { enabled: true },
                      readOnly: !isEditing,
                      scrollBeyondLastLine: false,
                      wordWrap: 'on',
                    }}
                    theme="vs-dark"
                    value={editorText}
                  />
                </Box>

                {/* Save / Reset bar inside the Drawer — shown when in edit mode */}
                {isEditing && (
                  <Box sx={{ borderColor: 'divider', borderTop: '1px solid', flexShrink: 0 }}>
                    {saveValidationErrors !== null && saveValidationErrors.length > 0 && (
                      <Alert severity="error" sx={{ borderRadius: 0, m: 0 }}>
                        <FormattedMessage {...messages.saveSpecInvalid} />
                        <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
                          {saveValidationErrors.map((msg, i) => (
                            <Typography component="li" key={i} variant="body2">
                              {msg}
                            </Typography>
                          ))}
                        </Box>
                      </Alert>
                    )}
                    <Box
                      sx={{
                        alignItems: 'center',
                        display: 'flex',
                        gap: 1,
                        justifyContent: 'flex-end',
                        px: 3,
                        py: 1.5,
                      }}
                    >
                      <Button
                        color="secondary"
                        disabled={isSaving}
                        onClick={() => {
                          setEditorText(savedInCurrentFormat);
                          setPendingFileName(null);
                          setIsEditing(false);
                          setSaveValidationErrors(null);
                        }}
                        size="small"
                        variant="outlined"
                      >
                        {intl.formatMessage(messages.reset)}
                      </Button>
                      <Button
                        disabled={!isDirty || !editorText.trim()}
                        loading={isSaving}
                        onClick={() => void handleSave()}
                        size="small"
                        variant="contained"
                      >
                        {intl.formatMessage(messages.save)}
                      </Button>
                    </Box>
                  </Box>
                )}
              </Stack>
            )}
          </Drawer>
        </Stack>
      ) : (
        /* Empty state: no spec uploaded yet. */
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            flexGrow: 1,
            justifyContent: 'center',
            minHeight: '50vh',
            px: 3,
            py: 6,
          }}
        >
          <Stack alignItems="center" spacing={2} sx={{ maxWidth: 440 }}>
            <MonitorIllustration />
            <Typography sx={{ fontWeight: 700, pt: 1 }} variant="h4">
              {intl.formatMessage(messages.emptyTitle)}
            </Typography>
            <Typography color="text.secondary" sx={{ textAlign: 'center' }}>
              {intl.formatMessage(messages.emptyDescription)}
            </Typography>
            <Stack direction="row" spacing={1} sx={{ pt: 1 }}>
              <Button
                onClick={() => setDialogOpen(true)}
                startIcon={<Upload size={16} />}
                variant="contained"
              >
                {intl.formatMessage(messages.addDefinition)}
              </Button>
              <Button
                disabled
                startIcon={<Sparkles size={16} />}
                variant="outlined"
              >
                {intl.formatMessage(messages.generateLabel)}
              </Button>
            </Stack>
          </Stack>
        </Box>
      )}
    </>
  );
}
