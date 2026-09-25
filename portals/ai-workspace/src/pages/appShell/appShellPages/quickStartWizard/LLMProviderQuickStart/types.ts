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

import type { ProviderTemplate, LLMProvider } from '../../../../../utils/types';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import type {
  FormState,
  GuardrailSelection,
} from '../../serviceProvider/AddNewProvider/serviceProviderTypes';

export type ResourceType = 'provider' | 'mcp';

export type LLMProviderQuickStartStep =
  | 'select-template'
  | 'configure-provider'
  | 'test-provider';

export type GatewayFormState = {
  displayName: string;
  description: string;
  vhost: string;
  environment: string;
  version: string;
};

export type ProviderDraft = {
  selectedTemplateId: string | null;
  resolvedTemplate: ProviderTemplate | null;
  openapiSpec: string;
  formState: FormState;
  showCredential: boolean;
  guardrails: GuardrailSelection[];
  guardrailDrawerOpen: boolean;
  selectedGuardrail: string | null;
  guardrailSettings: Record<string, unknown>;
  createdProvider: LLMProvider | null;
};

export type GatewaySetupState = {
  gateway: HybridGateway | null;
  registrationToken: string | null;
};
