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

import { get, getText } from '../clients/publicApiClient';
import { API_BASE_URLS } from '../config.env';
import { GuardrailsResponse } from '../utils/types';

/**
 * Fetch guardrail policies from Policy Hub API
 *
 * @param categories - Comma-separated list of categories (default: 'Guardrails,AI')
 * @param limit - Max number of policies to return per page (default: 40)
 * @param offset - Number of policies to skip, for pagination (default: 0)
 * @returns Promise with the policies response
 */
export const getGuardrails = async (
  categories: string,
  limit = 60,
  offset = 0
): Promise<GuardrailsResponse> => {
  return get<GuardrailsResponse>(
    '/policies',
    { categories, limit, offset },
    API_BASE_URLS.policyHubApi
  );
};

/** Fetch all policies for the gateway policy catalogue. */
export const getPolicies = async (
  offset = 0,
  limit = 100,
): Promise<GuardrailsResponse> => {
  return get<GuardrailsResponse>(
    '/policies',
    { offset, limit },
    API_BASE_URLS.policyHubApi,
  );
};

/**
 * Fetch raw policy definition (YAML) from Policy Hub API
 */
export const getPolicyDefinition = async (
  name: string,
  version: string
): Promise<string> => {
  const encodedName = encodeURIComponent(name);
  const encodedVersion = encodeURIComponent(version);
  return getText(
    `/policies/${encodedName}/versions/${encodedVersion}/definition`,
    undefined,
    API_BASE_URLS.policyHubApi,
    { Accept: 'text/yaml' }
  );
};

// Export all policy hub API functions
const policyHubApis = {
  getGuardrails,
  getPolicies,
  getPolicyDefinition,
};

export default policyHubApis;
