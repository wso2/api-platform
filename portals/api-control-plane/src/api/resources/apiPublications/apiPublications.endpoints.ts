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

import { http, type RequestOptions, type TextResponse } from '../../core/http';
import type { QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for the API Publications feature: the `/api-publications`
 * rollup, one API's draft/publication on one portal, and the publish/unpublish/
 * deprecate actions. One thin function per spec operation, typed by `operationId`.
 */

export type PublicationSummaryItem = Schema<'PublicationSummaryItem'>;
export type PublicationStatus = NonNullable<PublicationSummaryItem['status']>;
export type ListApiPublicationsResponse = ResponseOf<'listApiPublications'>;
export type ListApiPublicationsQuery = QueryOf<'listApiPublications'>;

export type PublicationDraftDetails = Schema<'PublicationDraftDetails'>;
export type PublicationDraftDetailsInput = Schema<'PublicationDraftDetailsInput'>;
export type Publication = Schema<'Publication'>;

/**
 * The draft definition as this app writes it: a parsed JSON document, not the
 * generated `string` the `format: binary` field produces.
 *
 * Deliberately not `BodyOf`: same class of deviation as `CreateRestApiBody` in
 * `restApis.endpoints.ts`. The spec's request body resolves to a bare `string`
 * (one `format: binary` field per accepted content type), which is technically
 * accurate but useless here: `http.put` always `JSON.stringify`s whatever body
 * it's given, so passing a *parsed* object is what puts the right bytes on the
 * wire for the `application/json` variant this app uses; passing the generated
 * `string` type would double-encode it.
 */
export type DraftDefinitionDocument = Record<string, unknown>;

/**
 * A stored definition as read back: the server returns it in whichever
 * serialization it was saved in (JSON, YAML, ...), so it stays text with its
 * content type and the caller decides how to read it.
 */
export type DefinitionText = TextResponse;

const PUBLICATIONS_BASE = '/api-publications';

/** `/api-portals/{apiPortalId}/apis/{apiType}/{apiId}{suffix}` — URL-encoded, handles are user-supplied. */
const portalApiPath = (apiPortalId: string, apiType: string, apiId: string, suffix = ''): string =>
  `/api-portals/${encodeURIComponent(apiPortalId)}/apis/${encodeURIComponent(apiType)}/${encodeURIComponent(apiId)}${suffix}`;

/** The only API type the publish, unpublish and deprecate actions support today. */
export const REST_API_TYPE = 'rest-api';

const restApiPortalActionPath = (apiPortalId: string, apiId: string, action: string): string =>
  portalApiPath(apiPortalId, REST_API_TYPE, apiId, `/${action}`);

export const listApiPublications = async (
  query: ListApiPublicationsQuery,
  options?: RequestOptions,
): Promise<ListApiPublicationsResponse> =>
  http.get<ListApiPublicationsResponse>(PUBLICATIONS_BASE, {
    ...options,
    query,
    operationName: 'listApiPublications',
  });

export const getApiPublicationDraft = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  options?: RequestOptions,
): Promise<PublicationDraftDetails> =>
  http.get<PublicationDraftDetails>(portalApiPath(apiPortalId, apiType, apiId, '/draft'), {
    ...options,
    operationName: 'getApiPublicationDraft',
  });

export const saveApiPublicationDraft = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  body: PublicationDraftDetailsInput,
  options?: RequestOptions,
): Promise<PublicationDraftDetails> =>
  http.put<PublicationDraftDetails>(portalApiPath(apiPortalId, apiType, apiId, '/draft'), body, {
    ...options,
    operationName: 'saveApiPublicationDraft',
  });

export const getApiPublicationDraftDefinition = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  options?: RequestOptions,
): Promise<DefinitionText> =>
  http.getText(portalApiPath(apiPortalId, apiType, apiId, '/draft/definition'), {
    ...options,
    operationName: 'getApiPublicationDraftDefinition',
  });

export const saveApiPublicationDraftDefinition = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  body: DraftDefinitionDocument,
  options?: RequestOptions,
): Promise<void> =>
  http.put<void>(portalApiPath(apiPortalId, apiType, apiId, '/draft/definition'), body, {
    ...options,
    operationName: 'saveApiPublicationDraftDefinition',
  });

/** The published definition — the fallback tier once no draft definition exists. */
export const getApiPublicationDefinition = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  options?: RequestOptions,
): Promise<DefinitionText> =>
  http.getText(portalApiPath(apiPortalId, apiType, apiId, '/publication/definition'), {
    ...options,
    operationName: 'getApiPublicationDefinition',
  });

export const getApiPublication = async (
  apiPortalId: string,
  apiType: string,
  apiId: string,
  options?: RequestOptions,
): Promise<Publication> =>
  http.get<Publication>(portalApiPath(apiPortalId, apiType, apiId, '/publication'), {
    ...options,
    operationName: 'getApiPublication',
  });

/** Publishes (or republishes) the current draft. The request has no body; the server publishes what the draft already holds. */
export const publishRestApiToApiPortal = async (
  apiPortalId: string,
  apiId: string,
  options?: RequestOptions,
): Promise<Publication> =>
  http.post<Publication>(restApiPortalActionPath(apiPortalId, apiId, 'publish'), undefined, {
    ...options,
    operationName: 'publishRestApiToApiPortal',
  });

/** Removes the live listing. Valid only when currently published or deprecated. */
export const unpublishRestApiFromApiPortal = async (
  apiPortalId: string,
  apiId: string,
  options?: RequestOptions,
): Promise<void> =>
  http.post<void>(restApiPortalActionPath(apiPortalId, apiId, 'unpublish'), undefined, {
    ...options,
    operationName: 'unpublishRestApiFromApiPortal',
  });

/** Marks the live listing deprecated — still visible on the portal, flagged as deprecated. Valid only when currently published. */
export const deprecateRestApiOnApiPortal = async (
  apiPortalId: string,
  apiId: string,
  options?: RequestOptions,
): Promise<Publication> =>
  http.post<Publication>(restApiPortalActionPath(apiPortalId, apiId, 'deprecate'), undefined, {
    ...options,
    operationName: 'deprecateRestApiOnApiPortal',
  });
