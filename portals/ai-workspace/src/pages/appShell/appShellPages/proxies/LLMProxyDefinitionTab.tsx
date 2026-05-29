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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { PenLine } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import Editor from '@monaco-editor/react';
import YAML from 'yaml';
import { useProxy } from '../../../../contexts/proxy';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import NoData from '../../../../assets/images/NoData.svg';
import { logger } from '../../../../utils/logger';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';

type OpenApiSpec = Record<string, unknown>;

type ResourceItem = {
  method: string;
  path: string;
  summary?: string;
};

const HTTP_METHODS = new Set([
  'get',
  'post',
  'put',
  'delete',
  'patch',
  'head',
  'options',
  'trace',
]);

function parseOpenApiSpec(text: string): OpenApiSpec | null {
  if (!text.trim()) return null;
  try {
    const jsonSpec = JSON.parse(text);
    return jsonSpec && typeof jsonSpec === 'object'
      ? (jsonSpec as OpenApiSpec)
      : null;
  } catch {
    try {
      const yamlSpec = YAML.parse(text);
      return yamlSpec && typeof yamlSpec === 'object'
        ? (yamlSpec as OpenApiSpec)
        : null;
    } catch (parseError) {
      logger.error('Failed to parse proxy OpenAPI spec:', parseError);
      return null;
    }
  }
}

function extractResourcesFromSpec(spec: OpenApiSpec | null): ResourceItem[] {
  if (!spec) return [];
  const paths = spec.paths;
  if (!paths || typeof paths !== 'object') return [];

  const extracted: ResourceItem[] = [];
  Object.entries(paths as Record<string, unknown>).forEach(
    ([path, rawOperations]) => {
      if (!rawOperations || typeof rawOperations !== 'object') return;

      Object.entries(rawOperations as Record<string, unknown>).forEach(
        ([method, operation]) => {
          if (!HTTP_METHODS.has(method.toLowerCase())) return;
          const operationObject =
            operation && typeof operation === 'object'
              ? (operation as Record<string, unknown>)
              : {};

          extracted.push({
            method: method.toUpperCase(),
            path,
            summary:
              (operationObject.summary as string | undefined) ||
              (operationObject.description as string | undefined),
          });
        }
      );
    }
  );

  extracted.sort((a, b) => {
    const pathCompare = a.path.localeCompare(b.path);
    return pathCompare !== 0 ? pathCompare : a.method.localeCompare(b.method);
  });

  return extracted;
}

function validateOpenApiText(
  text: string,
  parsedSpec: OpenApiSpec | null
): string | null {
  if (!text.trim()) {
    return 'OpenAPI definition cannot be empty.';
  }
  if (!parsedSpec) {
    return 'Failed to parse the OpenAPI definition. Use valid JSON or YAML.';
  }

  const openApiVersion = parsedSpec.openapi;
  const swaggerVersion = parsedSpec.swagger;
  if (
    typeof openApiVersion !== 'string' &&
    typeof swaggerVersion !== 'string'
  ) {
    return 'Definition must include an `openapi` or `swagger` version.';
  }

  const paths = parsedSpec.paths;
  if (!paths || typeof paths !== 'object' || Array.isArray(paths)) {
    return 'Definition must include a valid `paths` object.';
  }

  return null;
}

export default function LLMProxyDefinitionTab() {
  const { proxy, setLocalProxy } = useProxy();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [specUrl, setSpecUrl] = useState('');
  const [editorText, setEditorText] = useState('');
  const [isFetchingSpec, setIsFetchingSpec] = useState(false);
  const [updateSpecModalOpen, setUpdateSpecModalOpen] = useState(false);
  const showSnackbar = useAIWorkspaceSnackbar();

  useEffect(() => {
    setEditorText(proxy?.openapi || '');
  }, [proxy?.id, proxy?.openapi]);

  const parsedSpec = useMemo(() => parseOpenApiSpec(editorText), [editorText]);
  const editorLanguage = useMemo(() => {
    const trimmed = editorText.trimStart();
    if (!trimmed) return 'yaml';
    return trimmed.startsWith('{') || trimmed.startsWith('[') ? 'json' : 'yaml';
  }, [editorText]);
  const validationError = useMemo(
    () => validateOpenApiText(editorText, parsedSpec),
    [editorText, parsedSpec]
  );
  const resources = useMemo(
    () => extractResourcesFromSpec(parsedSpec),
    [parsedSpec]
  );
  const hasDefinition = editorText.trim().length > 0;

  const updateEditorText = (nextText: string) => {
    setEditorText(nextText);
    setLocalProxy((prev) => (prev ? { ...prev, openapi: nextText } : prev));
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  const applySpecificationFromText = (text: string, sourceName?: string) => {
    const parsed = parseOpenApiSpec(text);
    const nextValidationError = validateOpenApiText(text, parsed);
    if (nextValidationError) {
      showSnackbar(nextValidationError, 'error');
      return;
    }

    updateEditorText(text);
    if (sourceName) {
      setSpecUrl(sourceName);
    }
    showSnackbar('Loaded OpenAPI definition into the editor.', 'success');
  };

  const handleFetchAndClose = async (url: string) => {
    const nextUrl = url.trim();
    if (!nextUrl) {
      showSnackbar('Enter a valid OpenAPI URL first.', 'error');
      return;
    }

    setIsFetchingSpec(true);
    try {
      const response = await fetch(nextUrl);
      if (!response.ok) {
        throw new Error(`Unable to fetch specification from ${nextUrl}`);
      }
      const openApiText = await response.text();
      applySpecificationFromText(openApiText, nextUrl);
      setUpdateSpecModalOpen(false);
      setSpecUrl('');
    } catch (fetchError) {
      logger.error('Failed to import specification from URL:', fetchError);
      showSnackbar('Failed to import specification from URL.', 'error');
    } finally {
      setIsFetchingSpec(false);
    }
  };

  const handleFileChange: React.ChangeEventHandler<HTMLInputElement> = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;

    void (async () => {
      try {
        const openApiText = await file.text();
        applySpecificationFromText(openApiText, file.name);
        setUpdateSpecModalOpen(false);
        setSpecUrl('');
      } catch (readError) {
        logger.error('Failed to read specification file:', readError);
        showSnackbar('Failed to import specification file.', 'error');
      }
    })();

    e.target.value = '';
  };

  return (
    <Stack spacing={2}>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button
          variant="outlined"
          size="small"
          startIcon={<PenLine size={16} />}
          onClick={() => setUpdateSpecModalOpen(true)}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyDefinitionTab.update.openapi.definition"
            defaultMessage={'Update OpenAPI Definition'}
          />
        </Button>
      </Box>

      <Dialog
        open={updateSpecModalOpen}
        onClose={() => {
          setUpdateSpecModalOpen(false);
          setSpecUrl('');
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
                    disabled={isFetchingSpec}
                    onClick={() => {
                      void handleFetchAndClose(specUrl);
                    }}
                  >
                    {isFetchingSpec ? 'Fetching....' : 'Fetch specification'}
                  </Button>
                </Box>
              </Stack>
            </FormControl>
            <Divider sx={{ my: 2 }}>Or</Divider>
            <Button variant="outlined" fullWidth onClick={handleUploadClick}>
              Upload Your Specification
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              hidden
              accept=".json,.yaml,.yml"
              onChange={handleFileChange}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => {
              setUpdateSpecModalOpen(false);
              setSpecUrl('');
            }}
          >
            Cancel
          </Button>
        </DialogActions>
      </Dialog>

      {validationError && hasDefinition ? (
        <Alert severity="error">{validationError}</Alert>
      ) : null}

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 7 }}>
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              p: 2,
              bgcolor: 'background.paper',
            }}
          >
            <Typography variant="h6" sx={{ mb: 1, fontWeight: 600 }}>
              Swagger (OpenAPI) Editor
            </Typography>
            <Box
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
                overflow: 'hidden',
                position: 'relative',
                minHeight: { xs: 360, md: 540 },
                bgcolor: '#1e1e1e',
              }}
            >
              {!editorText.trim() ? (
                <Typography
                  variant="body2"
                  sx={{
                    position: 'absolute',
                    top: 10,
                    left: 14,
                    zIndex: 1,
                    pointerEvents: 'none',
                    color: 'rgba(255, 255, 255, 0.7)',
                  }}
                >
                  Paste or edit your OpenAPI JSON/YAML definition here.
                </Typography>
              ) : null}
              <Editor
                height="540px"
                language={editorLanguage}
                value={editorText}
                onChange={(value) => {
                  updateEditorText(value || '');
                }}
                options={{
                  minimap: { enabled: false },
                  scrollBeyondLastLine: false,
                  fontSize: 12,
                  lineHeight: 20,
                  wordWrap: 'on',
                  automaticLayout: true,
                }}
                theme="vs-dark"
                loading={<Box sx={{ p: 2 }}>Loading editor...</Box>}
              />
            </Box>
          </Box>
        </Grid>
        <Grid size={{ xs: 12, md: 5 }}>
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1,
              p: 2,
              bgcolor: 'background.paper',
              minHeight: '100%',
            }}
          >
            <Typography variant="h6" sx={{ mb: 1, fontWeight: 600 }}>
              Resources ({resources.length})
            </Typography>
            {!hasDefinition ? (
              <Stack
                spacing={1}
                alignItems="center"
                justifyContent="center"
                sx={{ py: 6, textAlign: 'center' }}
              >
                <Box
                  component="img"
                  src={NoData}
                  alt="No available resources"
                  sx={{ width: 80, maxWidth: '80%' }}
                />
                <Typography variant="body2" color="text.secondary">
                  No OpenAPI definition loaded.
                </Typography>
              </Stack>
            ) : validationError ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                Fix validation errors in the editor to generate the resource
                list.
              </Typography>
            ) : resources.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                No API operations were found under `paths`.
              </Typography>
            ) : (
              <Box
                sx={{
                  maxHeight: { xs: 300, md: 540 },
                  overflowY: 'auto',
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  bgcolor: 'background.paper',
                  pl: 2,
                  pr: 2,
                  pt: 2,
                }}
              >
                {parsedSpec ? (
                  <SwaggerSpecViewer
                    spec={parsedSpec}
                    disableTryOutBtn
                    hideInfoSection
                    hideServers
                    hideAuthorizeButton
                    hideTagHeaders
                    docExpansion="list"
                    defaultModelsExpandDepth={-1}
                    displayRequestDuration
                  />
                ) : null}
              </Box>
            )}
          </Box>
        </Grid>
      </Grid>
    </Stack>
  );
}
