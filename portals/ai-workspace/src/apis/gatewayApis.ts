/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { get, post, del } from '../clients/choreoApiClient';
import { PLATFORM_API_BASE_URL } from '../config.env';
import { logger } from '../utils/logger';
import type {
  GatewayListResponse,
  GatewayConfigs,
  DeploymentListResponseToPlatformGateway,
  DeploymentResponseToPlatformGateway,
  DeployAPIToPlatformGatewayRequest,
} from './gatewayTypes';

/**
 * Get all gateways for an organization.
 */
export async function getGateways(
  organizationId: string
): Promise<GatewayListResponse> {
  try {
    const response = await get<GatewayListResponse>(
      `/gateways?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch gateways:', error);
    throw error;
  }
}

/**
 * Get gateway configurations by gateway handle.
 */
export async function getGatewayConfigs(
  gatewayHandle: string,
  organizationId: string
): Promise<GatewayConfigs> {
  try {
    const response = await get<GatewayConfigs>(
      `/gateways/${encodeURIComponent(gatewayHandle)}/configs?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch gateway configs:', error);
    throw error;
  }
}

interface RawDeploymentListResponse {
  count: number;
  list?: DeploymentResponseToPlatformGateway[];
  deployments?: DeploymentResponseToPlatformGateway[];
}

function normalizeDeploymentListResponse(
  raw: RawDeploymentListResponse
): DeploymentListResponseToPlatformGateway {
  return {
    count: raw.count,
    deployments: raw.deployments ?? raw.list ?? [],
  };
}

/**
 * Fetch all deployments of an API to platform gateways
 */
export async function getApiPlatformGatewayDeployments(
  apiId: string,
  organizationId: string
): Promise<DeploymentListResponseToPlatformGateway> {
  try {
    const raw = await get<RawDeploymentListResponse>(
      `/apis/${apiId}/platform-gateway-deployments?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
    return normalizeDeploymentListResponse(raw);
  } catch (error) {
    logger.error('Failed to fetch platform gateway deployments:', error);
    throw error;
  }
}

/**
 * Deploy an API to a platform gateway
 */
export async function deployApiToPlatformGateway(
  apiId: string,
  organizationId: string,
  requestBody: DeployAPIToPlatformGatewayRequest
): Promise<DeploymentResponseToPlatformGateway> {
  try {
    const response = await post<DeploymentResponseToPlatformGateway>(
      `/apis/${apiId}/platform-gateway-deployments?organizationId=${encodeURIComponent(organizationId)}`,
      requestBody,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to deploy API to platform gateway:', error);
    throw error;
  }
}

/**
 * Restore a deployment on a platform gateway
 */
export async function restorePlatformGatewayDeployment(
  apiId: string,
  organizationId: string,
  deploymentId: string,
  gatewayHandle: string
): Promise<DeploymentResponseToPlatformGateway> {
  try {
    const response = await post<DeploymentResponseToPlatformGateway>(
      `/apis/${apiId}/platform-gateway-deployments/restore?organizationId=${encodeURIComponent(organizationId)}&deploymentId=${encodeURIComponent(deploymentId)}&gatewayHandle=${encodeURIComponent(gatewayHandle)}`,
      null,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to restore platform gateway deployment:', error);
    throw error;
  }
}

/**
 * Undeploy a deployment from a platform gateway
 */
export async function undeployFromPlatformGateway(
  apiId: string,
  organizationId: string,
  deploymentId: string,
  gatewayHandle: string
): Promise<DeploymentResponseToPlatformGateway> {
  try {
    const response = await post<DeploymentResponseToPlatformGateway>(
      `/apis/${apiId}/platform-gateway-deployments/undeploy?organizationId=${encodeURIComponent(organizationId)}&deploymentId=${encodeURIComponent(deploymentId)}&gatewayHandle=${encodeURIComponent(gatewayHandle)}`,
      null,
      PLATFORM_API_BASE_URL
    );
    return response;
  } catch (error) {
    logger.error('Failed to undeploy from platform gateway:', error);
    throw error;
  }
}

/**
 * Delete a deployment from a platform gateway
 */
export async function deletePlatformGatewayDeployment(
  apiId: string,
  organizationId: string,
  deploymentId: string
): Promise<void> {
  try {
    await del<void>(
      `/apis/${apiId}/platform-gateway-deployments/${deploymentId}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      PLATFORM_API_BASE_URL
    );
  } catch (error) {
    logger.error('Failed to delete platform gateway deployment:', error);
    throw error;
  }
}
