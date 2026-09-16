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
import type { PathOf, QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for `/graphql-apis`. One thin function per spec operation,
 * mirroring `restApis.endpoints.ts` — see that file's header comment for the
 * conventions this follows.
 *
 * Every create/update/validate operation is `multipart/form-data` only, even
 * when no file is attached: the spec packs the JSON body into a `metadata`
 * part alongside an optional `sdlFile` part, so every `schemaSource` variant
 * is expressed the same way. `toMultipart` below builds that envelope.
 */

export type GraphQLApi = Schema<'GraphQLAPI'>;
export type GraphQLApiDetail = Schema<'GraphQLAPIDetail'>;
export type GraphQLApiListResponse = ResponseOf<'ListGraphQLAPIs'>;
/** The `list` field's element shape — a slimmer projection than `GraphQLApi`, missing e.g. `schemaSource`. */
export type GraphQLApiListItem = Schema<'GraphQLAPIListItem'>;
export type ListGraphQLApisQuery = QueryOf<'ListGraphQLAPIs'>;
export type GraphQLApiSdlResponse = ResponseOf<'GetGraphQLAPISDL'>;
export type ValidateGraphQLSchemaRequest = Schema<'ValidateGraphQLSchemaRequest'>;
export type ValidateGraphQLSchemaResponse = Schema<'ValidateGraphQLSchemaResponse'>;

/**
 * JSON metadata for `POST /graphql-apis`, packed into the multipart body's
 * `metadata` part.
 *
 * Deliberately **not** `Schema<'CreateGraphQLAPIRequest'>`, which — like REST's
 * `CreateRESTAPIRequest` — generates as `GraphQLAPI & Record<string, never>`:
 * an `allOf` member with `required` but no `properties` collapses every field
 * to `never`, making the body uninhabitable. `GraphQLAPI` is an exact
 * substitute: every field the second member requires is already required on
 * `GraphQLAPI`. Restore the derived type once the spec is corrected upstream.
 */
export type CreateGraphQLApiMetadata = Schema<'GraphQLAPI'>;

export type CreateGraphQLApiBody = {
  metadata: CreateGraphQLApiMetadata;
  /** Required when `metadata.schemaSource` is `'file'`; omitted otherwise. */
  sdlFile?: File;
};

export type ValidateGraphQLSchemaBody = {
  metadata: ValidateGraphQLSchemaRequest;
  /** Required when `metadata.schemaSource` is `'file'`; omitted otherwise. */
  sdlFile?: File;
};

/** JSON metadata for `PUT /graphql-apis/{id}`, packed the same way as create. */
export type UpdateGraphQLApiBody = {
  metadata: Schema<'GraphQLAPI'>;
  /** Required when `metadata.schemaSource` is `'file'`; omitted otherwise. */
  sdlFile?: File;
};

const BASE = '/graphql-apis';

/** URL-encoded path for one API. Handles are user-supplied — always encode. */
const resourcePath = (graphqlApiId: PathOf<'GetGraphQLAPI'>['graphqlApiId']): string =>
  `${BASE}/${encodeURIComponent(graphqlApiId)}`;

/** Packs the shared `{ metadata, sdlFile }` envelope into `FormData`. */
const toMultipart = (body: { metadata: unknown; sdlFile?: File }): FormData => {
  const form = new FormData();
  form.append('metadata', JSON.stringify(body.metadata));
  if (body.sdlFile !== undefined) {
    form.append('sdlFile', body.sdlFile);
  }
  return form;
};

export const listGraphQLApis = async (
  options?: RequestOptions,
): Promise<GraphQLApiListResponse> => {
  return http.get<GraphQLApiListResponse>(BASE, {
    ...options,
    operationName: 'ListGraphQLAPIs',
  });
};

export const getGraphQLApi = async (
  graphqlApiId: string,
  options?: RequestOptions,
): Promise<GraphQLApiDetail> => {
  return http.get<GraphQLApiDetail>(resourcePath(graphqlApiId), {
    ...options,
    operationName: 'GetGraphQLAPI',
  });
};

/**
 * The resolved SDL, fetched separately from the detail — the spec splits it
 * out of `GraphQLAPIDetail` because it's large and not always needed
 * alongside the rest of the metadata.
 */
export const getGraphQLApiSdl = async (
  graphqlApiId: string,
  options?: RequestOptions,
): Promise<GraphQLApiSdlResponse> => {
  return http.get<GraphQLApiSdlResponse>(`${resourcePath(graphqlApiId)}/sdl`, {
    ...options,
    operationName: 'GetGraphQLAPISDL',
  });
};

export const createGraphQLApi = async (
  body: CreateGraphQLApiBody,
  options?: RequestOptions,
): Promise<GraphQLApi> => {
  return http.post<GraphQLApi>(BASE, toMultipart(body), {
    ...options,
    operationName: 'CreateGraphQLAPI',
  });
};

export const updateGraphQLApi = async (
  graphqlApiId: string,
  body: UpdateGraphQLApiBody,
  options?: RequestOptions,
): Promise<GraphQLApiDetail> => {
  return http.put<GraphQLApiDetail>(resourcePath(graphqlApiId), toMultipart(body), {
    ...options,
    operationName: 'UpdateGraphQLAPI',
  });
};

export const deleteGraphQLApi = async (
  graphqlApiId: string,
  options?: RequestOptions,
): Promise<void> => {
  return http.delete<void>(resourcePath(graphqlApiId), {
    ...options,
    operationName: 'DeleteGraphQLAPI',
  });
};

/**
 * Dry-runs schema resolution exactly as create/update would, without
 * persisting anything — the "Check"/introspection-preview action. A `resolved:
 * false` response is not a rejection: it means the declared source didn't
 * resolve (bad SDL, unreachable URL, introspection failure), reported so the
 * form can show it rather than the wizard silently creating an API with no
 * schema.
 */
export const validateGraphQLSchema = async (
  body: ValidateGraphQLSchemaBody,
  options?: RequestOptions,
): Promise<ValidateGraphQLSchemaResponse> => {
  return http.post<ValidateGraphQLSchemaResponse>(`${BASE}/validate-schema`, toMultipart(body), {
    ...options,
    operationName: 'ValidateGraphQLSchema',
  });
};
