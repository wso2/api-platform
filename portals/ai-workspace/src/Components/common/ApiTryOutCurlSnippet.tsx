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

interface EndpointOption {
  path: string;
  body: Record<string, unknown>;
  extraHeaders?: Record<string, string>;
}

const TEMPLATE_ENDPOINTS: Record<string, EndpointOption> = {
  openai: {
    path: '/chat/completions',
    body: {
      model: 'gpt-4o-mini',
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  mistralai: {
    path: '/v1/chat/completions',
    body: {
      model: 'mistral-large-latest',
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  anthropic: {
    path: '/v1/messages',
    extraHeaders: { 'anthropic-version': '2023-06-01' },
    body: {
      model: 'claude-sonnet-4-6',
      max_tokens: 1024,
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  gemini: {
    path: '/v1beta/models/gemini-2.5-flash:generateContent',
    body: {
      contents: [{ parts: [{ text: 'Say hello!' }] }],
    },
  },
  'azure-openai': {
    path: '/chat/completions?api-version=2024-02-01',
    body: {
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  'azureai-foundry': {
    path: '/chat/completions?api-version=2024-02-01',
    body: {
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  meta: {
    path: '/chat/completions',
    body: {
      model: 'us.meta.llama3-3-70b-instruct-v1:0',
      messages: [{ role: 'user', content: 'Say hello!' }],
    },
  },
  awsbedrock: {
    path: '/model/amazon.titan-text-express-v1/invoke',
    body: {
      inputText: 'Say hello!',
      textGenerationConfig: { maxTokenCount: 256, temperature: 0.7 },
    },
  },
  'aws-bedrock': {
    path: '/model/amazon.titan-text-express-v1/invoke',
    body: {
      inputText: 'Say hello!',
      textGenerationConfig: { maxTokenCount: 256, temperature: 0.7 },
    },
  },
  'google-vertex': {
    path: '/projects/{project}/locations/us-central1/publishers/google/models/gemini-2.0-flash:generateContent',
    body: {
      contents: [{ parts: [{ text: 'Say hello!' }] }],
    },
  },
};

const FALLBACK_ENDPOINT: EndpointOption = {
  path: '/chat/completions',
  body: {
    messages: [{ role: 'user', content: 'Say hello!' }],
  },
};

interface Props {
  apiKey: string;
  gatewayUrl: string;
  apiKeyHeaderName: string;
  apiKeyLocation: 'header' | 'query';
  providerTemplate?: string | null;
}

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
      ? `${url}${url.includes('?') ? '&' : '?'}${apiKeyHeaderName}=${apiKey}`
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
      lines.push(`  -H '${key}: ${value}' \\`);
    });
  }

  lines.push(`  -d '${bodyJson}'`);
  return lines.join('\n');
}

export default function ApiTryOutCurlSnippet({
  apiKey,
  gatewayUrl,
  apiKeyHeaderName,
  apiKeyLocation,
  providerTemplate,
}: Props) {
  const endpoint = useMemo(() => {
    const key = providerTemplate?.trim().toLowerCase() ?? '';
    return TEMPLATE_ENDPOINTS[key] ?? FALLBACK_ENDPOINT;
  }, [providerTemplate]);

  const [copied, setCopied] = useState(false);

  const curlCommand = useMemo(() => {
    const base = gatewayUrl ? gatewayUrl.replace(/\/+$/, '') : '<gateway-url>';
    const url = `${base}${endpoint.path}`;
    return buildCurlCommand(url, apiKeyHeaderName, apiKeyLocation, apiKey, endpoint.body, endpoint.extraHeaders);
  }, [apiKey, apiKeyHeaderName, apiKeyLocation, gatewayUrl, endpoint]);

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
