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
 * Transport layer for `/gateways`, including its `tokens` and `manifest`
 * sub-resources. One thin function per spec operation: no branching, no
 * adapters, no cache awareness — just "call this endpoint with these arguments
 * and get the spec's response type back".
 */

export type Gateway = Schema<'GatewayResponse'>;
export type GatewayListResponse = ResponseOf<'ListGateways'>;
export type ListGatewaysQuery = QueryOf<'ListGateways'>;
export type CreateGatewayBody = BodyOf<'CreateGateway'>;
export type UpdateGatewayBody = BodyOf<'UpdateGateway'>;

export type GatewayTokenListResponse = ResponseOf<'listGatewayTokens'>;
export type ListGatewayTokensQuery = QueryOf<'listGatewayTokens'>;
export type TokenRotationResponse = ResponseOf<'rotateGatewayToken'>;
export type GatewayManifest = ResponseOf<'GetGatewayManifest'>;

const BASE = '/gateways';

/** URL-encoded path for one gateway. Ids are user-supplied — always encode. */
const resourcePath = (gatewayId: PathOf<'GetGateway'>['gatewayId']): string =>
  `${BASE}/${encodeURIComponent(gatewayId)}`;

export const listGateways = async (
  options?: RequestOptions
): Promise<GatewayListResponse> => {
  return http.get<GatewayListResponse>(BASE, {
    ...options,
    operationName: 'ListGateways',
  });
};

export const getGateway = async (
  gatewayId: string,
  options?: RequestOptions
): Promise<Gateway> => {
  return http.get<Gateway>(resourcePath(gatewayId), {
    ...options,
    operationName: 'GetGateway',
  });
};

export const createGateway = async (
  body: CreateGatewayBody,
  options?: RequestOptions
): Promise<Gateway> => {
  return http.post<Gateway>(BASE, body, {
    ...options,
    operationName: 'CreateGateway',
  });
};

export const updateGateway = async (
  gatewayId: string,
  body: UpdateGatewayBody,
  options?: RequestOptions
): Promise<Gateway> => {
  return http.put<Gateway>(resourcePath(gatewayId), body, {
    ...options,
    operationName: 'UpdateGateway',
  });
};

export const deleteGateway = async (
  gatewayId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(gatewayId), {
    ...options,
    operationName: 'DeleteGateway',
  });
};

/* -------------------------------------------------------------------------- */
/* Tokens — the credentials a self-hosted gateway agent connects with          */
/* -------------------------------------------------------------------------- */

export const listGatewayTokens = async (
  gatewayId: string,
  options?: RequestOptions
): Promise<GatewayTokenListResponse> => {
  return http.get<GatewayTokenListResponse>(`${resourcePath(gatewayId)}/tokens`, {
    ...options,
    operationName: 'listGatewayTokens',
  });
};

/**
 * Issues a new token for a gateway.
 *
 * The plaintext token is returned once, in this response, and is never
 * retrievable again — callers must surface it immediately rather than caching
 * it for later display.
 */
export const rotateGatewayToken = async (
  gatewayId: string,
  options?: RequestOptions
): Promise<TokenRotationResponse> => {
  return http.post<TokenRotationResponse>(
    `${resourcePath(gatewayId)}/tokens`,
    undefined,
    { ...options, operationName: 'rotateGatewayToken' }
  );
};

export const revokeGatewayToken = async (
  gatewayId: string,
  tokenId: PathOf<'revokeGatewayToken'>['tokenId'],
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(
    `${resourcePath(gatewayId)}/tokens/${encodeURIComponent(tokenId)}`,
    { ...options, operationName: 'revokeGatewayToken' }
  );
};

/** Deployment manifest describing what this gateway should be running. */
export const getGatewayManifest = async (
  gatewayId: string,
  options?: RequestOptions
): Promise<GatewayManifest> => {
  return http.get<GatewayManifest>(`${resourcePath(gatewayId)}/manifest`, {
    ...options,
    operationName: 'GetGatewayManifest',
  });
};
