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

/*
 * Helper to exchange Asgardeo token for a Choreo STS token.
 */

import { Hooks } from '@asgardeo/auth-react';
import { tokenExchangeConfig, TOKEN_EXCHANGE_CONFIG_ID } from '../config.env';
import { logger } from '../utils/logger';

// Simplified types to avoid tight coupling with Asgardeo SDK types
type OnFn = (hook: Hooks, callback: (response?: any) => void, id?: string) => void;
type UpdateConfigFn = (cfg: Record<string, unknown>) => Promise<void>;
type RequestCustomGrantFn = (config: any) => any;

/**
 * Exchange an Asgardeo token for a Choreo STS token for a given org.
 *
 * @param on - `on` function from `useAuthContext` to listen for Hooks.CustomGrant
 * @param updateConfig - `updateConfig` function from `useAuthContext` to tweak SDK config
 * @param requestCustomGrant - `requestCustomGrant` function from `useAuthContext` to perform custom grant
 * @param orgHandle - Organization handle to include in the token exchange
 * @param onComplete - Optional callback invoked with the grant response when exchange completes
 */
export const exchangeToken = async (
  on: OnFn,
  updateConfig: UpdateConfigFn,
  requestCustomGrant: RequestCustomGrantFn,
  orgHandle: string,
  onComplete?: (response?: any) => void
): Promise<void> => {
  logger.log('Exchanging token for org:', orgHandle);

  // Disable ID token validation for STS token
  await updateConfig({
    validateIDToken: false,
  });

  return new Promise<void>((resolve) => {
    // Register callback for when token exchange completes
    on(
      Hooks.CustomGrant,
      (response?: { accessToken?: string }) => {
        logger.log('Token exchange completed successfully');
        onComplete?.(response);
        resolve();
      },
      TOKEN_EXCHANGE_CONFIG_ID
    );

    // Perform the token exchange
    requestCustomGrant({
      ...tokenExchangeConfig,
      data: {
        ...tokenExchangeConfig.data,
        orgHandle,
      },
    } as Parameters<RequestCustomGrantFn>[0]);
  });
};
