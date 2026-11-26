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
import type { BodyOf, ResponseOf } from '../../core/spec';

/**
 * Transport for `/gateway-relay/invoke`: the Test consoles' request to a deployed
 * REST or GraphQL API, sent by the Platform API rather than the browser.
 */

export type InvokeApiOnGatewayBody = BodyOf<'InvokeAPIOnGateway'>;
export type InvokeApiOnGatewayResponse = ResponseOf<'InvokeAPIOnGateway'>;

/**
 * Sends one test request to the API (`body.restApiId` or `body.graphqlApiId`) as
 * deployed on the gateway (`body.gatewayId`). Any answer from the gateway, whatever
 * its status, resolves: it is carried in `response`. A rejection means the request
 * was not relayed.
 */
export const invokeApiOnGateway = async (
  body: InvokeApiOnGatewayBody,
  options?: RequestOptions,
): Promise<InvokeApiOnGatewayResponse> => {
  return http.post<InvokeApiOnGatewayResponse>('/gateway-relay/invoke', body, {
    ...options,
    operationName: 'InvokeAPIOnGateway',
  });
};
