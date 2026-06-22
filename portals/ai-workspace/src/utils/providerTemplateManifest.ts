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

import YAML from 'yaml';
import type { ProviderTemplate } from './types';

export function buildTemplateManifestYaml(t: ProviderTemplate): string {
  const spec: Record<string, unknown> = { displayName: t.name };

  if (t.provider?.trim()) spec.provider = t.provider.trim();
  if (t.version) spec.version = t.version;

  const md = t.metadata;
  if (md) {
    const m: Record<string, unknown> = {};
    if (md.endpointUrl?.trim()) m.endpointUrl = md.endpointUrl.trim();
    if (md.auth) {
      const a: Record<string, unknown> = {};
      if (md.auth.type?.trim()) a.type = md.auth.type.trim();
      if (md.auth.header?.trim()) a.header = md.auth.header.trim();
      if (md.auth.valuePrefix?.trim()) a.valuePrefix = md.auth.valuePrefix.trim();
      if (Object.keys(a).length) m.auth = a;
    }
    if (md.logoUrl?.trim()) m.logoUrl = md.logoUrl.trim();
    if (md.openapiSpecUrl?.trim()) m.openapiSpecUrl = md.openapiSpecUrl.trim();
    if (Object.keys(m).length) spec.metadata = m;
  }

  const tokenKeys = [
    'promptTokens',
    'completionTokens',
    'totalTokens',
    'remainingTokens',
    'requestModel',
    'responseModel',
  ] as const;
  for (const key of tokenKeys) {
    const v = t[key];
    if (v && v.identifier?.trim()) {
      spec[key] = { location: v.location, identifier: v.identifier };
    }
  }

  if (t.resourceMappings?.resources?.length) {
    spec.resourceMappings = t.resourceMappings;
  }

  const manifest = {
    apiVersion: 'gateway.api-platform.wso2.com/v1alpha1',
    kind: 'LlmProviderTemplate',
    metadata: { name: t.id },
    spec,
  };
  return YAML.stringify(manifest);
}

// File name for the downloaded manifest, e.g. "openai-v2-template.yaml".
export function templateManifestFileName(t: ProviderTemplate): string {
  const versionPart = t.version ? `-${t.version}` : '';
  return `${t.id}${versionPart}-template.yaml`;
}

// Trigger a browser download of the template's manifest YAML.
export function downloadTemplateYaml(t: ProviderTemplate): string {
  const fileName = templateManifestFileName(t);
  const blob = new Blob([buildTemplateManifestYaml(t)], {
    type: 'text/yaml;charset=utf-8',
  });
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = objectUrl;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(objectUrl);
  return fileName;
}
