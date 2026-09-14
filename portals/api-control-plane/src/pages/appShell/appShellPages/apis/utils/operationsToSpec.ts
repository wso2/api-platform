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

import type { RestApi } from '@/api/resources/restApis';

/**
 * Turns a fetched `RESTAPI` into an OpenAPI document a spec viewer can render.
 *
 * The platform stores an API as a list of operations, not as a contract: the
 * creation wizard reads an uploaded definition, keeps `method`, `path`, `name`
 * and `description` off each operation, and discards the document itself
 * (`create/utils/createRestApiBody.ts`). `GET /rest-apis/{id}` has no
 * `/definition` sibling to fetch it back from, so an OpenAPI view of a saved
 * API can only be *rebuilt* from what survived.
 *
 * That makes the result a display shim rather than a contract. It carries
 * method, path and summary and nothing else — no parameters, request bodies,
 * schemas or responses, because the platform never held them. Don't hand it to
 * anything that treats an OpenAPI document as a source of truth (codegen, a
 * request builder, validation); it would read the absences as facts.
 */

/**
 * The document says 3.0.3 rather than 3.1: swagger-client renders both, and
 * 3.0 is what the wizard's own imports are overwhelmingly written against.
 */
const OPENAPI_VERSION = '3.0.3';

/**
 * Methods that are path-item keys in OpenAPI. The spec's `OperationRequest`
 * enum is a subset (no `trace`), so anything outside this set is a value the
 * platform shouldn't have stored — dropped rather than emitted as a key
 * swagger-client would treat as an extension field.
 */
const HTTP_METHODS = new Set([
  'delete',
  'get',
  'head',
  'options',
  'patch',
  'post',
  'put',
  'trace',
]);

/** A trimmed value, or `undefined` when nothing is left worth emitting. */
const trimmed = (value: string | undefined): string | undefined => {
  const next = value?.trim();
  return next === '' ? undefined : next;
};

export const restApiToOpenApiSpec = (api: RestApi): Record<string, unknown> => {
  const paths: Record<string, Record<string, unknown>> = {};

  // `operations` is optional on the spec's `RESTAPI`.
  (api.operations ?? []).forEach((operation) => {
    const path = trimmed(operation.request?.path);
    // Lowercased because OpenAPI path-item keys are lowercase, while the
    // platform stores the method uppercase. Normalized once, here, so nothing
    // downstream has to guess which case it is looking at.
    const method = trimmed(operation.request?.method)?.toLowerCase();
    if (path === undefined || method === undefined || !HTTP_METHODS.has(method)) return;

    const summary = trimmed(operation.name);
    const description = trimmed(operation.description);

    const pathItem = paths[path] ?? {};
    // Last one wins on a duplicate method+path. The platform shouldn't hold
    // two, and an object key can only carry one either way.
    pathItem[method] = {
      ...(summary === undefined ? {} : { summary }),
      ...(description === undefined ? {} : { description }),
      // Required by OpenAPI, and swagger-client reads it while rendering, so
      // it is present but empty rather than absent — the viewer hides the
      // section, since an empty table would read as a load that failed.
      responses: {},
    };
    paths[path] = pathItem;
  });

  return {
    openapi: OPENAPI_VERSION,
    info: {
      // `info` is required by OpenAPI, and every field of it here is required
      // too, so both fall back rather than being dropped when the API has
      // neither a display name nor a version. Nothing renders it: the viewer
      // hides the info block, which would otherwise repeat the page header.
      title: trimmed(api.displayName) ?? trimmed(api.id) ?? '',
      version: trimmed(api.version) ?? '',
      ...(trimmed(api.description) === undefined
        ? {}
        : { description: trimmed(api.description) }),
    },
    paths,
  };
};
