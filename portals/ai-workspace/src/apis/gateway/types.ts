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

/**
 * Gateway interface representing a hybrid gateway
 */
export interface Gateway {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description?: string;
  vhost: string;
  isCritical: boolean;
  functionalityType: string;
  isActive: boolean;
  version?: string;
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
  /** Handle (URL-friendly slug) for the gateway. Immutable after creation. If omitted, generated from displayName. */
  id?: string;
  vhost: string;
  functionalityType: string;
  description?: string;
  version?: string;
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
  vhost: string;
  functionalityType: string;
  description?: string;
}

/**
 * Hybrid Gateway with UI-specific fields
 */
export interface HybridGateway extends Gateway {
  status?: 'connected' | 'disconnected' | 'pending';
  token?: string | null;
}
