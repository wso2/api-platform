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

import type { ApiCreationWizardDraftState, ApiOperation, Operationrequest } from '../types';

/**
 * Reads what the general form needs out of a fetched definition.
 *
 * Written for documents that have not been validated: the contract step hands
 * over whatever parsed as YAML or JSON, so every read here is defensive and
 * anything unrecognised is left out rather than guessed at. A key the document
 * doesn't answer for is *absent* from the draft rather than empty, so merging
 * it over the form's own defaults can only ever fill blanks — never blank out
 * something the user already typed.
 *
 * Both OpenAPI 3.x (`servers`) and Swagger 2.0 (`host`/`basePath`/`schemes`)
 * are understood, since the step accepts either.
 */

/** Methods the form's own operation type can hold. */
const SUPPORTED_METHODS: Operationrequest['method'][] = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'];

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;

/** A non-empty trimmed string, or `undefined` for anything else. */
const asText = (value: unknown): string | undefined => {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
};

/**
 * A path segment made from a title: "Orders API v2" becomes "orders-api-v2".
 * Used only as the context of last resort, when the document names no server.
 */
const slugify = (value: string): string =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

/**
 * The first server URL a definition declares, assembled per spec version.
 *
 * A URL still carrying `{variables}` is dropped: substituting them is the
 * user's call, and a templated string is no use as an upstream.
 */
const readServerUrl = (spec: Record<string, unknown>): string | undefined => {
  const servers = spec.servers;
  if (Array.isArray(servers)) {
    for (const entry of servers) {
      const url = asText(asRecord(entry)?.url);
      if (url !== undefined && !url.includes('{')) {
        return url;
      }
    }
    return undefined;
  }

  // Swagger 2.0 spells the same thing across three fields.
  const host = asText(spec.host);
  if (host === undefined) {
    return undefined;
  }
  const schemes = Array.isArray(spec.schemes)
    ? spec.schemes.map(asText).filter((scheme) => scheme !== undefined)
    : [];
  const scheme = schemes.includes('https') ? 'https' : (schemes[0] ?? 'https');
  return `${scheme}://${host}${asText(spec.basePath) ?? ''}`;
};

/** The path part of a server URL — what the gateway would expose it under. */
const readContextFromUrl = (serverUrl: string): string | undefined => {
  try {
    const { pathname } = new URL(serverUrl);
    const trimmed = pathname.replace(/\/+$/, '');
    return trimmed === '' ? undefined : trimmed;
  } catch {
    // A relative server URL ("/v2") never parses on its own, and is already
    // the context in its own right.
    const trimmed = serverUrl.replace(/\/+$/, '');
    return trimmed.startsWith('/') ? trimmed : undefined;
  }
};

/** `http`/`https` from the server URL, so the form's transports match it. */
const readTransports = (serverUrl: string | undefined): ('http' | 'https')[] | undefined => {
  if (serverUrl === undefined) {
    return undefined;
  }
  if (serverUrl.startsWith('https://')) {
    return ['https'];
  }
  if (serverUrl.startsWith('http://')) {
    return ['http'];
  }
  return undefined;
};

/**
 * Every operation the definition declares, flattened for the form.
 *
 * `HEAD`, `OPTIONS` and `TRACE` are skipped rather than coerced: the form's
 * operation type has no room for them, and inventing a method would be worse
 * than leaving the row out for the user to add.
 */
export const extractOperations = (spec: Record<string, unknown> | undefined): ApiOperation[] => {
  const paths = asRecord(spec?.paths);
  if (paths === null) {
    return [];
  }

  return Object.entries(paths).flatMap(([path, pathItem]) => {
    const item = asRecord(pathItem);
    if (item === null || !path.startsWith('/')) {
      return [];
    }

    return SUPPORTED_METHODS.flatMap((method): ApiOperation[] => {
      const operation = asRecord(item[method.toLowerCase()]);
      if (operation === null) {
        return [];
      }
      const name =
        asText(operation.operationId) ?? asText(operation.summary) ?? `${method} ${path}`;

      return [
        {
          name,
          ...(asText(operation.description) === undefined
            ? {}
            : { description: asText(operation.description) }),
          request: { method, path },
        },
      ];
    });
  });
};

/**
 * The draft the general form starts from, read off a fetched definition.
 *
 * Only what the document actually says is returned. `id`, `readOnly` and the
 * lifecycle/kind constants belong to the form and to the platform, so they are
 * deliberately left alone.
 */
export const extractApiDetails = (
  spec: Record<string, unknown> | undefined,
): ApiCreationWizardDraftState => {
  if (spec === undefined) {
    return {};
  }

  const info = asRecord(spec.info) ?? {};
  const displayName = asText(info.title);
  const version = asText(info.version);
  const description = asText(info.description);
  const serverUrl = readServerUrl(spec);
  const transports = readTransports(serverUrl);
  const operations = extractOperations(spec);

  const context =
    (serverUrl === undefined ? undefined : readContextFromUrl(serverUrl)) ??
    (displayName === undefined ? undefined : `/${slugify(displayName)}`);

  return {
    ...(displayName === undefined ? {} : { displayName }),
    ...(version === undefined ? {} : { version }),
    ...(description === undefined ? {} : { description }),
    // An empty context slug ("/" from a title of punctuation only) is no
    // better than saying nothing.
    ...(context === undefined || context === '/' ? {} : { context }),
    ...(serverUrl === undefined ? {} : { upstream: { main: { url: serverUrl } } }),
    ...(transports === undefined ? {} : { transports }),
    ...(operations.length === 0 ? {} : { operations }),
  };
};
