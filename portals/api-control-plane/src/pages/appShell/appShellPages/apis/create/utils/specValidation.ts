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

/** Dialects the step reads. Anything else is not something it can import. */
export type SpecDialect = 'openapi-3.0' | 'openapi-3.1' | 'swagger-2.0';

export type SpecIssueCode =
  /** `info.title` missing; the API would be created unnamed. */
  | 'missingTitle'
  /** `info.version` missing; the API would be created unversioned. */
  | 'missingVersion'
  /** No `servers`/`host`, so no upstream can be read off the document. */
  | 'noServers';

export type SpecIssue = {
  /** The offending fragment, as data; never rendered as translated copy. */
  detail?: string;
  /** `error` stops the import; `warning` lets it through. */
  severity: 'error' | 'warning';
  code: SpecIssueCode;
};

const asText = (value: unknown): string | undefined => {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
};

/**
 * Which dialect the document claims, if any.
 *
 * A `3.x` the step hasn't met is read as 3.0 rather than refused — Swagger UI
 * and the extractor both handle it, and refusing a minor version bump would
 * age badly.
 */
export const readDialectFromSpec = (spec: Record<string, unknown>): SpecDialect | 'unsupported' | null => {
  const openapi = asText(spec.openapi);
  if (openapi !== undefined) {
    if (openapi.startsWith('3.1')) {
      return 'openapi-3.1';
    }
    return openapi.startsWith('3.') ? 'openapi-3.0' : 'unsupported';
  }

  const swagger = asText(spec.swagger);
  if (swagger !== undefined) {
    return swagger.startsWith('2.') ? 'swagger-2.0' : 'unsupported';
  }
  return null;
};

/**
 * Collects non-fatal warnings about a spec without blocking import.
 *
 * Checks: missingTitle, missingVersion, noServers.
 * External $ref validation is handled by backend with proper library.
 * Structural issues (noPaths, noOperations, badPathKeys) are considered errors
 * and are left to backend validation, not raised here as warnings.
 *
 * @param spec The parsed spec object.
 */
export const collectSpecWarnings = (
  spec: Record<string, unknown>,
): SpecIssue[] => {
  const warnings: SpecIssue[] = [];
  const info = spec.info as Record<string, unknown> | undefined;

  if (!asText(info?.title as unknown)) {
    warnings.push({ code: 'missingTitle', severity: 'warning' });
  }
  if (!asText(info?.version as unknown)) {
    warnings.push({ code: 'missingVersion', severity: 'warning' });
  }

  // noServers: OpenAPI 3.x uses `servers[]`, Swagger 2.x uses `host`.
  const isSwagger2 = asText(spec.swagger) !== undefined;
  if (isSwagger2) {
    if (!asText(spec.host as unknown)) {
      warnings.push({ code: 'noServers', severity: 'warning' });
    }
  } else {
    const servers = spec.servers as unknown[] | undefined;
    if (!servers || servers.length === 0) {
      warnings.push({ code: 'noServers', severity: 'warning' });
    }
  }

  return warnings;
};
