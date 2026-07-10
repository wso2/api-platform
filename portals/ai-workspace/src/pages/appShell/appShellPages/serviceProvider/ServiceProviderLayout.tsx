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

import React from 'react';
import { Outlet } from 'react-router-dom';
import { LLMProvidersProvider } from '../../../../contexts/llmProvider';
import { ProviderTemplatesProvider } from '../../../../contexts/llmProvider/providerTemplate';
import { GuardrailsProvider } from '../../../../contexts/GuardrailsContext';

export { formatRelativeTime, buildProviderId } from '../../../../contexts/llmProvider';

export type ProviderStatus = 'Active' | 'Degraded' | 'Paused';

export type ProviderConfiguration = {
  source: string;
  identifier: string;
};

export type ServiceProvider = {
  id: string;
  name: string;
  models: number;
  lastUpdated: string;
  status: ProviderStatus;
  logoUrl?: string;
  version?: string;
  vhost?: string;
  providerType?: string;
  endpoint?: string;
  authentication?: string;
  credential?: string;
  configurations?: {
    requestModel?: ProviderConfiguration;
    responseModel?: ProviderConfiguration;
    promptTokens?: ProviderConfiguration;
    completionTokens?: ProviderConfiguration;
    totalTokens?: ProviderConfiguration;
    cost?: ProviderConfiguration;
  };
};

export default function ServiceProviderLayout() {
  return (
    <LLMProvidersProvider>
      <ProviderTemplatesProvider>
        <GuardrailsProvider>
          <Outlet />
        </GuardrailsProvider>
      </ProviderTemplatesProvider>
    </LLMProvidersProvider>
  );
}
