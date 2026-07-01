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

import {
  get as getRequest,
  post,
  put,
  del,
} from '../../clients/choreoApiClient';
import { PLATFORM_API_BASE_URL } from '../../config.env';
import {
  RegisterGatewayRequest,
  RegisterGatewayResponse,
  GatewayListResponse,
  GatewayConfigs,
  Gateway,
  UpdateGatewayRequest,
} from './types';

export * from './types';

const GATEWAY_API_PATH = '/gateways';

/**
 * Helper to transform camelCase request to API format
 */
function transformRequestToApiFormat(
  request: RegisterGatewayRequest | UpdateGatewayRequest
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    displayName: request.displayName,
    vhost: request.vhost,
    functionalityType: request.functionalityType,
    description: request.description,
  };

  if ('id' in request && request.id) {
    payload.id = request.id;
  }

  if ('version' in request && request.version) {
    payload.version = request.version;
  }

  if ('environment' in request && request.environment) {
    payload.properties = {
      environment: request.environment,
    };
  }

  return payload;
}

/**
 * Get all gateways for an organization
 */
export async function getGateways(
  organizationId: string
): Promise<{ data: GatewayListResponse }> {
  const data = await getRequest<GatewayListResponse>(
    `${GATEWAY_API_PATH}`,
    undefined,
    PLATFORM_API_BASE_URL
  );

  return { data };
}

// TODO: /gateways/{gatewayId}/configs does not exist in openapi.yaml — needs backend clarification
/**
 * Get gateway configurations by gateway ID
 */
export async function getGatewayConfigs(
  gatewayId: string,
  organizationId: string
): Promise<{ data: GatewayConfigs }> {
  const data = await getRequest<GatewayConfigs>(
    `${GATEWAY_API_PATH}/${encodeURIComponent(gatewayId)}/configs`,
    undefined,
    PLATFORM_API_BASE_URL
  );

  return { data };
}

/**
 * Get a specific gateway by ID
 */
export async function getGatewayById(
  gatewayId: string,
  organizationId: string
): Promise<{ data: Gateway }> {
  const data = await getRequest<Gateway>(
    `${GATEWAY_API_PATH}/${gatewayId}`,
    undefined,
    PLATFORM_API_BASE_URL
  );

  return { data };
}

/**
 * Register a new gateway
 */
export async function registerGateway(
  gatewayData: RegisterGatewayRequest,
  organizationId: string
): Promise<{ data: RegisterGatewayResponse }> {
  const apiPayload = transformRequestToApiFormat(gatewayData);

  const data = await post<RegisterGatewayResponse>(
    `${GATEWAY_API_PATH}`,
    apiPayload,
    PLATFORM_API_BASE_URL
  );

  return { data };
}

/**
 * Update an existing gateway
 */
export async function updateGateway(
  gatewayId: string,
  gatewayData: UpdateGatewayRequest,
  organizationId: string
): Promise<{ data: Gateway }> {
  const apiPayload = transformRequestToApiFormat(gatewayData);

  const data = await put<Gateway>(
    `${GATEWAY_API_PATH}/${gatewayId}`,
    apiPayload,
    PLATFORM_API_BASE_URL
  );

  return { data };
}

/**
 * Delete a gateway
 */
export async function deleteGateway(
  gatewayId: string,
  organizationId: string
): Promise<{ data: void }> {
  await del<void>(
    `${GATEWAY_API_PATH}/${gatewayId}`,
    undefined,
    PLATFORM_API_BASE_URL
  );

  return { data: undefined };
}

// ----- Gateway Token Management -----

export interface GatewayToken {
  id: string;
  status: string;
  createdAt: string;
}

type GatewayTokenListResponse = GatewayToken[];

/**
 * List all tokens for a gateway
 */
export async function listGatewayTokens(
  gatewayId: string,
  organizationId: string
): Promise<GatewayTokenListResponse> {
  const data = await getRequest<GatewayTokenListResponse>(
    `${GATEWAY_API_PATH}/${gatewayId}/tokens`,
    undefined,
    PLATFORM_API_BASE_URL
  );

  return data || [];
}

/**
 * Revoke a specific gateway token
 */
export async function revokeGatewayToken(
  gatewayId: string,
  tokenId: string,
  organizationId: string
): Promise<void> {
  await del<void>(
    `${GATEWAY_API_PATH}/${gatewayId}/tokens/${tokenId}`,
    undefined,
    PLATFORM_API_BASE_URL
  );
}

interface RotateGatewayTokenResponse {
  token: string;
}

/**
 * Rotate (regenerate) gateway token - creates a new token
 */
export async function rotateGatewayToken(
  gatewayId: string,
  organizationId: string
): Promise<string> {
  const data = await post<RotateGatewayTokenResponse>(
    `${GATEWAY_API_PATH}/${gatewayId}/tokens`,
    {},
    PLATFORM_API_BASE_URL
  );

  return data.token;
}
