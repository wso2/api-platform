/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import {
  del,
  get,
  post,
} from '../clients/choreoApiClient';

const GATEWAY_API_PATH = '/gateways';
const CUSTOM_POLICY_API_PATH = '/gateway-custom-policies';

export interface GatewayManifestPolicy {
  name: string;
  version: string;
  displayName?: string;
  description?: string;
  managedBy?: string;
  /** @deprecated Older Platform API versions returned this boolean. */
  isCustomPolicy?: boolean;
  policyDefinition?: Record<string, unknown>;
}

export interface GatewayPolicyManifest {
  policies?: GatewayManifestPolicy[];
}

export interface GatewayCustomPolicy {
  uuid: string;
  organizationUuid: string;
  name: string;
  displayName?: string;
  version: string;
  description?: string;
  provider?: string;
  policyDefinition: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
}

export interface GatewayCustomPolicyListResponse {
  count: number;
  list: GatewayCustomPolicy[];
  pagination: {
    limit: number;
    offset: number;
    total: number;
  };
}

/** Get the policy manifest installed on a gateway. */
export const getGatewayPolicyManifest = (
  gatewayId: string,
): Promise<GatewayPolicyManifest> =>
  get<GatewayPolicyManifest>(
    `${GATEWAY_API_PATH}/${encodeURIComponent(gatewayId)}/manifest`,
  );

/** List custom policies synced to the organization in the current session. */
export const getGatewayCustomPolicies = (): Promise<GatewayCustomPolicyListResponse> =>
  get<GatewayCustomPolicyListResponse>(CUSTOM_POLICY_API_PATH);

/** Sync a custom policy from a gateway manifest into the organization. */
export const syncGatewayCustomPolicy = (
  gatewayId: string,
  policyName: string,
  policyVersion: string,
): Promise<GatewayCustomPolicy> => {
  const params = new URLSearchParams({
    gatewayId,
    policyName,
    policyVersion,
  });

  return post<GatewayCustomPolicy>(
    `${CUSTOM_POLICY_API_PATH}/sync?${params.toString()}`,
  );
};

/** Get one custom-policy version from the current organization. */
export const getGatewayCustomPolicy = (
  gatewayCustomPolicyId: string,
  version: string,
): Promise<GatewayCustomPolicy> =>
  get<GatewayCustomPolicy>(
    `${CUSTOM_POLICY_API_PATH}/${encodeURIComponent(gatewayCustomPolicyId)}/versions/${encodeURIComponent(version)}`,
  );

/** Delete one custom-policy version from the current organization. */
export const deleteGatewayCustomPolicy = (
  gatewayCustomPolicyId: string,
  version: string,
): Promise<void> =>
  del<void>(
    `${CUSTOM_POLICY_API_PATH}/${encodeURIComponent(gatewayCustomPolicyId)}/versions/${encodeURIComponent(version)}`,
  );

const gatewayPolicyApis = {
  getGatewayPolicyManifest,
  getGatewayCustomPolicies,
  syncGatewayCustomPolicy,
  getGatewayCustomPolicy,
  deleteGatewayCustomPolicy,
};

export default gatewayPolicyApis;
