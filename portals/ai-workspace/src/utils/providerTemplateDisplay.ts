/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const PROVIDER_TEMPLATE_NAME_MAP: Record<string, string> = {
  openai: 'OpenAI',
  mistralai: 'Mistral',
  gemini: 'Gemini',
  'azure-openai': 'Azure OpenAI',
  'azureai-foundry': 'Azure AI Foundry',
  awsbedrock: 'AWS Bedrock',
  'aws-bedrock': 'AWS Bedrock',
  anthropic: 'Anthropic',
  'google-vertex': 'Google Vertex AI',
};

export function getProviderTemplateDisplayName(template?: string | null): string {
  const normalizedTemplate = template?.trim().toLowerCase();
  if (!normalizedTemplate) {
    return '';
  }

  return PROVIDER_TEMPLATE_NAME_MAP[normalizedTemplate] ?? template ?? '';
}

export function truncateProviderDisplayName(
  name?: string | null,
  maxLength = 30
): string {
  const normalizedName = name?.trim() ?? '';
  if (normalizedName.length <= maxLength) {
    return normalizedName;
  }

  return `${normalizedName.slice(0, maxLength).trim()}…`;
}
