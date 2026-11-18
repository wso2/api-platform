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

import { useMutation } from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { HANDLED_LOCALLY } from '../../core/queryClient';
import { useApiScope } from '../../core/scope';
import {
  invokeApiOnGateway,
  type InvokeApiOnGatewayBody,
  type InvokeApiOnGatewayResponse,
} from './gatewayRelay.endpoints';

export type InvokeApiOnGatewayVariables = {
  body: InvokeApiOnGatewayBody;
  signal?: AbortSignal;
};

/**
 * Sends one test request to a REST or GraphQL API as deployed on a gateway (the
 * Test consoles). The console shows any failure itself, so the global error
 * snackbar stays out of it, and a test request is never resent behind the user's back.
 */
export const useInvokeApiOnGateway = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);

  return useMutation<InvokeApiOnGatewayResponse, ApiError, InvokeApiOnGatewayVariables>({
    mutationFn: ({ body, signal }) => invokeApiOnGateway(body, { orgId, signal }),
    meta: HANDLED_LOCALLY,
    retry: false,
  });
};
