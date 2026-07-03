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
import { familyHandle } from './providerTemplateDisplay';

export function buildTemplateManifestYaml(t: ProviderTemplate): string {
  const spec: Record<string, unknown> = { displayName: t.displayName };

  const groupId = t.groupId?.trim() || familyHandle(t.id);
  if (groupId) spec.groupId = groupId;

  const managedBy = (t.managedBy ?? t.provider)?.trim();
  if (managedBy) spec.managedBy = managedBy;

  if (t.version) spec.version = t.version;

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

  const resources = t.resourceMappings?.resources ?? [];
  const mappedResources = resources
    .filter((r) => r.resource?.trim())
    .map((r) => {
      const entry: Record<string, unknown> = { resource: r.resource };
      let overrides = 0;
      for (const key of tokenKeys) {
        const v = r[key];
        if (!v || !v.identifier?.trim()) continue;
        const g = t[key];
        if (g && g.identifier === v.identifier && g.location === v.location) {
          continue;
        }
        entry[key] = { location: v.location, identifier: v.identifier };
        overrides += 1;
      }
      return overrides ? entry : null;
    })
    .filter((entry): entry is Record<string, unknown> => entry !== null);
  if (mappedResources.length) {
    spec.resourceMappings = { resources: mappedResources };
  }

  const manifest = {
    apiVersion: 'gateway.api-platform.wso2.com/v1',
    kind: 'LlmProviderTemplate',
    metadata: { name: t.id },
    spec,
  };
  return YAML.stringify(manifest);
}

// Generate a filename for the template's manifest YAML.
export function templateManifestFileName(t: ProviderTemplate): string {
  return `${t.id ?? 'template'}-template.yaml`;
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
