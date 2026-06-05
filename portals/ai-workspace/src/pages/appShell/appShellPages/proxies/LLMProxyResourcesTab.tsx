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

import React, { useMemo, useRef } from 'react';
import { Box, Button, Stack, Typography } from '@wso2/oxygen-ui';
import { Upload } from '@wso2/oxygen-ui-icons-react';
import YAML from 'yaml';
import { useProxy } from '../../../../contexts/proxy';
import { logger } from '../../../../utils/logger';
import NoData from '../../../../assets/images/NoData.svg';
import { FormattedMessage } from 'react-intl';
import SwaggerSpecViewer from '../../../../Components/SwaggerSpecViewer';

// ─── Shared helpers ──────────────────────────────────────────────────────────

type ResourceItem = {
  method: string;
  path: string;
};

type OpenApiSpec = Record<string, unknown>;

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
      extracted.push({
        method: methodKey.toUpperCase(),
        path,
      });
    });
  });

  extracted.sort((a, b) => {
    const p = a.path.localeCompare(b.path);
    return p !== 0 ? p : a.method.localeCompare(b.method);
  });
  return extracted;
}

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
    } catch (err) {
      logger.error('Failed to parse OpenAPI spec:', err);
      return null;
    }
  }
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * Resources tab – import/paste an OpenAPI spec and display the parsed
 * resources.
 */
export default function LLMProxyResourcesTab() {
  const { proxy, setLocalProxy } = useProxy();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const openApiSpecText = proxy?.openapi || '';
  const parsedOpenApiSpec = useMemo(
    () => parseOpenApiSpec(openApiSpecText),
    [openApiSpecText]
  );
  const resources = useMemo(
    () => (parsedOpenApiSpec ? extractResourcesFromSpecJson(parsedOpenApiSpec) : []),
    [parsedOpenApiSpec]
  );
  const hasOpenApiSpecText = openApiSpecText.trim().length > 0;

  // ── Spec import handlers ────────────────────────────────────────────────

  const handleSpecTextChange = (text: string) => {
    setLocalProxy((prev) => (prev ? { ...prev, openapi: text } : prev));
  };

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    const validExtensions = ['.json', '.yaml', '.yml'];
    const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase();
    if (!validExtensions.includes(ext)) {
      logger.error('Invalid file type – only JSON/YAML allowed.');
      return;
    }

    const reader = new FileReader();
    reader.onload = (e) => {
      const text = (e.target?.result as string) ?? '';
      handleSpecTextChange(text);
    };
    reader.readAsText(file);

    // Reset so the same file can be re-selected
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  // ── Render ──────────────────────────────────────────────────────────────

  return (
    <Stack spacing={3}>
      {/* Import section */}
      <Box>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1,
            flexWrap: 'wrap',
            mb: 1,
          }}
        >
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyResourcesTab.openapi.specification"
              defaultMessage={'OpenAPI Resources '}
            />
          </Typography>

          <Stack direction="row" spacing={1}>
            <Button
              variant="outlined"
              size="small"
              startIcon={<Upload size={16} />}
              onClick={() => fileInputRef.current?.click()}
            >
              Import from file
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,.yaml,.yml"
              hidden
              onChange={handleFileUpload}
            />
          </Stack>
        </Box>

        <Typography variant="body2" color="text.secondary">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyResourcesTab.import.or.paste.an.openapi.spec.json.yaml.to.define.the.resources.available.thro"
            defaultMessage={
              'Import an OpenAPI spec (JSON / YAML) to define the resources available through this proxy.'
            }
          />
        </Typography>
      </Box>

      {/* Parsed resources */}
      <Box>
        <Typography variant="body2" sx={{ mb: 1 }}>
          Resources ({resources.length})
        </Typography>

        {!hasOpenApiSpecText ? (
          <Stack alignItems="center" spacing={1} sx={{ py: 4 }}>
            <Box
              component="img"
              src={NoData}
              alt="No resources"
              sx={{ width: 70 }}
            />
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.proxies.LLMProxyResourcesTab.no.resources.found.import.or.paste.an.openapi.spec.above"
                defaultMessage={
                  'No resources available. Import or paste an OpenAPI specification above.'
                }
              />
            </Typography>
          </Stack>
        ) : !parsedOpenApiSpec ? (
          <Typography variant="body2" color="error" sx={{ py: 2 }}>
            Failed to parse the OpenAPI specification from proxy response.
          </Typography>
        ) : (
          <Box
            sx={{
              maxHeight: 360,
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
            <SwaggerSpecViewer
              spec={parsedOpenApiSpec}
              hideInfoSection
              hideServers
              hideAuthorizeButton
              disableTryOutBtn
              disableResponseSection
              hideTagHeaders
              docExpansion="list"
              defaultModelsExpandDepth={-1}
              displayRequestDuration
            />
          </Box>
        )}
      </Box>
    </Stack>
  );
}
