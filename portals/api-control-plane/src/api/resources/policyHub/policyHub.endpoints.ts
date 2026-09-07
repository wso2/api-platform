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

import yaml from 'js-yaml';

import { runtimeConfig } from '@/config/runtime';
import { ApiError, ClientErrorCode } from '../../core/errors';
import type { ParameterSchema } from './policySchema';

/**
 * Transport layer for the Policy Hub catalog.
 *
 * Two things make this resource the exception to the rules in
 * `src/api/README.md`, and both are properties of the upstream, not choices
 * made here:
 *
 * 1. **No spec-derived types.** The Policy Hub is a separate service with no
 *    operations in `platform-api/resources/openapi.yaml`, so there is no
 *    `operationId` to derive `ResponseOf`/`BodyOf` from. The types below are
 *    hand-written *and* every response is run through a normalizer, because a
 *    hand-written type is an assumption about an untyped wire format rather
 *    than a guarantee from a generator.
 *
 * 2. **No `core/http`.** That client is pinned to the BFF's same-origin proxy
 *    (`platformApiBaseUrl()`), while the hub is a separate origin read straight
 *    from `runtimeConfig.policyHubBaseUrl`. Routing through it would also add
 *    `X-Request-Id`/`Content-Type` to a cross-origin GET and so require a CORS
 *    preflight the hub is not currently configured to answer. `getPolicyDefinition`
 *    additionally returns YAML, which `core/http`'s JSON body parser cannot read.
 *
 * What this file does *not* opt out of is the error contract: every failure
 * leaves here as a `core/errors` `ApiError` with a `kind` and a `code`, so the
 * hooks and the UI above see the same error type as every other resource.
 */

/** A policy as listed by the Policy Hub catalog. */
export type PolicySummary = {
  name: string;
  version: string;
  displayName: string;
  provider: string;
  categories: string[];
  tags: string[];
  isLatest: boolean;
  description?: string;
  iconUrl?: string;
};

export type PolicyListResult = {
  policies: PolicySummary[];
  total: number;
};

/** Parsed policy definition (from the YAML), used to render the config form. */
export type PolicyDefinition = {
  name: string;
  version: string;
  description?: string;
  /** Recursive JSON-Schema for the policy's parameters. */
  schema: ParameterSchema;
};

/** Everything a Policy Hub read accepts. No `orgId`: the hub is not org-scoped. */
export type PolicyHubRequestOptions = {
  signal?: AbortSignal;
};

/**
 * True when an operator has pointed the console at a Policy Hub.
 *
 * Deliberately a plain predicate rather than a hook: it reads static runtime
 * config, so nothing about it is reactive, and naming it `use*` (as the legacy
 * module did) made every call site look like a subscription it is not.
 */
export const isPolicyHubConfigured = (): boolean => Boolean(runtimeConfig.policyHubBaseUrl);

/* -------------------------------------------------------------------------- */
/* Normalizers — the wire is untyped, so nothing is trusted by shape alone      */
/* -------------------------------------------------------------------------- */

const asArray = (value: unknown): unknown[] => (Array.isArray(value) ? value : []);

const str = (value: unknown): string =>
  typeof value === 'string' ? value : value == null ? '' : String(value);

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : {};

const toSummary = (value: unknown): PolicySummary => {
  const source = asRecord(value);
  return {
    name: str(source.name),
    version: str(source.version),
    displayName: str(source.displayName) || str(source.name),
    provider: str(source.provider),
    categories: asArray(source.categories).map(str),
    tags: asArray(source.tags).map(str),
    isLatest: source.isLatest !== false,
    description: str(source.description) || undefined,
    iconUrl: str(source.iconUrl) || undefined,
  };
};

const EMPTY_SCHEMA: ParameterSchema = { type: 'object', properties: {} };

/** Normalizes a raw YAML node into a recursive ParameterSchema. */
const toSchema = (raw: unknown): ParameterSchema => {
  const source = asRecord(raw);
  const type = str(source.type) as ParameterSchema['type'];
  const schema: ParameterSchema = {
    type: type || 'string',
    title: str(source.title) || undefined,
    description: str(source.description) || undefined,
    default: source.default,
    enum: asArray(source.enum).map(str),
    required: asArray(source.required).map(str),
  };
  if (schema.enum && schema.enum.length === 0) delete schema.enum;
  if (schema.required && schema.required.length === 0) delete schema.required;
  if (typeof source['x-wso2-policy-advanced-param'] === 'boolean') {
    schema.advanced = source['x-wso2-policy-advanced-param'] as boolean;
  }
  const props = asRecord(source.properties);
  if (Object.keys(props).length > 0) {
    schema.properties = Object.fromEntries(
      Object.entries(props).map(([key, value]) => [key, toSchema(value)]),
    );
  }
  if (source.items) schema.items = toSchema(source.items);
  // A dynamic key→value map: object with `additionalProperties` as a value
  // schema (the boolean form is ignored). Renders as a key-value editor.
  if (source.additionalProperties && typeof source.additionalProperties === 'object') {
    schema.additionalProperties = toSchema(source.additionalProperties);
  }
  return schema;
};

/* -------------------------------------------------------------------------- */
/* Transport                                                                    */
/* -------------------------------------------------------------------------- */

/**
 * Builds the catalog query string. Page is 1-based; the hub uses offset/limit.
 * Exported for testing.
 */
export const buildPolicyQuery = (page: number, pageSize: number, categories?: string[]): string => {
  const params = new URLSearchParams();
  params.set('offset', String((page - 1) * pageSize));
  params.set('limit', String(pageSize));
  const selected = (categories || []).filter(Boolean);
  if (selected.length > 0) params.set('categories', selected.join(','));
  return params.toString();
};

/**
 * One fetch, one error contract.
 *
 * `accept` varies because the definition endpoint answers YAML while the rest
 * answer JSON; everything else — abort wiring, failure classification — is the
 * same for every call, which is why it lives here rather than at each call site.
 */
const hubFetch = async (
  path: string,
  accept: string,
  options: PolicyHubRequestOptions = {},
): Promise<Response> => {
  let response: Response;
  try {
    response = await fetch(`${runtimeConfig.policyHubBaseUrl}${path}`, {
      headers: { Accept: accept },
      signal: options.signal,
    });
  } catch (error) {
    // An aborted request is React Query cancelling a superseded or unmounted
    // query, not a failure the user should ever be shown. It has to be
    // distinguished here because `fetch` reports it the same way as a genuine
    // network error.
    const aborted =
      options.signal?.aborted || (error instanceof DOMException && error.name === 'AbortError');
    throw new ApiError(aborted ? 'Request was cancelled' : 'Policy Hub is unreachable', {
      kind: aborted ? 'aborted' : 'network',
      code: aborted ? ClientErrorCode.CLIENT_REQUEST_ABORTED : ClientErrorCode.CLIENT_NETWORK_ERROR,
      operation: path,
      cause: error,
    });
  }

  if (!response.ok) {
    // The hub's failure bodies are not the platform's error envelope, so no
    // server-supplied code is available — hence the client sentinel. The status
    // travels for `isRetryable`, and the message stays generic: the hub's own
    // wording could name internal hosts (see `.claude/rules/error-handling.md`).
    throw new ApiError('The Policy Hub request could not be completed.', {
      kind: 'http',
      status: response.status,
      code: ClientErrorCode.CLIENT_MALFORMED_ERROR,
      operation: path,
    });
  }

  return response;
};

const hubGetJson = async <T>(path: string, options?: PolicyHubRequestOptions): Promise<T> => {
  const response = await hubFetch(path, 'application/json', options);
  return (await response.json()) as T;
};

/** GET /policies — paginated catalog, optionally filtered by category. */
export const listPolicies = async (
  page: number,
  pageSize: number,
  categories?: string[],
  options?: PolicyHubRequestOptions,
): Promise<PolicyListResult> => {
  const data = await hubGetJson<{
    data?: unknown[];
    count?: number;
    pagination?: { total?: number };
  }>(`/policies?${buildPolicyQuery(page, pageSize, categories)}`, options);

  return {
    policies: asArray(data.data).map(toSummary),
    total: data.pagination?.total ?? data.count ?? 0,
  };
};

/** GET /policies/categories — available category names. */
export const listPolicyCategories = async (
  options?: PolicyHubRequestOptions,
): Promise<string[]> => {
  const data = await hubGetJson<{ data?: unknown[] }>('/policies/categories', options);
  return asArray(data.data).map(str);
};

/** GET /policies/{name}/versions — all versions of a policy. */
export const listPolicyVersions = async (
  name: string,
  options?: PolicyHubRequestOptions,
): Promise<PolicySummary[]> => {
  const data = await hubGetJson<unknown>(`/policies/${encodeURIComponent(name)}/versions`, options);
  // The endpoint may return a bare array or a { data } wrapper.
  const list = Array.isArray(data) ? data : asArray(asRecord(data).data);
  return list.map(toSummary);
};

/**
 * GET /policies/{name}/versions/{version}/definition — raw YAML definition,
 * parsed into a PolicyDefinition carrying the recursive parameter schema.
 */
export const getPolicyDefinition = async (
  name: string,
  version: string,
  options?: PolicyHubRequestOptions,
): Promise<PolicyDefinition> => {
  const response = await hubFetch(
    `/policies/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}/definition`,
    'text/yaml, application/json',
    options,
  );

  const text = await response.text();
  let doc: Record<string, unknown>;
  try {
    // `yaml.load` (not `loadAll`) with the default schema: no custom tags, so a
    // definition cannot construct arbitrary JS objects during parsing.
    doc = asRecord(yaml.load(text));
  } catch (error) {
    throw new ApiError('The policy definition could not be read.', {
      kind: 'http',
      code: ClientErrorCode.CLIENT_MALFORMED_ERROR,
      operation: 'getPolicyDefinition',
      cause: error,
    });
  }

  return {
    name: str(doc.name) || name,
    version: str(doc.version) || version,
    description: str(doc.description) || undefined,
    schema: doc.parameters ? toSchema(doc.parameters) : EMPTY_SCHEMA,
  };
};
