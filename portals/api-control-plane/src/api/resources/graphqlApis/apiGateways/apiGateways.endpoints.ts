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

import { http, type RequestOptions } from '../../../core/http';
import type { BodyOf, ItemOf, PathOf, QueryOf, ResponseOf } from '../../../core/spec';

/**
 * Wire calls for a GraphQL API's gateway associations — the read side of
 * `restApis/apiGateways`, which the GraphQL spec shares (same response
 * schema, each item carrying `isDeployed`). Scoped to the API, so it needs
 * only `ap:graphql_api:gateway:read`, not organization-wide gateway access.
 */

export type GraphQLApiGatewayListResponse = ResponseOf<'GetGraphQLAPIGateways'>;
export type GraphQLApiGateway = ItemOf<'GetGraphQLAPIGateways'>;
export type ListGraphQLApiGatewaysQuery = QueryOf<'GetGraphQLAPIGateways'>;
export type InvokeGraphQLApiOnGatewayBody = BodyOf<'InvokeGraphQLAPIOnGateway'>;
export type InvokeGraphQLApiOnGatewayResponse = ResponseOf<'InvokeGraphQLAPIOnGateway'>;

const collectionPath = (graphqlApiId: PathOf<'GetGraphQLAPIGateways'>['graphqlApiId']): string =>
  `/graphql-apis/${encodeURIComponent(graphqlApiId)}/gateways`;

export const listGraphQLApiGateways = async (
  graphqlApiId: string,
  options?: RequestOptions,
): Promise<GraphQLApiGatewayListResponse> => {
  return http.get<GraphQLApiGatewayListResponse>(collectionPath(graphqlApiId), {
    ...options,
    operationName: 'GetGraphQLAPIGateways',
  });
};

/**
 * Sends one GraphQL request to the GraphQL API (`body.graphqlApiId`) as deployed
 * on the gateway (`body.gatewayId`). Any answer from the gateway, whatever its
 * status, resolves: it is carried in `response`. A rejection means the request
 * was not relayed.
 */
export const invokeGraphQLApiOnGateway = async (
  body: InvokeGraphQLApiOnGatewayBody,
  options?: RequestOptions,
): Promise<InvokeGraphQLApiOnGatewayResponse> => {
  return http.post<InvokeGraphQLApiOnGatewayResponse>('/gateway-relay/invoke-graphql', body, {
    ...options,
    operationName: 'InvokeGraphQLAPIOnGateway',
  });
};
