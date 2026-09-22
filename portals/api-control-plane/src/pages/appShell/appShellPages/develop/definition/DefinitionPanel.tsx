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
  ButtonGroup,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Braces, Download, Pencil, Plus, Upload } from '@wso2/oxygen-ui-icons-react';
import yaml from 'js-yaml';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ApiError } from '@/api/core/errors';
import {
  usePutRestApiOpenApi,
  useRestApi,
  useRestApiOpenApi,
  useValidateOpenApiSpec,
  type OpenAPIContent,
  type Operation,
} from '@/api/resources/restApis';
import { MonitorIllustration } from '@/components/illustrations/MonitorIllustration';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { useFormatters } from '@/i18n/useFormatters';
import { OperationsList } from './OperationsList';

const messages = defineMessages({
  title: {
    id: 'develop.definition.DefinitionPanel.title',
    defaultMessage: 'Definition',
  },
  resourceCount: {
    id: 'develop.definition.DefinitionPanel.resourceCount',
    defaultMessage: '{count, plural, one {# resource} other {# resources}}',
  },
  lastUpdated: {
    id: 'develop.definition.DefinitionPanel.lastUpdated',
    defaultMessage: 'Last updated {relative}',
  },
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
    defaultMessage: 'Upload an OpenAPI or Swagger spec to document your API contract.',
  },
  addDefinition: {
    id: 'develop.definition.DefinitionPanel.addDefinition',
    defaultMessage: 'Import definition',
  },
  downloadLabel: {
    id: 'develop.definition.DefinitionPanel.downloadLabel',
    defaultMessage: 'Download Definition',
  },
  updateOpenApi: {
    id: 'develop.definition.DefinitionPanel.updateOpenApi',
    defaultMessage: 'Import Definition',
  },
  edit: {
    id: 'develop.definition.DefinitionPanel.edit',
    defaultMessage: 'Edit',
  },
  save: {
    id: 'develop.definition.DefinitionPanel.save',
    defaultMessage: 'Save',
  },
  confirmSaveTitle: {
    id: 'develop.definition.DefinitionPanel.confirmSave.title',
    defaultMessage: 'Update API',
  },
  confirmSaveDefinitionBody: {
    id: 'develop.definition.DefinitionPanel.confirmSave.definitionBody',
    defaultMessage: 'Are you sure you want to save the updated API Definition?',
  },
  confirmSaveResourcesBody: {
    id: 'develop.definition.DefinitionPanel.confirmSave.resourcesBody',
    defaultMessage: 'Are you sure you want to save the updated resources?',
  },
  confirmSaveAction: {
    id: 'develop.definition.DefinitionPanel.confirmSave.action',
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
  saveSpecInvalid: {
    id: 'develop.definition.DefinitionPanel.saveSpecInvalid',
    defaultMessage: 'Failed to save the specification. Fix the following issues:',
    description: 'Heading above spec validation errors shown when Save is clicked.',
  },
  saveValidationUnavailable: {
    id: 'develop.definition.DefinitionPanel.saveValidationUnavailable',
    defaultMessage: 'Spec validation is currently unavailable. Please try again.',
    description: 'Error shown when the validation service itself fails (network/auth error).',
  },
  discard: {
    id: 'develop.definition.DefinitionPanel.discard',
    defaultMessage: 'Discard',
  },
  fileReadError: {
    id: 'develop.definition.DefinitionPanel.fileReadError',
    defaultMessage: 'Failed to read the selected file.',
  },
  orDivider: {
    id: 'develop.definition.DefinitionPanel.orDivider',
    defaultMessage: 'Or',
    description: 'Separator between the URL input and the file upload button.',
  },
  editorLoading: {
    id: 'develop.definition.DefinitionPanel.editorLoading',
    defaultMessage: 'Loading editor…',
    description: 'Placeholder shown while Monaco editor initialises.',
  },
  addResource: {
    id: 'develop.definition.DefinitionPanel.addResource',
    defaultMessage: 'Add resource',
  },
  addResourceTitle: {
    id: 'develop.definition.DefinitionPanel.addResourceTitle',
    defaultMessage: 'Add Resource',
  },
  addResourceConfirm: {
    id: 'develop.definition.DefinitionPanel.addResourceConfirm',
    defaultMessage: 'Add',
  },
  addResourceCancel: {
    id: 'develop.definition.DefinitionPanel.addResourceCancel',
    defaultMessage: 'Cancel',
  },
  addResourcePathPlaceholder: {
    id: 'develop.definition.DefinitionPanel.addResourcePathPlaceholder',
    defaultMessage: '/resource/:id',
    description: 'Placeholder for the path input in the Add resource form.',
  },
  addResourceDescriptionPlaceholder: {
    id: 'develop.definition.DefinitionPanel.addResourceDescriptionPlaceholder',
    defaultMessage: 'Description (optional)',
    description: 'Placeholder for the description input in the Add resource form.',
  },
  pathLabel: {
    id: 'develop.definition.DefinitionPanel.pathLabel',
    defaultMessage: 'Path',
  },
  descriptionLabel: {
    id: 'develop.definition.DefinitionPanel.descriptionLabel',
    defaultMessage: 'Description',
  },
  methodLabel: {
    id: 'develop.definition.DefinitionPanel.methodLabel',
    defaultMessage: 'Method',
    description: 'Accessible label for the HTTP method select in the Add resource form.',
  },
});

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'] as const;
type HttpMethod = (typeof HTTP_METHODS)[number];

type OpenApiSpec = Record<string, unknown>;

const SUPPORTED_METHODS = [
  'get',
  'post',
  'put',
  'delete',
  'patch',
  'head',
  'options',
  'trace',
] as const;

const asRecord = (v: unknown): Record<string, unknown> | null =>
  typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : null;

const asText = (v: unknown): string | undefined => {
  if (typeof v !== 'string') return undefined;
  const t = v.trim();
  return t === '' ? undefined : t;
};

function extractOperations(spec: OpenApiSpec): Operation[] {
  const paths = asRecord(spec.paths);
  if (!paths) return [];
  return Object.entries(paths).flatMap(([path, pathItem]) => {
    const item = asRecord(pathItem);
    if (!item || !path.startsWith('/')) return [];
    return SUPPORTED_METHODS.flatMap((method): Operation[] => {
      const op = asRecord(item[method]);
      if (!op) return [];
      const name = asText(op.description) ?? asText(op.summary);
      const description = asText(op.description);
      return [{
        name,
        ...(description !== undefined ? { description } : {}),
        request: { method: method.toUpperCase() as Operation['request']['method'], path }
      }];
    });
  });
}

function parseSpec(text: string): OpenApiSpec | null {
  const trimmed = text.trimStart();
  if (!trimmed) return null;
  try {
    const doc: unknown = JSON.parse(trimmed);
    return doc && typeof doc === 'object' && !Array.isArray(doc) ? (doc as OpenApiSpec) : null;
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
  const { relativeTime } = useFormatters();
  const { params } = useConsoleScope();
  const restApiId = params.apiHandler;

  const fileInputRef = useRef<HTMLInputElement>(null);

  const openApiQuery = useRestApiOpenApi(restApiId);
  const apiQuery = useRestApi(restApiId);
  const putOpenApi = usePutRestApiOpenApi();
  const validateSpec = useValidateOpenApiSpec();

  const openApiError = openApiQuery.error as ApiError | null;
  const openApiData = openApiQuery.data as OpenAPIContent | undefined;

  const savedContent = openApiData?.content ?? '';
  const [editorText, setEditorText] = useState(savedContent);
  const detectFormat = (text: string): 'yaml' | 'json' =>
    text.trimStart().startsWith('{') ? 'json' : 'yaml';
  const [format, setFormat] = useState<'yaml' | 'json'>(() => detectFormat(savedContent));
  const [pendingFileName, setPendingFileName] = useState<string | null>(null);

  const [saveValidationErrors, setSaveValidationErrors] = useState<string[] | null>(null);
  const [isValidating, setIsValidating] = useState(false);
  const [isEditing, setIsEditing] = useState(false);

  // true = Monaco editor (Source), false = operations list.
  const [showSource, setShowSource] = useState(true);

  // Add-resource modal state
  const [showAddModal, setShowAddModal] = useState(false);
  const [newMethod, setNewMethod] = useState<HttpMethod>('GET');
  const [newPath, setNewPath] = useState('');
  const [newDescription, setNewDescription] = useState('');

  const [dialogOpen, setDialogOpen] = useState(false);
  const [confirmSaveOpen, setConfirmSaveOpen] = useState(false);
  const [specUrl, setSpecUrl] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  useEffect(() => {
    const content = openApiData?.content ?? '';
    const fmt = detectFormat(content);
    if (fmt === 'json' && content.trim()) {
      try {
        setEditorText(JSON.stringify(JSON.parse(content) as unknown, null, 2));
      } catch {
        setEditorText(content);
      }
    } else {
      setEditorText(content);
    }
    setPendingFileName(null);
    setFormat(fmt);
  }, [openApiData?.content]);

  useEffect(() => {
    setSaveValidationErrors(null);
  }, [editorText]);

  const isDirty = useMemo(() => {
    const savedParsed = parseSpec(savedContent);
    const editorParsed = parseSpec(editorText);
    if (!savedParsed && !editorParsed) return false;
    if (!savedParsed || !editorParsed) return true;
    return JSON.stringify(savedParsed) !== JSON.stringify(editorParsed);
  }, [savedContent, editorText]);

  const parsedSpec = useMemo(() => parseSpec(editorText), [editorText]);

  const extractedOperations = useMemo<Operation[]>(
    () => (parsedSpec ? extractOperations(parsedSpec) : []),
    [parsedSpec],
  );

  const definitionVersion = useMemo(() => {
    if (!parsedSpec) return 'OpenAPI';
    const openApiVersion = asText(parsedSpec.openapi);
    if (openApiVersion) return `OpenAPI ${openApiVersion}`;
    const swaggerVersion = asText(parsedSpec.swagger);
    return swaggerVersion ? `Swagger ${swaggerVersion}` : 'OpenAPI';
  }, [parsedSpec]);

  const savedInCurrentFormat = useMemo(() => {
    if (!savedContent) return savedContent;
    if (format === 'json') {
      try {
        const parsed = yaml.load(savedContent) as Record<string, unknown>;
        return JSON.stringify(parsed, null, 2);
      } catch {
        return savedContent;
      }
    }
    return savedContent;
  }, [savedContent, format]);

  const isSaving = isValidating || putOpenApi.isPending;

  const isSavingRef = useRef(false);
  useEffect(() => {
    isSavingRef.current = isSaving;
  }, [isSaving]);

  const handleFormatToggle = useCallback(
    (newFormat: 'yaml' | 'json') => {
      if (newFormat === format || isSaving) return;
      try {
        if (newFormat === 'json') {
          const parsed = yaml.load(editorText) as Record<string, unknown>;
          setEditorText(JSON.stringify(parsed, null, 2));
        } else {
          const parsed = JSON.parse(editorText) as Record<string, unknown>;
          setEditorText(yaml.dump(parsed));
        }
      } catch {
        // Switch language mode even if content is invalid.
      }
      setFormat(newFormat);
    },
    [format, editorText, isSaving],
  );

  const closeDialog = () => {
    setDialogOpen(false);
    setSpecUrl('');
    setFetchError(null);
  };

  const closeAddModal = () => {
    setShowAddModal(false);
    setNewPath('');
    setNewMethod('GET');
    setNewDescription('');
  };

  const applyFileContent = (file: File) => {
    void file
      .text()
      .then((text) => {
        const parsedSpec = parseSpec(text);
        setEditorText(parsedSpec ? yaml.dump(parsedSpec) : text);
        setPendingFileName(file.name.replace(/\.json$/i, '.yaml'));
        setFormat('yaml');
        setIsEditing(true);
      })
      .catch(() => {
        setFetchError(intl.formatMessage(messages.fileReadError));
      });
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    event.target.value = '';
    if (isSaving) return;
    applyFileContent(file);
    closeDialog();
  };

  const handleFetchSpec = async () => {
    const url = specUrl.trim();
    if (!url) return;
    setIsFetchingSpec(true);
    setFetchError(null);
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error('fetch failed');
      const reader = response.body?.getReader();
      if (!reader) throw new Error('fetch failed');
      const chunks: Uint8Array[] = [];
      let totalBytes = 0;
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        totalBytes += value.length;
        chunks.push(value);
      }
      const combined = new Uint8Array(totalBytes);
      let offset = 0;
      for (const chunk of chunks) {
        combined.set(chunk, offset);
        offset += chunk.length;
      }
      const text = new TextDecoder().decode(combined);
      if (isSavingRef.current) {
        setFetchError(intl.formatMessage(messages.dialogFetchError));
        return;
      }
      const parsedSpecContent = parseSpec(text);
      if (!parsedSpecContent) {
        setFetchError(intl.formatMessage(messages.dialogParseError));
        return;
      }
      setEditorText(yaml.dump(parsedSpecContent));
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

  const handleDownload = () => {
    let content = savedContent;
    let filename = 'api_definition.yaml';
    let mimeType = 'application/x-yaml';

    if (format === 'json') {
      filename = 'api_definition.json';
      mimeType = 'application/json';
      try {
        const parsed = yaml.load(savedContent) as Record<string, unknown>;
        content = JSON.stringify(parsed, null, 2);
      } catch {
        /* keep as-is */
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

    const content = editorText;
    const isEmpty = !content.trim();

    if (!isEmpty) {
      setIsValidating(true);
      setSaveValidationErrors(null);
      try {
        const validation = await validateSpec.mutateAsync(content);
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

    const mimeType = format === 'json' ? 'application/json' : 'application/x-yaml';
    const ext = format === 'json' ? '.json' : '.yaml';
    const baseName = (pendingFileName ?? 'openapi.yaml').replace(/\.(json|yaml|yml)$/i, '');
    const fileName = `${baseName}${ext}`;
    const blob = new Blob([content], { type: mimeType });
    const file = new File([blob], fileName, { type: mimeType });
    const formData = new FormData();
    formData.append('file', file);
    putOpenApi.mutate({ restApiId, formData }, { onSuccess: () => setIsEditing(false) });
  };

  /** Adds a new operation to the spec (editorText) and enters edit mode. */
  const handleAddOperation = () => {
    const raw = newPath.trim();
    if (!raw) return;
    const path = raw.startsWith('/') ? raw : `/${raw}`;
    const spec: OpenApiSpec = parseSpec(editorText) ?? {};
    const paths = (spec.paths as Record<string, Record<string, unknown>>) ?? {};
    const lm = newMethod.toLowerCase();
    if (!paths[path]) paths[path] = {};
    if (!(paths[path] as Record<string, unknown>)[lm]) {
      const description = newDescription.trim();
      (paths[path] as Record<string, unknown>)[lm] = {
        ...(description ? { description } : {}),
        responses: { '200': { description: 'OK' } },
      };
    }
    spec.paths = paths;
    const newText = format === 'json' ? JSON.stringify(spec, null, 2) : yaml.dump(spec);
    setEditorText(newText);
    setIsEditing(true);
    closeAddModal();
  };

  /** Removes an operation by its index in extractedOperations and enters edit mode. */
  const handleDeleteOperation = (index: number) => {
    const op = extractedOperations[index];
    if (!op) return;
    const { method, path } = op.request;
    const spec = parseSpec(editorText);
    if (!spec) return;
    const paths = spec.paths as Record<string, Record<string, unknown>> | undefined;
    if (!paths?.[path]) return;
    const lm = method.toLowerCase();
    delete (paths[path] as Record<string, unknown>)[lm];
    if (Object.keys(paths[path]).length === 0) delete paths[path];
    const newText = format === 'json' ? JSON.stringify(spec, null, 2) : yaml.dump(spec);
    setEditorText(newText);
    setIsEditing(true);
  };

  if (openApiQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  if (openApiError && openApiError.status !== 404) {
    return <ErrorState title={intl.formatMessage(messages.loadError)} />;
  }

  const hasSpec = Boolean(openApiData);

  const showSaveBar = isEditing || !hasSpec;

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
              <Stack spacing={2} sx={{ mt: 1 }}>
                <Stack alignItems="center" direction={{ sm: 'row', xs: 'column' }} spacing={1.5}>
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
                </Stack>

                <Divider>{intl.formatMessage(messages.orDivider)}</Divider>

                <Button
                  onClick={() => fileInputRef.current?.click()}
                  size="small"
                  sx={{ alignSelf: 'flex-start', whiteSpace: 'nowrap' }}
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

      {/* Save confirmation dialog */}
      <Dialog fullWidth maxWidth="xs" onClose={() => setConfirmSaveOpen(false)} open={confirmSaveOpen}>
        <DialogTitle>{intl.formatMessage(messages.confirmSaveTitle)}</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            {intl.formatMessage(
              showSource ? messages.confirmSaveDefinitionBody : messages.confirmSaveResourcesBody,
            )}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button color="secondary" onClick={() => setConfirmSaveOpen(false)} variant="outlined">
            {intl.formatMessage(messages.dialogCancel)}
          </Button>
          <Button
            onClick={() => {
              setConfirmSaveOpen(false);
              void handleSave();
            }}
            variant="contained"
          >
            {intl.formatMessage(messages.confirmSaveAction)}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Add Resource modal */}
      <Dialog fullWidth maxWidth="sm" onClose={closeAddModal} open={showAddModal}>
        <DialogTitle>{intl.formatMessage(messages.addResourceTitle)}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 2 }}>
            <FormControl fullWidth>
              <FormLabel>{intl.formatMessage(messages.methodLabel)}</FormLabel>
              <Select
                onChange={(e) => setNewMethod(e.target.value as HttpMethod)}
                size="small"
                value={newMethod}
              >
                {HTTP_METHODS.map((m) => (
                  <MenuItem key={m} value={m}>
                    {m}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="resource-path">
                {intl.formatMessage(messages.pathLabel)}
              </FormLabel>
              <TextField
                autoFocus
                fullWidth
                id="resource-path"
                onChange={(e) => {
                  const val = e.target.value;
                  setNewPath(val && !val.startsWith('/') ? `/${val}` : val);
                }}
                placeholder={intl.formatMessage(messages.addResourcePathPlaceholder)}
                sx={{ mt: 0.75 }}
                value={newPath}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="resource-description">
                {intl.formatMessage(messages.descriptionLabel)}
              </FormLabel>
              <TextField
                fullWidth
                id="resource-description"
                multiline
                onChange={(e) => setNewDescription(e.target.value)}
                placeholder={intl.formatMessage(messages.addResourceDescriptionPlaceholder)}
                rows={2}
                sx={{ mt: 0.75 }}
                value={newDescription}
              />
            </FormControl>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button color="secondary" onClick={closeAddModal} variant="outlined">
            {intl.formatMessage(messages.addResourceCancel)}
          </Button>
          <Button disabled={!newPath.trim()} onClick={handleAddOperation} variant="contained">
            {intl.formatMessage(messages.addResourceConfirm)}
          </Button>
        </DialogActions>
      </Dialog>

      {hasSpec || editorText ? (
        <Stack spacing={2}>
          {/* Definition summary and primary actions. */}
          <Stack
            direction={{ md: 'row', xs: 'column' }}
            spacing={2}
            sx={{ alignItems: { md: 'flex-end', xs: 'stretch' }, justifyContent: 'space-between' }}
          >
            <Stack spacing={1}>
              <Typography sx={{ fontWeight: 700 }} variant="h1">
                {intl.formatMessage(messages.title)}
              </Typography>
              <Stack
                direction="row"
                spacing={1.5}
                sx={{ alignItems: 'center', color: 'text.secondary', flexWrap: 'wrap' }}
              >
                <Chip label={definitionVersion} size="small" variant="outlined" />
                <Typography variant="body2">
                  {intl.formatMessage(messages.resourceCount, {
                    count: extractedOperations.length,
                  })}
                </Typography>
                {apiQuery.data?.updatedAt ? (
                  <>
                    <Typography aria-hidden variant="body2">
                      •
                    </Typography>
                    <Typography variant="body2">
                      {intl.formatMessage(messages.lastUpdated, {
                        relative: relativeTime(apiQuery.data.updatedAt),
                      })}
                    </Typography>
                  </>
                ) : null}
              </Stack>
            </Stack>

            <Stack direction={{ sm: 'row', xs: 'column' }} spacing={1}>
              <Button
                onClick={() => setShowSource(!showSource)}
                startIcon={<Braces size={16} />}
                sx={{ textTransform: 'none' }}
                variant="outlined"
              >
                {showSource ? 'View Resources' : 'View Definition'}
              </Button>
              <ButtonGroup aria-label="Definition file actions" variant="outlined">
                <Tooltip title={intl.formatMessage(messages.updateOpenApi)}>
                  <Button
                    aria-label={intl.formatMessage(messages.updateOpenApi)}
                    disabled={isSaving}
                    onClick={() => setDialogOpen(true)}
                    sx={{ minWidth: 40, px: 1 }}
                  >
                    <Upload size={18} />
                  </Button>
                </Tooltip>
                {hasSpec ? (
                  <Tooltip title={intl.formatMessage(messages.downloadLabel)}>
                    <Button
                      aria-label={intl.formatMessage(messages.downloadLabel)}
                      onClick={handleDownload}
                      sx={{ minWidth: 40, px: 1 }}
                    >
                      <Download size={18} />
                    </Button>
                  </Tooltip>
                ) : null}
              </ButtonGroup>
            </Stack>
          </Stack>

          {/* Single panel with Spec / Operations toggle */}
          <Box
            sx={{
              bgcolor: 'background.paper',
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              display: 'flex',
              flexDirection: 'column',
              height: 'clamp(480px, calc(100vh - 280px), 800px)',
              overflow: 'hidden',
            }}
          >
            {/* Panel header: source controls or the existing Add Resource action. */}
            <Box
              sx={{
                alignItems: 'center',
                borderBottom: '1px solid',
                borderColor: 'divider',
                display: 'flex',
                flexShrink: 0,
                justifyContent: 'flex-end',
                px: 2,
                py: 1,
              }}
            >
              {/* YAML/JSON toggle + Edit button (source view) OR Add Resource button (operations view) */}
              {showSource ? (
                <Stack alignItems="center" direction="row" spacing={1}>
                  <ToggleButtonGroup
                    aria-label={intl.formatMessage(messages.formatLabel)}
                    color="primary"
                    disabled={isSaving}
                    exclusive
                    onChange={(_event, next: 'yaml' | 'json' | null) => {
                      if (next !== null) handleFormatToggle(next);
                    }}
                    size="small"
                    sx={{ '& .MuiToggleButton-sizeSmall': { py: '3px' } }}
                    value={format}
                  >
                    <ToggleButton value="yaml">YAML</ToggleButton>
                    <ToggleButton value="json">JSON</ToggleButton>
                  </ToggleButtonGroup>
                  {hasSpec && !isEditing && (
                    <Button
                      onClick={() => setIsEditing(true)}
                      size="small"
                      startIcon={<Pencil size={16} />}
                      variant="outlined"
                    >
                      <FormattedMessage {...messages.edit} />
                    </Button>
                  )}
                </Stack>
              ) : (
                <Button
                  onClick={() => setShowAddModal(true)}
                  size="small"
                  startIcon={<Plus size={16} />}
                  variant="outlined"
                >
                  {intl.formatMessage(messages.addResource)}
                </Button>
              )}
            </Box>

            {/* Panel content */}
            <Box sx={{ display: 'flex', flex: 1, flexDirection: 'column', minHeight: 0 }}>
              {showSource ? (
                /* Raw spec editor */
                <Box sx={{ flex: 1, minHeight: 0, p: 1 }}>
                  <Editor
                    height="100%"
                    language={format}
                    loading={<LoadingState label={intl.formatMessage(messages.editorLoading)} />}
                    onChange={(value) => setEditorText(value ?? '')}
                    options={{
                      automaticLayout: true,
                      fontSize: 12,
                      lineHeight: 20,
                      minimap: { enabled: false },
                      readOnly: isSaving || (hasSpec && !isEditing),
                      scrollBeyondLastLine: false,
                      wordWrap: 'on',
                    }}
                    theme="vs-dark"
                    value={editorText}
                  />
                </Box>
              ) : (
                /* Operations list */
                <OperationsList
                  canParse={parsedSpec !== null}
                  onDelete={handleDeleteOperation}
                  operations={extractedOperations}
                  showDelete
                />
              )}
            </Box>

            {/* Save / Reset bar */}
            {showSaveBar && (
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
                      if (hasSpec) {
                        setEditorText(savedInCurrentFormat);
                        setIsEditing(false);
                      } else {
                        setEditorText('');
                      }
                      setPendingFileName(null);
                      setSaveValidationErrors(null);
                    }}
                    size="small"
                    variant="outlined"
                  >
                    {hasSpec
                      ? intl.formatMessage(messages.reset)
                      : intl.formatMessage(messages.discard)}
                  </Button>
                  <Button
                    disabled={!isDirty || (!editorText.trim() && hasSpec)}
                    loading={isSaving}
                    onClick={() => setConfirmSaveOpen(true)}
                    size="small"
                    variant="contained"
                  >
                    {intl.formatMessage(messages.save)}
                  </Button>
                </Box>
              </Box>
            )}
          </Box>
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
            <Button
              disabled={isSaving}
              onClick={() => setDialogOpen(true)}
              startIcon={<Upload size={16} />}
              sx={{ mt: 1 }}
              variant="contained"
            >
              {intl.formatMessage(messages.addDefinition)}
            </Button>
          </Stack>
        </Box>
      )}
    </>
  );
}
