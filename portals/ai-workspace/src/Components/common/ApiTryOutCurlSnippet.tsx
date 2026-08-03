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

import { useMemo, useState } from 'react';
import {
  Box,
  CodeBlock,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { CheckCircle2, Copy, FlaskConical } from '@wso2/oxygen-ui-icons-react';
import { formatPrefixedKey } from '../../utils/apiKeyAuthDisplay';

interface EndpointOption {
  path: string;
  body: Record<string, unknown>;
  extraHeaders?: Record<string, string>;
}

interface TemplateEndpoint {
  /** Used only when the provider has no model configured yet. */
  defaultModel: string;
  build: (model: string) => EndpointOption;
}

const SAMPLE_PROMPT = 'Say hello!';

/**
 * Model ids appear in the request path for Gemini and AWS Bedrock. Percent-encode
 * the segment but keep ':' literal — Bedrock model ids embed it
 * (`us.meta.llama3-3-70b-instruct-v1:0`) and the gateway route matches it raw.
 */
const encodeModelPathSegment = (model: string): string =>
  encodeURIComponent(model).replace(/%3A/gi, ':');

/**
 * Paths, query parameters and request bodies here must match the OpenAPI spec each
 * default template ships with (`llm-provider-specs/<template>/openapi.yaml`), because
 * the gateway only routes the operations present in that spec. Keys are the template
 * ids from `platform-api/resources/default-llm-provider-templates`.
 */
const TEMPLATE_ENDPOINTS: Record<string, TemplateEndpoint> = {
  openai: {
    defaultModel: 'gpt-4o-mini',
    build: (model) => ({
      path: '/chat/completions',
      body: {
        model,
        messages: [{ role: 'user', content: SAMPLE_PROMPT }],
      },
    }),
  },
  mistralai: {
    defaultModel: 'mistral-large-latest',
    build: (model) => ({
      path: '/v1/chat/completions',
      body: {
        model,
        messages: [{ role: 'user', content: SAMPLE_PROMPT }],
      },
    }),
  },
  anthropic: {
    defaultModel: 'claude-sonnet-4-6',
    build: (model) => ({
      path: '/v1/messages',
      extraHeaders: { 'anthropic-version': '2023-06-01' },
      body: {
        model,
        max_tokens: 1024,
        messages: [{ role: 'user', content: SAMPLE_PROMPT }],
      },
    }),
  },
  gemini: {
    defaultModel: 'gemini-2.5-flash',
    build: (model) => ({
      path: `/v1beta/models/${encodeModelPathSegment(model)}:generateContent`,
      body: {
        contents: [{ parts: [{ text: SAMPLE_PROMPT }] }],
      },
    }),
  },
  'azure-openai': {
    defaultModel: 'gpt-4o',
    build: (model) => ({
      path: '/openai/responses?api-version=2025-04-01-preview',
      body: {
        model,
        input: SAMPLE_PROMPT,
      },
    }),
  },
  'azureai-foundry': {
    defaultModel: 'gpt-4o',
    build: (model) => ({
      path: '/models/chat/completions?api-version=2024-05-01-preview',
      body: {
        model,
        messages: [{ role: 'user', content: SAMPLE_PROMPT }],
      },
    }),
  },
  awsbedrock: {
    defaultModel: 'amazon.titan-text-express-v1',
    build: (model) => ({
      path: `/model/${encodeModelPathSegment(model)}/converse`,
      body: {
        messages: [{ role: 'user', content: [{ text: SAMPLE_PROMPT }] }],
      },
    }),
  },
};

const FALLBACK_ENDPOINT: TemplateEndpoint = {
  defaultModel: '<model-id>',
  build: (model) => ({
    path: '/chat/completions',
    body: {
      model,
      messages: [{ role: 'user', content: SAMPLE_PROMPT }],
    },
  }),
};

interface Props {
  apiKey: string;
  gatewayUrl: string;
  apiKeyHeaderName: string;
  apiKeyLocation: 'header' | 'query';
  apiKeyValuePrefix?: string;
  providerTemplate?: string | null;
  /** Model ids configured on the provider; the first one is used in the sample. */
  models?: string[];
}

/**
 * Close the single-quoted shell argument, emit an escaped quote, reopen it — the
 * standard '\'' idiom. Model ids and header values reach the snippet from provider
 * config, so a value containing a single quote would otherwise end the -d/-H argument
 * early and hand the rest of the JSON to the shell.
 */
const escapeSingleQuoted = (value: string): string => value.replace(/'/g, `'\\''`);

function buildCurlCommand(
  url: string,
  apiKeyHeaderName: string,
  apiKeyLocation: 'header' | 'query',
  apiKey: string,
  body: Record<string, unknown>,
  extraHeaders?: Record<string, string>
): string {
  const bodyJson = JSON.stringify(body, null, 2);
  const fullUrl =
    apiKeyLocation === 'query'
      ? `${url}${url.includes('?') ? '&' : '?'}${apiKeyHeaderName}=${encodeURIComponent(apiKey)}`
      : url;

  const lines = [
    `curl -X POST "${fullUrl}" \\`,
    `  -H "Content-Type: application/json" \\`,
  ];

  if (apiKeyLocation === 'header') {
    lines.push(`  -H "${apiKeyHeaderName}: ${apiKey}" \\`);
  }

  if (extraHeaders) {
    Object.entries(extraHeaders).forEach(([key, value]) => {
      lines.push(`  -H '${escapeSingleQuoted(`${key}: ${value}`)}' \\`);
    });
  }

  lines.push(`  -d '${escapeSingleQuoted(bodyJson)}'`);
  return lines.join('\n');
}

export default function ApiTryOutCurlSnippet({
  apiKey,
  gatewayUrl,
  apiKeyHeaderName,
  apiKeyLocation,
  apiKeyValuePrefix,
  providerTemplate,
  models,
}: Props) {
  const endpoint = useMemo(() => {
    const key = providerTemplate?.trim().toLowerCase() ?? '';
    const template = TEMPLATE_ENDPOINTS[key] ?? FALLBACK_ENDPOINT;
    const configuredModel = models
      ?.map((model) => model.trim())
      .find((model) => model.length > 0);
    return template.build(configuredModel || template.defaultModel);
  }, [models, providerTemplate]);

  const [copied, setCopied] = useState(false);

  const curlCommand = useMemo(() => {
    const base = gatewayUrl ? gatewayUrl.replace(/\/+$/, '') : '<gateway-url>';
    const url = `${base}${endpoint.path}`;
    const keyValue = formatPrefixedKey(apiKeyValuePrefix ?? '', apiKey);
    return buildCurlCommand(url, apiKeyHeaderName, apiKeyLocation, keyValue, endpoint.body, endpoint.extraHeaders);
  }, [apiKey, apiKeyHeaderName, apiKeyLocation, apiKeyValuePrefix, gatewayUrl, endpoint]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(curlCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard not available
    }
  };

  return (
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={1} alignItems="center">
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 32,
            height: 32,
            borderRadius: '50%',
            bgcolor: '#db621e',
            flexShrink: 0,
          }}
        >
          <FlaskConical size={16} color="#fff" />
        </Box>
        <Stack spacing={0.25}>
          <Typography variant="subtitle2" fontWeight={600}>
            Try it out
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Use the <Box component="span" sx={{ fontWeight: 700 }}>Sample Curl</Box> command below to test this
            endpoint with the generated API key.
          </Typography>
        </Stack>
      </Stack>
      <Box sx={{ position: 'relative' }}>
        <CodeBlock code={curlCommand} language="bash" />
        <Tooltip title={copied ? 'Copied!' : 'Copy command'}>
          <IconButton
            size="small"
            onClick={() => {
              void handleCopy();
            }}
            sx={{
              position: 'absolute',
              top: 6,
              right: 6,
              color: copied ? 'success.main' : 'grey.400',
              '&:hover': { bgcolor: 'grey.700', color: 'grey.100' },
            }}
          >
            {copied ? <CheckCircle2 size={14} /> : <Copy size={14} />}
          </IconButton>
        </Tooltip>
      </Box>
    </Stack>
  );
}
