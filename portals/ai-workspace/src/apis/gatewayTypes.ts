/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://www.wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

/**
 * Gateway interface representing a hybrid gateway
 */
export interface Gateway {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  endpoints?: string[];
  /** @deprecated Use `endpoints` instead. Kept for gateways created before the `endpoints` field existed. */
  vhost?: string;
  isCritical: boolean;
  functionalityType: string;
  isActive: boolean;
  createdAt?: string;
  updatedAt?: string;
  properties?: Record<string, string>;
}

/**
 * Response interface for listing gateways
 */
export interface GatewayListResponse {
  count: number;
  list: Gateway[];
}

/**
 * Gateway configurations response
 */
export interface GatewayConfig {
  name?: string;
  issuerUrl?: string;
  jwksUrl?: string;
  moesifKey?: string;
  [key: string]: string | undefined;
}

export type GatewayConfigs = GatewayConfig | GatewayConfig[];

/**
 * Request interface for registering a new gateway
 */
export interface RegisterGatewayRequest {
  displayName: string;
  name: string;
  vhost: string;
  functionalityType: string;
  description?: string;
  sandboxVhost?: string;
  environment?: string;
}

/**
 * Response interface for gateway registration
 */
export interface RegisterGatewayResponse extends Gateway {
  token?: string | null;
}

/**
 * Request interface for updating a gateway
 */
export interface UpdateGatewayRequest {
  displayName: string;
  name: string;
  vhost: string;
  functionalityType: string;
  description?: string;
}

/**
 * Deployment information for a gateway
 */
export interface GatewayDeploymentInfo {
  revisionId: string;
  status: string;
  deployedAt: string;
}

/**
 * Deployed Gateway interface - Gateway with deployment status
 */
export interface DeployedGateway extends Gateway {
  associatedAt?: string;
  isDeployed: boolean;
  deployment?: GatewayDeploymentInfo;
}

/**
 * Response interface for listing deployed gateways for an API
 */
export interface DeployedGatewayListResponse {
  count: number;
  list: DeployedGateway[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
}

export enum DeploymentResponseToPlatformGatewayStatusEnum {
  DEPLOYED = 'DEPLOYED',
  UNDEPLOYED = 'UNDEPLOYED',
  ARCHIVED = 'ARCHIVED',
  DEPLOYING = 'DEPLOYING',
  UNDEPLOYING = 'UNDEPLOYING',
  FAILED = 'FAILED',
}

export interface DeploymentResponseToPlatformGateway {
  deploymentId: string;
  name?: string;
  apiId?: string;
  gatewayId?: string;
  status: string;
  statusReason?: string | null;
  createdAt?: string;
  metadata?: Record<string, unknown>;
}

export interface DeploymentListResponseToPlatformGateway {
  count: number;
  deployments: DeploymentResponseToPlatformGateway[];
}

export interface DeployAPIToPlatformGatewayRequest {
  name: string;
  base: string;
  gatewayId: string;
  metadata?: object;
}

/**
 * HybridGateway extends Gateway with UI-specific fields
 */
export interface HybridGateway extends Gateway {
  status?: 'connected' | 'disconnected' | 'pending';
  token?: string | null;
}

/**
 * Gateway deployment status interface
 */
export interface GatewayDeployment {
  gatewayId: string;
  status: 'DEPLOYED' | 'UNDEPLOYED' | 'ARCHIVED' | 'DEPLOYING' | 'UNDEPLOYING' | 'FAILED';
  statusReason?: string | null;
  buildId?: string;
  revisionId?: string;
  deploymentId?: string;
  deployedTime?: string;
  apiId?: string;
}
