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

import { CreateLLMProviderRequest, LLMProvider, ModelProvider } from './types';

const MODEL_CATALOG_BY_KEY: Record<string, string[]> = {
  meta: [
    'us.meta.llama3-3-70b-instruct-v1:0',
    'us.meta.llama4-maverick-17b-instruct-v1:0',
  ],
  openai: ['gpt-4o-mini', 'gpt-4.1-mini', 'o4-mini'],
  anthropic: ['claude-3.5-sonnet', 'claude-3-opus'],
  'google-vertex': ['gemini-1.5-pro', 'gemini-1.5-flash'],
  'aws-bedrock': ['amazon.titan-text-premier', 'anthropic.claude-v2'],
  mistralai: [
    'mistral-large-latest',
    'mistral-small-latest',
    'open-mixtral-8x22b',
  ],
};

const TEMPLATE_TO_MODEL_KEY: Record<string, string> = {
  meta: 'meta',
  openai: 'openai',
  'azure-openai': 'openai',
  anthropic: 'anthropic',
  gemini: 'google-vertex',
  'google-vertex': 'google-vertex',
  awsbedrock: 'aws-bedrock',
  'aws-bedrock': 'aws-bedrock',
  mistralai: 'mistralai',
};

const TEMPLATE_PROVIDER_NAME_BY_ID: Record<string, string> = {
  meta: 'Meta',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  mistralai: 'Mistral',
  'azure-openai': 'Azure OpenAI',
  'azureai-foundry': 'Azure AI Foundry',
  awsbedrock: 'AWS Bedrock',
  'aws-bedrock': 'AWS Bedrock',
  'google-vertex': 'Google Vertex',
};

function buildModelProvidersFromTemplate(templateId?: string): ModelProvider[] {
  const normalizedTemplateId = templateId?.trim().toLowerCase();
  if (!normalizedTemplateId) {
    return [];
  }

  const modelKey = TEMPLATE_TO_MODEL_KEY[normalizedTemplateId];
  if (!modelKey) {
    return [];
  }

  const modelIds = MODEL_CATALOG_BY_KEY[modelKey] ?? [];
  if (!modelIds.length) {
    return [];
  }

  return [
    {
      id: normalizedTemplateId,
      displayName:
        TEMPLATE_PROVIDER_NAME_BY_ID[normalizedTemplateId] ??
        normalizedTemplateId,
      models: modelIds.map((modelId) => ({
        id: modelId,
        displayName: modelId,
      })),
    },
  ];
}

/**
 * Temporary utility to convert CreateLLMProviderRequest to full LLMProvider
 * by adding hardcoded default values for missing fields.
 *
 * TODO: Remove this utility once the backend supports partial creation
 */
export function buildFullProviderRequest(
  request: CreateLLMProviderRequest
): LLMProvider {
  const toProviderId = (displayName: string): string =>
    displayName
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '');

  const resolvedId = request.id?.trim() || toProviderId(request.displayName);

  return {
    // Use values from the request
    id: resolvedId,
    displayName: request.displayName,
    description: request.description,
    version: request.version,
    vhost: request.vhost,
    context: request.context?.trim() || '/',
    template: request.template,
    upstream: request.upstream,
    accessControl: request.accessControl,
    policies: request.policies ?? [],

    // Hardcoded default values for missing fields
    openapi: request.openapi ?? '',
    modelProviders:
      request.modelProviders ?? buildModelProvidersFromTemplate(request.template),
    rateLimiting: {},
    security: {
      enabled: true,
      apiKey: {
        enabled: true,
        key: 'X-API-Key',
        in: 'header',
      },
    },
  };
}
