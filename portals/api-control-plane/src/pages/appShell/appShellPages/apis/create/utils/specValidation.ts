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

import { extractOperations } from './specDetails';

/**
 * Is this fetched document an OpenAPI definition this step can work with?
 *
 * The bar is deliberately "usable", not "conformant". Two things consume a
 * fetched contract; the preview, which needs a document Swagger UI
 * recognises, and the create request, which needs a name, a version and a set
 * of operations, and nothing downstream checks either: the platform's own API
 * takes those fields, never the document, so a spec that slips through here is
 * never caught later.
 *
 * Full schema conformance would need a validator library and, in the browser,
 * a `Buffer` polyfill; the checks below need neither and cover the failures
 * that actually break this wizard.
 */

/** Dialects the step reads. Anything else is not something it can import. */
export type SpecDialect = 'openapi-3.0' | 'openapi-3.1' | 'swagger-2.0';

export type SpecIssueCode =
  /** No `openapi`/`swagger` key at all; a README, an AsyncAPI doc, JSON data. */
  | 'notASpec'
  /** Names a version this step doesn't read, e.g. `openapi: 4.0.0`. */
  | 'unsupportedDialect'
  /** `paths` missing, or not an object. */
  | 'noPaths'
  /** `paths` is there but declares no operation the platform can express. */
  | 'noOperations'
  /** `info.title` missing; the API would be created unnamed. */
  | 'missingTitle'
  /** `info.version` missing; the API would be created unversioned. */
  | 'missingVersion'
  /** No `servers`/`host`, so no upstream can be read off the document. */
  | 'noServers'
  /** Path keys that aren't paths; those entries are skipped. */
  | 'badPathKeys'
  /** `$ref`s pointing outside the document, which nothing here resolves. */
  | 'externalRefs';

export type SpecIssue = {
  /** The offending fragment, as data; never rendered as translated copy. */
  detail?: string;
  /** `error` stops the import; `warning` lets it through. */
  severity: 'error' | 'warning';
  code: SpecIssueCode;
};

export type SpecValidation =
  | { dialect: SpecDialect; status: 'valid'; warnings: SpecIssue[] }
  | { issues: SpecIssue[]; status: 'invalid' };

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;

const asText = (value: unknown): string | undefined => {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
};

/** How many path keys are listed before the message stops naming them. */
const MAX_LISTED_PATHS = 3;

/** Nodes walked while hunting for `$ref`s, so a huge document can't stall. */
const MAX_WALKED_NODES = 20000;

/**
 * T1: which dialect the document claims, if any.
 *
 * A `3.x` the step hasn't met is read as 3.0 rather than refused — Swagger UI
 * and the extractor both handle it, and refusing a minor version bump would
 * age badly.
 */
const readDialect = (spec: Record<string, unknown>): SpecDialect | 'unsupported' | null => {
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
 * Any `$ref` that leaves the document; `./schemas/order.yaml`, or an http
 * URL. Nothing here resolves them, so the preview shows the gaps and the
 * created API carries whatever the operations themselves declared.
 *
 * The walk is both node-capped and cycle-aware: YAML anchors can produce a
 * structure that refers back to itself, and a naive recursion over one would
 * never return.
 */
const findExternalRefs = (spec: Record<string, unknown>): string[] => {
  const found = new Set<string>();
  const seen = new WeakSet<object>();
  const queue: unknown[] = [spec];
  let walked = 0;

  while (queue.length > 0 && walked < MAX_WALKED_NODES) {
    const node = queue.pop();
    walked += 1;
    if (typeof node !== 'object' || node === null) {
      continue;
    }
    if (seen.has(node)) {
      continue;
    }
    seen.add(node);

    if (Array.isArray(node)) {
      queue.push(...node);
      continue;
    }
    for (const [key, value] of Object.entries(node)) {
      if (key === '$ref' && typeof value === 'string' && !value.startsWith('#')) {
        found.add(value);
        continue;
      }
      queue.push(value);
    }
  }

  return [...found];
};

/**
 * Validates a parsed document (T0 has already turned bytes into an object).
 *
 * Errors are the things that leave nothing to show or nothing to create;
 * everything the general form can still be completed by hand is a warning, so
 * an otherwise good contract is never refused over a missing description.
 */
export const validateApiSpec = (spec: Record<string, unknown> | undefined): SpecValidation => {
  if (spec === undefined) {
    return { issues: [{ code: 'notASpec', severity: 'error' }], status: 'invalid' };
  }

  // ---- T1: is it an OpenAPI document at all? ----
  const dialect = readDialect(spec);
  if (dialect === null) {
    return { issues: [{ code: 'notASpec', severity: 'error' }], status: 'invalid' };
  }
  if (dialect === 'unsupported') {
    return {
      issues: [
        {
          code: 'unsupportedDialect',
          detail: asText(spec.openapi) ?? asText(spec.swagger),
          severity: 'error',
        },
      ],
      status: 'invalid',
    };
  }

  // ---- T2: is it usable? ----
  const errors: SpecIssue[] = [];
  const warnings: SpecIssue[] = [];

  const paths = asRecord(spec.paths);
  if (paths === null) {
    errors.push({ code: 'noPaths', severity: 'error' });
  } else {
    // The same reader the extractor uses, so "has operations" here means
    // exactly "the create request will carry operations".
    const operations = extractOperations(spec);
    if (operations.length === 0) {
      errors.push({ code: 'noOperations', severity: 'error' });
    }

    const badKeys = Object.keys(paths).filter((key) => !key.startsWith('/'));
    if (badKeys.length > 0) {
      warnings.push({
        code: 'badPathKeys',
        detail: badKeys.slice(0, MAX_LISTED_PATHS).join(', '),
        severity: 'warning',
      });
    }
  }

  const info = asRecord(spec.info) ?? {};
  if (asText(info.title) === undefined) {
    warnings.push({ code: 'missingTitle', severity: 'warning' });
  }
  if (asText(info.version) === undefined) {
    warnings.push({ code: 'missingVersion', severity: 'warning' });
  }

  const hasServers =
    (Array.isArray(spec.servers) && spec.servers.length > 0) || asText(spec.host) !== undefined;
  if (!hasServers) {
    warnings.push({ code: 'noServers', severity: 'warning' });
  }

  const externalRefs = findExternalRefs(spec);
  if (externalRefs.length > 0) {
    warnings.push({
      code: 'externalRefs',
      detail: externalRefs.slice(0, MAX_LISTED_PATHS).join(', '),
      severity: 'warning',
    });
  }

  return errors.length > 0
    ? { issues: errors, status: 'invalid' }
    : { dialect, status: 'valid', warnings };
};
