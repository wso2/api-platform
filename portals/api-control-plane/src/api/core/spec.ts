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

import type { components, operations } from '../generated/platform';

/**
 * Spec-derived type helpers. These turn `openapi-typescript`'s deeply nested
 * output into one-word lookups keyed by `operationId`, which is what keeps a
 * ~180-schema, ~200-operation surface usable by hand:
 *
 *   type Response = ResponseOf<'ListRESTAPIs'>   // RESTAPIListResponse
 *   type Body     = BodyOf<'CreateRESTAPI'>      // CreateRESTAPIRequest
 *   type Query    = QueryOf<'ListRESTAPIs'>      // { projectId, limit?, ... }
 *
 * Nothing here is hand-maintained: rename a schema in openapi.yaml, re-run
 * codegen, and every affected call site fails typecheck in CI. That is the
 * whole point — drift becomes a build error instead of a runtime bug.
 */

export type Schemas = components['schemas'];
export type Schema<K extends keyof Schemas> = Schemas[K];
export type OperationId = keyof operations;

type SuccessStatus = 200 | 201 | 202 | 204;

type ResponseContent<T> = T extends { content: { 'application/json': infer B } }
  ? B
  : T extends { content?: never }
    ? void
    : never;

/**
 * The success response body for an operation. Resolves 200/201/202 to their
 * JSON schema and a bodiless 204 to `void`, so a delete hook is typed
 * `void` rather than `unknown` or `any`.
 */
export type ResponseOf<Id extends OperationId> = operations[Id] extends {
  responses: infer R;
}
  ? SuccessStatus extends infer S
    ? S extends keyof R
      ? ResponseContent<R[S]>
      : never
    : never
  : never;

/** The JSON request body for an operation. `never` if it takes none. */
export type BodyOf<Id extends OperationId> = operations[Id] extends {
  requestBody: { content: { 'application/json': infer B } };
}
  ? B
  : operations[Id] extends {
        requestBody?: { content: { 'application/json': infer B } };
      }
    ? B | undefined
    : never;

/** Multipart body (spec import, gateway manifest upload). */
export type FormBodyOf<Id extends OperationId> = operations[Id] extends {
  requestBody: { content: { 'multipart/form-data': infer B } };
}
  ? B
  : never;

/** Query-string parameters, including which of them are required. */
export type QueryOf<Id extends OperationId> = operations[Id] extends {
  parameters: { query?: infer Q };
}
  ? Q
  : never;

/** Path parameters (`restApiId`, `deploymentId`, …). */
export type PathOf<Id extends OperationId> = operations[Id] extends {
  parameters: { path: infer P };
}
  ? P
  : never;

/**
 * Any paginated collection envelope: every list endpoint in the spec returns
 * `{ count, list, pagination }`, so one generic covers all of them.
 */
export type ListEnvelope<T> = {
  count: number;
  list: T[];
  pagination: Schema<'Pagination'>;
};

/** The item type inside a list response, e.g. `ItemOf<'ListRESTAPIs'>` → RESTAPI. */
export type ItemOf<Id extends OperationId> =
  ResponseOf<Id> extends { list?: (infer T)[] } ? T : never;
