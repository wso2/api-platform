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

import { http, type RequestOptions } from '../../core/http';
import type { BodyOf, PathOf, QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for `/rest-apis`. One thin function per spec operation:
 * no branching, no adapters, no cache awareness — just "call this endpoint
 * with these arguments and get the spec's response type back".
 *
 * Every signature is derived from the generated types via the `operationId`,
 * so renaming a field in openapi.yaml breaks this file at compile time. Nothing
 * here is hand-typed.
 */

export type RestApi = Schema<'RESTAPI'>;
/**
 * One entry of `RESTAPI.operations`. Re-exported because the Develop section
 * edits operations in place and must not restate their shape: `Operation`
 * nests the wire fields under `request`, and a hand-written mirror is exactly
 * the drift the spec-derived types exist to prevent.
 */
export type Operation = Schema<'Operation'>;
/** One entry of `RESTAPI.policies` or `Operation.request.policies`. */
export type Policy = Schema<'Policy'>;
/** `RESTAPI.upstream` — the main/sandbox backend pair. */
export type Upstream = Schema<'Upstream'>;
/** One side of `Upstream`; carries `url` or `ref`, plus optional `auth`. */
export type UpstreamDefinition = Schema<'UpstreamDefinition'>;
export type RestApiListResponse = ResponseOf<'ListRESTAPIs'>;
export type ListRestApisQuery = QueryOf<'ListRESTAPIs'>;
/**
 * Body for `CreateRESTAPI`.
 *
 * Deliberately **not** `BodyOf<'CreateRESTAPI'>`, which is the one place in
 * this layer where the generated type cannot be used. The spec defines:
 *
 *   CreateRESTAPIRequest:
 *     allOf:
 *       - $ref: RESTAPI
 *       - type: object
 *         required: [displayName, context, version, projectId]   # no `properties`
 *
 * An object schema carrying `required` but no `properties` generates as
 * `Record<string, never>`, so the intersection resolves to
 * `RESTAPI & Record<string, never>` — every property becomes `never` and the
 * body is uninhabitable. Nothing could be passed to this function at all.
 *
 * `RESTAPI` is an exact substitute rather than a loosening: all four fields the
 * second member names are already required on `RESTAPI`, so that member adds no
 * constraint and only breaks codegen. Restore the derived type once the spec is
 * corrected upstream.
 */
export type CreateRestApiBody = Schema<'RESTAPI'>;
export type UpdateRestApiBody = BodyOf<'UpdateRESTAPI'>;

const BASE = '/rest-apis';

/** URL-encoded path for one API. Handles are user-supplied — always encode. */
const resourcePath = (restApiId: PathOf<'GetRESTAPI'>['restApiId']): string =>
  `${BASE}/${encodeURIComponent(restApiId)}`;

export const listRestApis = async (options?: RequestOptions): Promise<RestApiListResponse> => {
  return http.get<RestApiListResponse>(BASE, {
    ...options,
    operationName: 'ListRESTAPIs',
  });
};

export const getRestApi = async (restApiId: string, options?: RequestOptions): Promise<RestApi> => {
  return http.get<RestApi>(resourcePath(restApiId), {
    ...options,
    operationName: 'GetRESTAPI',
  });
};

export const createRestApi = async (
  body: CreateRestApiBody,
  options?: RequestOptions,
): Promise<RestApi> => {
  return http.post<RestApi>(BASE, body, {
    ...options,
    operationName: 'CreateRESTAPI',
  });
};

export const updateRestApi = async (
  restApiId: string,
  body: UpdateRestApiBody,
  options?: RequestOptions,
): Promise<RestApi> => {
  return http.put<RestApi>(resourcePath(restApiId), body, {
    ...options,
    operationName: 'UpdateRESTAPI',
  });
};

export const deleteRestApi = async (restApiId: string, options?: RequestOptions): Promise<void> => {
  return http.delete<void>(resourcePath(restApiId), {
    ...options,
    operationName: 'DeleteRESTAPI',
  });
};
