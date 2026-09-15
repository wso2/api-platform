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

import type {
  MCPServerCapabilities,
  MCPServerPrompt,
  MCPServerResource,
  MCPServerTool,
} from './types';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isValidTool(item: unknown): item is MCPServerTool {
  return isRecord(item) && typeof item.name === 'string';
}

function isValidResource(item: unknown): item is MCPServerResource {
  return isRecord(item) && typeof item.name === 'string' && typeof item.uri === 'string';
}

function isValidPrompt(item: unknown): item is MCPServerPrompt {
  return isRecord(item) && typeof item.name === 'string';
}

function validateItems<T>(
  value: unknown,
  fieldName: string,
  isValid: (item: unknown) => item is T
): T[] {
  if (value === undefined) return [];
  if (!Array.isArray(value)) {
    throw new Error(`"${fieldName}" must be an array`);
  }
  value.forEach((item, index) => {
    if (!isValid(item)) {
      throw new Error(`"${fieldName}[${index}]" is missing required fields`);
    }
  });
  return value as T[];
}

/**
 * Validates a raw, already JSON/YAML-parsed capabilities object. Throws with
 * a user-facing message on any structural or per-item validation failure —
 * an invalid tools/resources/prompts entry is rejected rather than silently
 * dropped, so staged/saved capabilities can never diverge from what the user
 * actually supplied.
 */
export function parseMCPServerCapabilities(raw: unknown): MCPServerCapabilities {
  if (!isRecord(raw)) {
    throw new Error('Capabilities must be a JSON object');
  }
  return {
    tools: validateItems(raw.tools, 'tools', isValidTool),
    resources: validateItems(raw.resources, 'resources', isValidResource),
    prompts: validateItems(raw.prompts, 'prompts', isValidPrompt),
  };
}
