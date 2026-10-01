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
import type { FormBodyOf, PathOf, QueryOf, ResponseOf, Schema } from '../../core/spec';

/**
 * Transport layer for user-authored API documents
 * (`/apis/{apiType}/{apiId}/docs`).
 *
 * Documents hang off the generic `/apis/{apiType}/{apiId}` path rather than a
 * per-kind one, so every call names the API's type alongside its handle — a
 * handle is unique only within its own type.
 *
 * Reads are split: `GET …/docs/{docId}` returns metadata only, and the body
 * comes from `GET …/docs/{docId}/content` as raw bytes labelled with the stored
 * content type, so the console reads it as text and lets the caller decide how
 * to render it.
 *
 * Writes are multipart, per the spec. The console always sends the body as
 * `inlineContent` (a file the user uploads is read into the editor first, so
 * they can review it before saving) and passes the original file name along
 * in `fileName` when there was one.
 */

export type ApiDocumentType = Schema<'APIDocumentType'>;
export type ApiDocumentMetadata = Schema<'APIDocumentMetadata'>;
/** Metadata of one document — no body. */
export type ApiDocument = ResponseOf<'GetAPIDocument'>;
/** A document's body as text, with the content type the server stored it under. */
export type ApiDocumentContent = TextResponse;
export type ApiDocumentListResponse = ResponseOf<'ListAPIDocuments'>;
export type ListApiDocumentsQuery = NonNullable<QueryOf<'ListAPIDocuments'>>;
export type CreateApiDocumentBody = FormBodyOf<'CreateAPIDocument'>;
export type CreateApiDocumentResponse = ResponseOf<'CreateAPIDocument'>;
export type UpdateApiDocumentBody = FormBodyOf<'UpdateAPIDocument'>;
export type UpdateApiDocumentResponse = ResponseOf<'UpdateAPIDocument'>;

type ApiTypeParam = PathOf<'ListAPIDocuments'>['apiType'];
type DocIdParam = PathOf<'GetAPIDocument'>['docId'];

/** `/apis/{apiType}/{apiId}/docs` — URL-encoded, handles are user-supplied. */
const collectionPath = (apiType: ApiTypeParam, apiId: string): string =>
  `/apis/${encodeURIComponent(apiType)}/${encodeURIComponent(apiId)}/docs`;

const resourcePath = (apiType: ApiTypeParam, apiId: string, docId: DocIdParam): string =>
  `${collectionPath(apiType, apiId)}/${encodeURIComponent(docId)}`;

/**
 * Turns a typed multipart body into `FormData`.
 *
 * Absent optional fields are omitted rather than sent as the string
 * "undefined", which the server would otherwise store as the field's value.
 */
const toFormData = (body: Record<string, unknown>): FormData => {
  const form = new FormData();
  for (const [field, value] of Object.entries(body)) {
    if (value === undefined || value === null) continue;
    form.append(field, value instanceof Blob ? value : String(value));
  }
  return form;
};

/** One page of document metadata — the list never carries document bodies. */
export const listApiDocuments = async (
  apiType: string,
  apiId: string,
  query: ListApiDocumentsQuery = {},
  options?: RequestOptions
): Promise<ApiDocumentListResponse> =>
  http.get<ApiDocumentListResponse>(collectionPath(apiType, apiId), {
    ...options,
    query,
    operationName: 'ListAPIDocuments',
  });

/** One document's metadata. The body is fetched separately — see `getApiDocumentContent`. */
export const getApiDocument = async (
  apiType: string,
  apiId: string,
  docId: string,
  options?: RequestOptions
): Promise<ApiDocument> =>
  http.get<ApiDocument>(resourcePath(apiType, apiId, docId), {
    ...options,
    operationName: 'GetAPIDocument',
  });

/**
 * One document's body, as text. A document with nothing stored answers 204,
 * which reads as empty text rather than an error.
 */
export const getApiDocumentContent = async (
  apiType: string,
  apiId: string,
  docId: string,
  options?: RequestOptions
): Promise<ApiDocumentContent> =>
  http.getText(`${resourcePath(apiType, apiId, docId)}/content`, {
    ...options,
    operationName: 'GetAPIDocumentContent',
  });

export const createApiDocument = async (
  apiType: string,
  apiId: string,
  body: CreateApiDocumentBody,
  options?: RequestOptions
): Promise<CreateApiDocumentResponse> =>
  http.post<CreateApiDocumentResponse>(collectionPath(apiType, apiId), toFormData(body), {
    ...options,
    operationName: 'CreateAPIDocument',
  });

/**
 * Updates a document. Every field is optional; leaving out both `file` and
 * `inlineContent` is a metadata-only update that does not touch the stored
 * content.
 */
export const updateApiDocument = async (
  apiType: string,
  apiId: string,
  docId: string,
  body: UpdateApiDocumentBody,
  options?: RequestOptions
): Promise<UpdateApiDocumentResponse> =>
  http.put<UpdateApiDocumentResponse>(resourcePath(apiType, apiId, docId), toFormData(body), {
    ...options,
    operationName: 'UpdateAPIDocument',
  });

export const deleteApiDocument = async (
  apiType: string,
  apiId: string,
  docId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(apiType, apiId, docId), {
    ...options,
    operationName: 'DeleteAPIDocument',
  });
};
