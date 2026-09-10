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

import { useEffect, useMemo, useRef, useState, useCallback } from 'react';
import Editor from '@monaco-editor/react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Divider,
  Drawer,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Download, Maximize2, Minimize2, Pencil, Sparkles, Trash2, Upload } from '@wso2/oxygen-ui-icons-react';
import yaml from 'js-yaml';
import { defineMessages, useIntl } from 'react-intl';

import { ApiError } from '@/api/core/errors';
import {
  useDeleteRestApiOpenApi,
  usePutRestApiOpenApi,
  useRestApiOpenApi,
  useUpdateRestApi,
  useValidateOpenApiSpec,
  type RestApi,
  type OpenAPIContent,
} from '@/api/resources/restApis';
import { extractOperations } from '@/pages/appShell/appShellPages/apis/create/utils/specDetails';
import { OpenAPIOperationsView } from '@/components/OpenAPIOperationsView';
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
  uploadSpec: {
    id: 'develop.definition.DefinitionPanel.uploadSpec',
    defaultMessage: 'Upload spec',
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
  deleteLabel: {
    id: 'develop.definition.DefinitionPanel.deleteLabel',
    defaultMessage: 'Delete',
  },
  updateOpenApi: {
    id: 'develop.definition.DefinitionPanel.updateOpenApi',
    defaultMessage: 'Import Definition',
  },
  editorHeading: {
    id: 'develop.definition.DefinitionPanel.editorHeading',
    defaultMessage: 'OpenAPI Definition',
  },
  resourcesHeading: {
    id: 'develop.definition.DefinitionPanel.resourcesHeading',
    defaultMessage: 'Resources',
  },
  save: {
    id: 'develop.definition.DefinitionPanel.save',
    defaultMessage: 'Save',
  },
  reset: {
    id: 'develop.definition.DefinitionPanel.reset',
    defaultMessage: 'Reset',
  },
  resourcesNoOps: {
    id: 'develop.definition.DefinitionPanel.resourcesNoOps',
    defaultMessage: 'No resources defined. Save a spec to populate this list.',
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
  editLabel: {
    id: 'develop.definition.DefinitionPanel.editLabel',
    defaultMessage: 'Edit',
  },
  deleteConfirmTitle: {
    id: 'develop.definition.DefinitionPanel.deleteConfirmTitle',
    defaultMessage: 'Delete API Definition',
  },
  deleteConfirmMessage: {
    id: 'develop.definition.DefinitionPanel.deleteConfirmMessage',
    defaultMessage:
      'Are you sure you want to delete the API definition? This action cannot be undone.',
  },
  deleteConfirmButton: {
    id: 'develop.definition.DefinitionPanel.deleteConfirmButton',
    defaultMessage: 'Delete',
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
  saveSpecInvalid: {
    id: 'develop.definition.DefinitionPanel.saveSpecInvalid',
    defaultMessage: 'Fix the following issues before saving:',
    description: 'Heading above spec validation errors shown when Save is clicked.',
  },
});

const EXPANDED_WIDTH = { md: 'min(1100px, 92vw)', xs: '100%' };

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

export function DefinitionPanel({ api }: { api: RestApi }) {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const restApiId = params.apiHandler;

  const fileInputRef = useRef<HTMLInputElement>(null);

  const openApiQuery = useRestApiOpenApi(restApiId);
  const putOpenApi = usePutRestApiOpenApi();
  const deleteOpenApi = useDeleteRestApiOpenApi();
  const updateApi = useUpdateRestApi();
  const validateSpec = useValidateOpenApiSpec();

  const openApiError = openApiQuery.error as ApiError | null;
  const openApiData = openApiQuery.data as OpenAPIContent | undefined;

  // savedContent is always YAML (server always stores YAML).
  const savedContent = openApiData?.content ?? '';
  const [editorText, setEditorText] = useState(savedContent);
  // 'yaml' | 'json' — controls the Monaco language and the format sent on Save.
  const [format, setFormat] = useState<'yaml' | 'json'>('yaml');
  // Tracks the filename for the next Save (set when user picks a file or fetches from URL).
  const [pendingFileName, setPendingFileName] = useState<string | null>(null);

  // Validation errors from the backend, shown in the save bar. Cleared on each edit.
  const [saveValidationErrors, setSaveValidationErrors] = useState<string[] | null>(null);
  const [isValidating, setIsValidating] = useState(false);

  // Whether the editor is in edit mode (writable). Default: read-only.
  const [isEditing, setIsEditing] = useState(false);
  // Whether the editor is expanded into a full-width right drawer.
  const [expanded, setExpanded] = useState(false);

  // Dialog state — import/update dialog
  const [dialogOpen, setDialogOpen] = useState(false);
  const [specUrl, setSpecUrl] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Delete confirmation dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Sync editor when the stored spec (re-)loads; always reset to YAML view.
  useEffect(() => {
    setEditorText(openApiData?.content ?? '');
    setPendingFileName(null);
    setFormat('yaml');
    setIsEditing(false);
  }, [openApiData?.content]);

  // Clear save-time validation errors whenever the editor content changes.
  useEffect(() => {
    setSaveValidationErrors(null);
  }, [editorText]);

  // Dirty check compares parsed semantic content, not raw strings.
  // This means format-toggling (YAML ↔ JSON) never affects dirty state.
  const isDirty = useMemo(() => {
    const savedParsed = parseSpec(savedContent);
    const editorParsed = parseSpec(editorText);
    if (!savedParsed && !editorParsed) return false;
    if (!savedParsed || !editorParsed) return true;
    return JSON.stringify(savedParsed) !== JSON.stringify(editorParsed);
  }, [savedContent, editorText]);

  // savedContent in the current display format — used only by the Reset button.
  const savedInCurrentFormat = useMemo(() => {
    if (!savedContent || format === 'yaml') return savedContent;
    try {
      const parsed = yaml.load(savedContent) as Record<string, unknown>;
      return JSON.stringify(parsed, null, 2);
    } catch {
      return savedContent;
    }
  }, [savedContent, format]);
  const isSaving = isValidating || putOpenApi.isPending || updateApi.isPending;

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
      setIsEditing(true);
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
      // Convert to YAML if the fetched content is JSON.
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
      setIsEditing(true);
      closeDialog();
    } catch {
      setFetchError(intl.formatMessage(messages.dialogFetchError));
    } finally {
      setIsFetchingSpec(false);
    }
  };

  // Opens the delete confirmation dialog.
  const handleDelete = () => setDeleteDialogOpen(true);

  // Called when the user confirms deletion in the dialog.
  const handleConfirmDelete = () => {
    if (!restApiId) return;
    setDeleteDialogOpen(false);
    deleteOpenApi.mutate({ restApiId }, {
      onSuccess: () => {
        setEditorText('');
        setPendingFileName(null);
        setFormat('yaml');
        setIsEditing(false);
      },
    });
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

  const handleGenerateSpec = () => {
    const operations = api.operations ?? [];

    // Build OpenAPI paths from existing operations. Falls back to a wildcard
    // resource when the API has no operations yet.
    const paths: Record<string, Record<string, unknown>> = {};
    if (operations.length === 0) {
      paths['/*'] = {
        get: { summary: 'Get Resource', responses: { '200': { description: 'Success' } } },
        post: { summary: 'Create Resource', responses: { '201': { description: 'Created' } } },
      };
    } else {
      for (const op of operations) {
        const path = op.request.path;
        const method = op.request.method.toLowerCase();
        if (!paths[path]) paths[path] = {};
        paths[path][method] = {
          summary: op.name,
          ...(op.description ? { description: op.description } : {}),
          responses: { '200': { description: 'Success' } },
        };
      }
    }

    const spec = {
      openapi: '3.0.3',
      info: {
        title: api.displayName ?? 'API',
        version: api.version ?? '1.0.0',
        ...(api.description ? { description: api.description } : {}),
      },
      ...(api.upstream?.main?.url ? { servers: [{ url: api.upstream.main.url }] } : {}),
      paths,
    };

    // Load into the editor without making a BE call — the Save button at the
    // bottom of the editor issues PUT /openapi when the user is ready.
    // Policies on existing operations are preserved by handleSave, which
    // matches spec operations back to api.operations by method+path.
    setEditorText(yaml.dump(spec));
    setPendingFileName('openapi.yaml');
    setFormat('yaml');
    setIsEditing(true);
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

    // Validate non-empty specs before persisting — skip for empty content so the
    // user can save an empty editor (treating it as deleting the spec).
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
        // Network/auth error — don't block the save; let the backend respond.
      } finally {
        setIsValidating(false);
      }
    }

    const fileName = pendingFileName ?? 'openapi.yaml';
    const blob = new Blob([yamlContent], { type: 'application/x-yaml' });
    const file = new File([blob], fileName, { type: 'application/x-yaml' });
    const formData = new FormData();
    formData.append('file', file);
    putOpenApi.mutate({ restApiId, formData }, {
      onSuccess: () => setIsEditing(false),
    });

    // Derive operations from spec only when there is content. When the editor is
    // empty the PUT above handles the spec side; existing operations are left
    // untouched so deleting the spec definition doesn't wipe configured resources.
    if (!isEmpty) {
      // method+path uniquely identifies an operation (operationId is not persisted).
      // A changed method or path is treated as a new operation; its predecessor's
      // policies are dropped.
      const specOps = extractOperations(parseSpec(yamlContent) ?? undefined);
      const existingByKey = new Map(
        (api.operations ?? []).map((op) => [`${op.request.method}:${op.request.path}`, op]),
      );
      const ops = specOps.map((op) => {
        const existing = existingByKey.get(`${op.request.method}:${op.request.path}`);
        return {
          name: op.name,
          ...(op.description !== undefined ? { description: op.description } : {}),
          request: {
            method: op.request.method,
            path: op.request.path,
            ...(existing?.request.policies?.length ? { policies: existing.request.policies } : {}),
          },
        };
      });
      updateApi.mutate({ restApiId, body: { ...api, operations: ops } });
    }
  };

  if (openApiQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  if (openApiError && openApiError.status !== 404) {
    return <ErrorState title={intl.formatMessage(messages.loadError)} />;
  }

  const hasSpec = Boolean(openApiData);
  const currentOperations = api.operations ?? [];

  // Height adjusts so the expanded drawer fills the viewport; normal view is fixed.
  const editorHeight = expanded ? 'calc(100vh - 196px)' : '540px';

  // The editor panel is defined here (not inside JSX) so it can be placed in either
  // the normal Grid column or the Drawer without mounting two Monaco instances at once.
  const editorPanel = (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        overflow: 'hidden',
      }}
    >
      {/* Editor header */}
      <Box
        sx={{
          alignItems: 'center',
          borderBottom: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          justifyContent: 'space-between',
          px: 2,
          py: 1,
        }}
      >
        <Stack alignItems="center" direction="row" spacing={1.5}>
          <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
            {intl.formatMessage(messages.editorHeading)}
          </Typography>
          {/* Format toggle */}
          <Stack direction="row">
            <Button
              onClick={() => handleFormatToggle('yaml')}
              size="small"
              sx={{ borderRadius: '4px 0 0 4px', minWidth: 52 }}
              variant={format === 'yaml' ? 'contained' : 'outlined'}
            >
              YAML
            </Button>
            <Button
              onClick={() => handleFormatToggle('json')}
              size="small"
              sx={{ borderRadius: '0 4px 4px 0', ml: '-1px', minWidth: 52 }}
              variant={format === 'json' ? 'contained' : 'outlined'}
            >
              JSON
            </Button>
          </Stack>
        </Stack>
        <Stack alignItems="center" direction="row" spacing={1}>
          {/* Edit button — only shown when not in edit mode */}
          {!isEditing && (
            <Button
              onClick={() => setIsEditing(true)}
              size="small"
              startIcon={<Pencil size={14} />}
              variant="outlined"
            >
              {intl.formatMessage(messages.editLabel)}
            </Button>
          )}
          {/* Expand / collapse */}
          <Tooltip title={intl.formatMessage(expanded ? messages.collapse : messages.expand)}>
            <IconButton
              aria-label={intl.formatMessage(expanded ? messages.collapse : messages.expand)}
              onClick={() => setExpanded((prev) => !prev)}
              size="small"
            >
              {expanded ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </IconButton>
          </Tooltip>
        </Stack>
      </Box>
      <Editor
        height={editorHeight}
        language={format}
        loading={
          <Box sx={{ bgcolor: '#1e1e1e', height: editorHeight, p: 2 }}>
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
          minimap: { enabled: expanded },
          readOnly: !isEditing,
          scrollBeyondLastLine: false,
          wordWrap: 'on',
        }}
        theme="vs-dark"
        value={editorText}
      />
      {/* Save / Reset bar — shown at the bottom of the editor when there are unsaved changes */}
      {isDirty && (
        <Box sx={{ borderTop: '1px solid', borderColor: 'divider' }}>
          {saveValidationErrors !== null && saveValidationErrors.length > 0 && (
            <Alert severity="error" sx={{ borderRadius: 0 }}>
              {intl.formatMessage(messages.saveSpecInvalid)}
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
            <Button loading={isSaving} onClick={() => void handleSave()} size="small" variant="contained">
              {intl.formatMessage(messages.save)}
            </Button>
          </Box>
        </Box>
      )}
    </Box>
  );

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
      <Dialog
        fullWidth
        maxWidth="sm"
        onClose={closeDialog}
        open={dialogOpen}
      >
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

      {/* Delete confirmation dialog */}
      <Dialog
        maxWidth="xs"
        onClose={() => setDeleteDialogOpen(false)}
        open={deleteDialogOpen}
      >
        <DialogTitle>{intl.formatMessage(messages.deleteConfirmTitle)}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {intl.formatMessage(messages.deleteConfirmMessage)}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button color="secondary" onClick={() => setDeleteDialogOpen(false)} variant="outlined">
            {intl.formatMessage(messages.dialogCancel)}
          </Button>
          <Button
            color="error"
            loading={deleteOpenApi.isPending}
            onClick={handleConfirmDelete}
            variant="contained"
          >
            {intl.formatMessage(messages.deleteConfirmButton)}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Show editor view when a spec is saved OR the user has loaded content into the editor. */}
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
            {hasSpec && (
              <Tooltip title={intl.formatMessage(messages.deleteLabel)}>
                <span style={{ display: 'inline-flex', alignItems: 'center' }}>
                  <Button
                    color="error"
                    loading={deleteOpenApi.isPending}
                    onClick={handleDelete}
                    sx={{ borderRadius: '50%', minWidth: 0, p: '9px' }}
                    variant="outlined"
                  >
                    <Trash2 size={16} />
                  </Button>
                </span>
              </Tooltip>
            )}
          </Stack>

          <Grid container spacing={2} sx={{ alignItems: 'flex-start' }}>
            {/* Left: Monaco editor (or placeholder when expanded into the Drawer) */}
            <Grid size={{ md: 7, xs: 12 }}>
              {expanded ? (
                // Static blurred preview — keeps the visual context without mounting a second Monaco.
                <Box
                  sx={{
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
                      height: 540,
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
                editorPanel
              )}
            </Grid>

            {/* Right: Resources — reflects api.operations, updated on Save */}
            <Grid size={{ md: 5, xs: 12 }}>
              <Box
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  p: 2,
                }}
              >
                <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="subtitle2">
                  {intl.formatMessage(messages.resourcesHeading)}
                  {currentOperations.length > 0 && (
                    <Typography
                      color="text.secondary"
                      component="span"
                      sx={{ fontWeight: 400, ml: 0.75 }}
                      variant="caption"
                    >
                      ({currentOperations.length})
                    </Typography>
                  )}
                </Typography>

                {currentOperations.length === 0 ? (
                  <Typography color="text.secondary" variant="body2">
                    {intl.formatMessage(messages.resourcesNoOps)}
                  </Typography>
                ) : (
                  <Box
                    sx={{
                      maxHeight: { md: 'calc(540px - 40px)', xs: 320 },
                      overflowY: 'auto',
                      pr: 0.5,
                    }}
                  >
                    <OpenAPIOperationsView operations={currentOperations} showDelete={false} />
                  </Box>
                )}
              </Box>
            </Grid>
          </Grid>

          {/* Expanded editor drawer — mounts the editor only while open so exactly one Monaco
              instance exists at a time; text survives the remount as React state. */}
          <Drawer
            anchor="right"
            onClose={() => setExpanded(false)}
            open={expanded}
            slotProps={{ paper: { sx: { width: EXPANDED_WIDTH } } }}
          >
            {expanded && (
              <Stack spacing={2} sx={{ height: '100%', minHeight: 0, p: 3 }}>
                <Typography sx={{ flexShrink: 0, fontWeight: 700 }} variant="h6">
                  {intl.formatMessage(messages.expandedTitle)}
                </Typography>
                {editorPanel}
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
                onClick={handleGenerateSpec}
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
