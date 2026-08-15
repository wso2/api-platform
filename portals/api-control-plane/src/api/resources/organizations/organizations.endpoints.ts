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
 * Transport layer for `/organizations`. One thin function per spec operation:
 * no branching, no adapters, no cache awareness — just "call this endpoint
 * with these arguments and get the spec's response type back".
 *
 * Unlike every other resource, these endpoints are **not** organization-scoped.
 * `/organizations` is what the org switcher reads before an organization is
 * known, so callers pass no `orgId` and the request deliberately carries no
 * `X-Org-Id` header.
 */

export type Organization = Schema<'Organization'>;
export type OrganizationListResponse = ResponseOf<'ListOrganizations'>;
export type ListOrganizationsQuery = QueryOf<'ListOrganizations'>;
export type RegisterOrganizationBody = BodyOf<'RegisterOrganization'>;

const BASE = '/organizations';

/** URL-encoded path for one organization. Ids are user-supplied — always encode. */
const resourcePath = (
  organizationId: PathOf<'GetOrganization'>['organizationId']
): string => `${BASE}/${encodeURIComponent(organizationId)}`;

/**
 * Options for a global (non-organization-scoped) operation: everything
 * `RequestOptions` offers except the org scope, which has no meaning here.
 */
export type GlobalRequestOptions = Omit<RequestOptions, 'orgId'>;

/**
 * Removes `orgId` so no `X-Org-Id` header is sent.
 */
const withoutOrgScope = (
  options?: GlobalRequestOptions
): GlobalRequestOptions => {
  if (!options) return {};
  const { orgId: _orgId, ...rest } = options as RequestOptions;
  void _orgId;
  return rest;
};

export const listOrganizations = async (
  options?: GlobalRequestOptions
): Promise<OrganizationListResponse> => {
  return http.get<OrganizationListResponse>(BASE, {
    ...withoutOrgScope(options),
    operationName: 'ListOrganizations',
  });
};

export const getOrganization = async (
  organizationId: string,
  options?: GlobalRequestOptions
): Promise<Organization> => {
  return http.get<Organization>(resourcePath(organizationId), {
    ...withoutOrgScope(options),
    operationName: 'GetOrganization',
  });
};

/** Onboarding only: registers a new organization and returns it. */
export const registerOrganization = async (
  body: RegisterOrganizationBody,
  options?: GlobalRequestOptions
): Promise<Organization> => {
  return http.post<Organization>(BASE, body, {
    ...withoutOrgScope(options),
    operationName: 'RegisterOrganization',
  });
};
