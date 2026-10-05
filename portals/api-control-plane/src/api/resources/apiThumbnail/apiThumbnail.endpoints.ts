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

import { ApiError } from '../../core/errors';
import { http, type BlobResponse, type RequestOptions } from '../../core/http';
import type { PathOf } from '../../core/spec';

/**
 * Transport layer for an artifact's singleton thumbnail
 * (`/apis/{apiType}/{apiId}/thumbnail`).
 *
 * There's no POST — a thumbnail is one-per-API, so `PUT` is an upsert that
 * creates or replaces atomically. `GET` returns the bytes as a `Blob`; the
 * caller wraps it in a URL for an `<img>` tag. `DELETE` removes the stored
 * image and the UI falls back to the API's name-initials avatar.
 *
 * Content-type is sniffed server-side — the uploader's declared
 * `Content-Type` and filename extension are ignored for the type decision.
 */

type ApiTypeParam = PathOf<'GetAPIThumbnail'>['apiType'];

/** `/apis/{apiType}/{apiId}/thumbnail` — URL-encoded, handles are user-supplied. */
const resourcePath = (apiType: ApiTypeParam, apiId: string): string =>
  `/apis/${encodeURIComponent(apiType)}/${encodeURIComponent(apiId)}/thumbnail`;

/**
 * One API's stored thumbnail bytes, as a Blob plus the sniffed content type,
 * or `null` when no thumbnail is set.
 */
export const getApiThumbnail = async (
  apiType: string,
  apiId: string,
  options?: RequestOptions
): Promise<BlobResponse | null> => {
  try {
    return await http.getBlob(resourcePath(apiType, apiId), {
      ...options,
      operationName: 'GetAPIThumbnail',
    });
  } catch (err) {
    if (err instanceof ApiError && err.code === 'NOT_FOUND') return null;
    throw err;
  }
};

/**
 * Creates or replaces the API's thumbnail. The body is a multipart form with
 * a single `file` field; the server rejects anything that isn't JPEG or PNG.
 */
export const upsertApiThumbnail = async (
  apiType: string,
  apiId: string,
  file: Blob,
  options?: RequestOptions
): Promise<void> => {
  const form = new FormData();
  form.append('file', file);
  await http.put<void>(resourcePath(apiType, apiId), form, {
    ...options,
    operationName: 'UpsertAPIThumbnail',
  });
};

export const deleteApiThumbnail = async (
  apiType: string,
  apiId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(apiType, apiId), {
    ...options,
    operationName: 'DeleteAPIThumbnail',
  });
};
