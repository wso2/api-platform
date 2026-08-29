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
  BodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
} from '../../core/spec';

/**
 * Transport layer for `/applications`, including its `api-keys` and
 * `associations` sub-resources. One thin function per spec operation: no
 * branching, no adapters, no cache awareness — just "call this endpoint with
 * these arguments and get the spec's response type back".
 *
 * Unlike REST APIs, applications *do* expose a per-parent key listing
 * (`ListApplicationAPIKeys`), so their keys are modelled here as a child rather
 * than through the caller-scoped `/me/api-keys` route.
 */

export type Application = Schema<'Application'>;
export type ApplicationListResponse = ResponseOf<'ListApplications'>;
export type ListApplicationsQuery = QueryOf<'ListApplications'>;
export type CreateApplicationBody = BodyOf<'CreateApplication'>;
export type UpdateApplicationBody = BodyOf<'UpdateApplication'>;

export type MappedApiKeyListResponse = ResponseOf<'ListApplicationAPIKeys'>;
export type ListApplicationApiKeysQuery = QueryOf<'ListApplicationAPIKeys'>;
export type AddApplicationApiKeysBody = BodyOf<'AddApplicationAPIKeys'>;
export type RemoveApplicationApiKeyQuery = QueryOf<'RemoveApplicationAPIKey'>;

export type ApplicationAssociationListResponse =
  ResponseOf<'ListApplicationAssociations'>;
export type ListApplicationAssociationsQuery =
  QueryOf<'ListApplicationAssociations'>;
export type AddApplicationAssociationsBody =
  BodyOf<'AddApplicationAssociations'>;
export type ListAssociationApiKeysQuery =
  QueryOf<'ListApplicationAssociationAPIKeys'>;

const BASE = '/applications';

/** URL-encoded path for one application. Handles are user-supplied — always encode. */
const resourcePath = (
  applicationId: PathOf<'GetApplication'>['applicationId']
): string => `${BASE}/${encodeURIComponent(applicationId)}`;

export const listApplications = async (
  options?: RequestOptions
): Promise<ApplicationListResponse> => {
  return http.get<ApplicationListResponse>(BASE, {
    ...options,
    operationName: 'ListApplications',
  });
};

export const getApplication = async (
  applicationId: string,
  options?: RequestOptions
): Promise<Application> => {
  return http.get<Application>(resourcePath(applicationId), {
    ...options,
    operationName: 'GetApplication',
  });
};

export const createApplication = async (
  body: CreateApplicationBody,
  options?: RequestOptions
): Promise<Application> => {
  return http.post<Application>(BASE, body, {
    ...options,
    operationName: 'CreateApplication',
  });
};

export const updateApplication = async (
  applicationId: string,
  body: UpdateApplicationBody,
  options?: RequestOptions
): Promise<Application> => {
  return http.put<Application>(resourcePath(applicationId), body, {
    ...options,
    operationName: 'UpdateApplication',
  });
};

export const deleteApplication = async (
  applicationId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(applicationId), {
    ...options,
    operationName: 'DeleteApplication',
  });
};

/* -------------------------------------------------------------------------- */
/* API keys mapped to an application                                          */
/* -------------------------------------------------------------------------- */

export const listApplicationApiKeys = async (
  applicationId: string,
  options?: RequestOptions
): Promise<MappedApiKeyListResponse> => {
  return http.get<MappedApiKeyListResponse>(
    `${resourcePath(applicationId)}/api-keys`,
    { ...options, operationName: 'ListApplicationAPIKeys' }
  );
};

export const addApplicationApiKeys = async (
  applicationId: string,
  body: AddApplicationApiKeysBody,
  options?: RequestOptions
): Promise<MappedApiKeyListResponse> => {
  return http.post<MappedApiKeyListResponse>(
    `${resourcePath(applicationId)}/api-keys`,
    body,
    { ...options, operationName: 'AddApplicationAPIKeys' }
  );
};

/**
 * Unmaps a key from an application.
 *
 * `entityID` is a **required query parameter** on this DELETE — it identifies
 * the artifact the mapping points at, which the key id alone does not. Callers
 * must pass it through `options.query`; omitting it is a 400, not a no-op.
 */
export const removeApplicationApiKey = async (
  applicationId: string,
  apiKeyId: PathOf<'RemoveApplicationAPIKey'>['apiKeyId'],
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(
    `${resourcePath(applicationId)}/api-keys/${encodeURIComponent(apiKeyId)}`,
    { ...options, operationName: 'RemoveApplicationAPIKey' }
  );
};

/* -------------------------------------------------------------------------- */
/* Associations — the providers an application is bound to                    */
/* -------------------------------------------------------------------------- */

export const listApplicationAssociations = async (
  applicationId: string,
  options?: RequestOptions
): Promise<ApplicationAssociationListResponse> => {
  return http.get<ApplicationAssociationListResponse>(
    `${resourcePath(applicationId)}/associations`,
    { ...options, operationName: 'ListApplicationAssociations' }
  );
};

export const addApplicationAssociations = async (
  applicationId: string,
  body: AddApplicationAssociationsBody,
  options?: RequestOptions
): Promise<ApplicationAssociationListResponse> => {
  return http.post<ApplicationAssociationListResponse>(
    `${resourcePath(applicationId)}/associations`,
    body,
    { ...options, operationName: 'AddApplicationAssociations' }
  );
};

export const removeApplicationAssociation = async (
  applicationId: string,
  associationId: PathOf<'RemoveApplicationAssociation'>['associationId'],
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(
    `${resourcePath(applicationId)}/associations/${encodeURIComponent(associationId)}`,
    { ...options, operationName: 'RemoveApplicationAssociation' }
  );
};

/** Keys issued for one association of an application. */
export const listAssociationApiKeys = async (
  applicationId: string,
  associationId: string,
  options?: RequestOptions
): Promise<MappedApiKeyListResponse> => {
  return http.get<MappedApiKeyListResponse>(
    `${resourcePath(applicationId)}/associations/${encodeURIComponent(associationId)}/api-keys`,
    { ...options, operationName: 'ListApplicationAssociationAPIKeys' }
  );
};
