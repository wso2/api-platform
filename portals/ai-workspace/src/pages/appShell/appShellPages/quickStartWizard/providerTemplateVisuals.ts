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

import type { ProviderTemplate } from '../../../../utils/types';
import awsBedrockLogo from '../../../../assets/brands/AWSBedrock.webp';
import openAiLogo from '../../../../assets/brands/openAI.png';
import anthropicLogo from '../../../../assets/brands/Anthropic.jpg';
import azureLogo from '../../../../assets/brands/Azure.png';
import googleVertexLogo from '../../../../assets/brands/GoogleVertex.png';
import googleGeminiLogo from '../../../../assets/brands/googlegemini.png';
import mistralAiLogo from '../../../../assets/brands/mistralai.png';

const PROVIDER_FAMILIES = [
  {
    key: 'openai',
    matches: ['openai'],
  },
  {
    key: 'mistral',
    matches: ['mistral'],
  },
  {
    key: 'gemini',
    matches: ['gemini'],
  },
  {
    key: 'azure-openai',
    matches: ['azure-openai', 'azure openai'],
  },
  {
    key: 'azureai-foundry',
    matches: ['azure ai foundry', 'azureai-foundry', 'azure ai studio'],
  },
  {
    key: 'aws-bedrock',
    matches: ['awsbedrock', 'aws bedrock', 'bedrock'],
  },
  {
    key: 'anthropic',
    matches: ['anthropic', 'claude'],
  },
] as const;

const COMING_SOON_TEMPLATE_IDS = new Set(['awsbedrock', 'aws-bedrock']);

function normalizeTemplateValue(value?: string): string {
  return (value ?? '').trim().toLowerCase();
}

function matchesTemplate(
  template: ProviderTemplate,
  matchers: readonly string[]
): boolean {
  const id = normalizeTemplateValue(template.id);
  const name = normalizeTemplateValue(template.displayName);

  return matchers.some((matcher) => id.includes(matcher) || name.includes(matcher));
}

export function getQuickStartProviderTemplates(
  templates: ProviderTemplate[]
): ProviderTemplate[] {
  const remaining = [...templates];
  const prioritized: ProviderTemplate[] = [];

  PROVIDER_FAMILIES.forEach(({ matches }) => {
    const index = remaining.findIndex((template) =>
      matchesTemplate(template, matches)
    );

    if (index >= 0) {
      prioritized.push(remaining.splice(index, 1)[0]);
    }
  });

  return [
    ...prioritized,
    ...remaining.sort((templateA, templateB) =>
      templateA.displayName.localeCompare(templateB.displayName)
    ),
  ];
}

export function getProviderLogoForTemplate(templateName: string): string | null {
  const lowerName = templateName.toLowerCase();

  if (lowerName.includes('azure')) {
    return azureLogo;
  }
  if (lowerName.includes('openai') && !lowerName.includes('azure')) {
    return openAiLogo;
  }
  if (lowerName.includes('anthropic') || lowerName.includes('claude')) {
    return anthropicLogo;
  }
  if (lowerName.includes('mistral')) {
    return mistralAiLogo;
  }
  if (lowerName.includes('gemini')) {
    return googleGeminiLogo;
  }
  if (lowerName.includes('google') || lowerName.includes('vertex')) {
    return googleVertexLogo;
  }
  if (lowerName.includes('bedrock') || lowerName.includes('aws')) {
    return awsBedrockLogo;
  }

  return null;
}

export function getShortNameForTemplate(templateName: string): string {
  const words = templateName.split(/[\s-_]+/).filter(Boolean);

  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase();
  }

  return templateName.substring(0, 2).toUpperCase();
}

export function isComingSoonTemplate(templateId?: string): boolean {
  return COMING_SOON_TEMPLATE_IDS.has(normalizeTemplateValue(templateId));
}
