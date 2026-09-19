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

const MCP_SPEC_VERSION_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

// Advisory only - warn, never reject. The gateway decides at deploy, so this may safely go stale.
const SUPPORTED_MCP_SPEC_VERSIONS = ['2025-06-18', '2025-11-25', '2026-07-28'];

function unknownMCPSpecVersions(values: string[]): string[] {
  return values.filter(
    (value) => !SUPPORTED_MCP_SPEC_VERSIONS.includes(value.trim())
  );
}

export function mcpSpecVersionWarning(values: string[]): string | null {
  const unknown = unknownMCPSpecVersions(values);
  if (unknown.length === 0) {
    return null;
  }
  const noun = unknown.length === 1 ? 'version' : 'versions';
  return `Unknown MCP spec ${noun} provided: ${unknown.join(', ')}`;
}

// Shape only: 2012-02-31 deliberately passes, so a revision newer than this build stays typeable.
export function validateMCPSpecVersion(candidate: string): string | null {
  if (!MCP_SPEC_VERSION_PATTERN.test(candidate.trim())) {
    return 'Use the YYYY-MM-DD form, for example 2026-07-28';
  }
  return null;
}

// undefined, not []: platform-api rejects a request that sets both spec version fields.
export function normalizeMCPSpecVersions(values: string[]): string[] | undefined {
  const cleaned = Array.from(
    new Set(values.map((value) => value.trim()).filter(Boolean))
  );
  return cleaned.length > 0 ? cleaned : undefined;
}
