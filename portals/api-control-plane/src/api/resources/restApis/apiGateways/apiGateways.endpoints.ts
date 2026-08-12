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
import type { BodyOf, PathOf, QueryOf, ResponseOf } from '../../../core/spec';

/**
 * Transport layer for `/rest-apis/{restApiId}/gateways` — which gateways an API
 * is associated with.
 *
 * This is the association, not the gateway itself: `resources/gateways`
 * manages gateways as entities, while this module manages the link between one
 * API and them. Associating an API with a gateway is what makes it deployable
 * there, so this sits upstream of `deployments`.
 */

export type RestApiGatewayListResponse = ResponseOf<'GetRESTAPIGateways'>;
export type ListRestApiGatewaysQuery = QueryOf<'GetRESTAPIGateways'>;
export type AddGatewaysToApiBody = BodyOf<'AddGatewaysToAPI'>;

const collectionPath = (
  restApiId: PathOf<'GetRESTAPIGateways'>['restApiId']
): string => `/rest-apis/${encodeURIComponent(restApiId)}/gateways`;

export const listRestApiGateways = async (
  restApiId: string,
  options?: RequestOptions
): Promise<RestApiGatewayListResponse> => {
  return http.get<RestApiGatewayListResponse>(collectionPath(restApiId), {
    ...options,
    operationName: 'GetRESTAPIGateways',
  });
};

/**
 * Associates gateways with an API.
 *
 * The body is an **array** of associations, not a single object — the spec
 * models this as a bulk add, and it returns the API's full gateway list rather
 * than only what was just added.
 */
export const addGatewaysToApi = async (
  restApiId: string,
  body: AddGatewaysToApiBody,
  options?: RequestOptions
): Promise<RestApiGatewayListResponse> => {
  return http.post<RestApiGatewayListResponse>(collectionPath(restApiId), body, {
    ...options,
    operationName: 'AddGatewaysToAPI',
  });
};
