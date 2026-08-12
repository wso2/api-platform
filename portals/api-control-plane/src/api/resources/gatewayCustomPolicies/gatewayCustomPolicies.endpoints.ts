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
 * Transport layer for `/gateway-custom-policies` — mediation policies an
 * operator has published to a gateway. One thin function per spec operation:
 * no branching, no adapters, no cache awareness.
 *
 * Two things set this resource apart from the others:
 *
 * - **Policies are version-keyed.** A policy is identified by id *and* version
 *   (`/{gatewayCustomPolicyId}/versions/{version}`), so both are required to
 *   read or delete one, and both belong in its cache key.
 * - **There is no create or update.** Policies arrive by being synced from a
 *   gateway, not authored through this API — hence `syncCustomPolicy` in place
 *   of the usual POST/PUT pair.
 */

export type CustomPolicy = Schema<'CustomPolicyResponse'>;
export type CustomPolicyListResponse = ResponseOf<'ListGatewayCustomPolicies'>;
export type ListCustomPoliciesQuery = QueryOf<'ListGatewayCustomPolicies'>;
export type SyncCustomPolicyQuery = QueryOf<'SyncCustomPolicy'>;

const BASE = '/gateway-custom-policies';

/**
 * URL-encoded path for one policy version. Both segments are user-supplied —
 * always encode.
 */
const resourcePath = (
  gatewayCustomPolicyId: PathOf<'GetGatewayCustomPolicy'>['gatewayCustomPolicyId'],
  version: PathOf<'GetGatewayCustomPolicy'>['version']
): string =>
  `${BASE}/${encodeURIComponent(gatewayCustomPolicyId)}/versions/${encodeURIComponent(version)}`;

export const listGatewayCustomPolicies = async (
  options?: RequestOptions
): Promise<CustomPolicyListResponse> => {
  return http.get<CustomPolicyListResponse>(BASE, {
    ...options,
    operationName: 'ListGatewayCustomPolicies',
  });
};

export const getGatewayCustomPolicy = async (
  gatewayCustomPolicyId: string,
  version: string,
  options?: RequestOptions
): Promise<CustomPolicy> => {
  return http.get<CustomPolicy>(resourcePath(gatewayCustomPolicyId, version), {
    ...options,
    operationName: 'GetGatewayCustomPolicy',
  });
};

/**
 * Deletes one version of a policy.
 *
 * Other versions of the same policy are unaffected — this removes a single
 * version, not the policy as a whole.
 */
export const deleteGatewayCustomPolicy = async (
  gatewayCustomPolicyId: string,
  version: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(gatewayCustomPolicyId, version), {
    ...options,
    operationName: 'DeleteGatewayCustomPolicy',
  });
};

/**
 * Pulls a policy definition from a gateway into the control plane.
 *
 * All three of `gatewayId`, `policyName` and `policyVersion` are **required
 * query parameters** — this POST carries no path segment and no body, which is
 * unusual enough to be worth stating rather than leaving to be discovered from
 * a 400.
 */
export const syncCustomPolicy = async (
  options?: RequestOptions
): Promise<CustomPolicy> => {
  return http.post<CustomPolicy>(`${BASE}/sync`, undefined, {
    ...options,
    operationName: 'SyncCustomPolicy',
  });
};
