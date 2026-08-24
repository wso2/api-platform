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
import type {
  FormBodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
} from '../../core/spec';

/**
 * Transport layer for `/secrets`.
 *
 * These are the only two multipart operations in the entire spec: `createSecret`
 * and `rotateSecret` send `multipart/form-data`, not JSON. The transport already
 * knows to leave `Content-Type` alone for a `FormData` body so the browser can
 * set its own boundary — this module's job is to build that `FormData` from the
 * spec's typed shape rather than letting callers assemble one by hand.
 *
 * Caveat worth knowing before changing anything here: jsdom's XHR does not
 * implement `FormData` bodies, so the multipart wire format cannot be asserted
 * in the unit suite (plain axios behaves identically there). These two paths
 * need a browser-run test.
 */

export type SecretSummary = Schema<'SecretSummary'>;
export type SecretResponse = Schema<'SecretResponse'>;
export type SecretListResponse = ResponseOf<'listSecrets'>;
export type ListSecretsQuery = QueryOf<'listSecrets'>;
export type CreateSecretBody = FormBodyOf<'createSecret'>;
export type RotateSecretBody = FormBodyOf<'rotateSecret'>;

const BASE = '/secrets';

const resourcePath = (secretId: PathOf<'getSecret'>['secretId']): string =>
  `${BASE}/${encodeURIComponent(secretId)}`;

/**
 * Turns a typed multipart body into `FormData`.
 *
 * Absent optional fields are omitted rather than sent as the string
 * "undefined", which is what a naive loop would produce and what the server
 * would then store as the field's value.
 */
const toFormData = (body: Record<string, unknown>): FormData => {
  const form = new FormData();
  for (const [field, value] of Object.entries(body)) {
    if (value === undefined || value === null) continue;
    form.append(field, value instanceof Blob ? value : String(value));
  }
  return form;
};

export const listSecrets = async (
  options?: RequestOptions
): Promise<SecretListResponse> => {
  return http.get<SecretListResponse>(BASE, {
    ...options,
    operationName: 'listSecrets',
  });
};

/** Metadata only — the secret's value is never returned by a read. */
export const getSecret = async (
  secretId: string,
  options?: RequestOptions
): Promise<SecretSummary> => {
  return http.get<SecretSummary>(resourcePath(secretId), {
    ...options,
    operationName: 'getSecret',
  });
};

/**
 * Creates a secret. Sent as multipart, per the spec.
 *
 * The response carries the stored secret's metadata; the value the caller
 * supplied is never echoed back and cannot be read again afterwards.
 */
export const createSecret = async (
  body: CreateSecretBody,
  options?: RequestOptions
): Promise<SecretResponse> => {
  return http.post<SecretResponse>(BASE, toFormData(body), {
    ...options,
    operationName: 'createSecret',
  });
};

/** Replaces a secret's value in place, keeping its id and references. */
export const rotateSecret = async (
  secretId: string,
  body: RotateSecretBody,
  options?: RequestOptions
): Promise<SecretResponse> => {
  return http.put<SecretResponse>(resourcePath(secretId), toFormData(body), {
    ...options,
    operationName: 'rotateSecret',
  });
};

/**
 * Deletes a secret.
 *
 * The backend refuses while other resources still reference it, answering with
 * `SECRET_IN_USE` and listing the blocking resources in `ApiError.details`.
 */
export const deleteSecret = async (
  secretId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(secretId), {
    ...options,
    operationName: 'deleteSecret',
  });
};
