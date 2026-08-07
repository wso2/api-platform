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

import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { useApiClient } from '../ApiClientProvider';
import { usePolicyHub as isPolicyHubConfigured } from './policyHubClient';

// Re-export the Policy Hub config predicate from the hook layer so components
// can read it without importing the client module directly (see the
// no-restricted-imports rule + src/api/README.md).
export { usePolicyHub } from './policyHubClient';

export const policyHubKeys = {
  policies: (page: number, pageSize: number, categories: string[]) =>
    ['policy-hub', 'policies', page, pageSize, categories] as const,
  categories: ['policy-hub', 'categories'] as const,
  definition: (name: string, version: string) =>
    ['policy-hub', 'definition', name, version] as const,
};

export const usePolicyHubPolicies = (
  page: number,
  pageSize: number,
  categories: string[],
  enabled = true
) => {
  const client = useApiClient();
  return useQuery({
    queryKey: policyHubKeys.policies(page, pageSize, categories),
    queryFn: () => client.listPolicies(page, pageSize, categories),
    enabled: enabled && isPolicyHubConfigured(),
    placeholderData: keepPreviousData,
    staleTime: 5 * 60 * 1000,
  });
};

export const usePolicyHubCategories = (enabled = true) => {
  const client = useApiClient();
  return useQuery({
    queryKey: policyHubKeys.categories,
    queryFn: client.listPolicyCategories,
    enabled: enabled && isPolicyHubConfigured(),
    staleTime: 10 * 60 * 1000,
  });
};

export const usePolicyDefinition = (
  name?: string,
  version?: string,
  enabled = true
) => {
  const client = useApiClient();
  return useQuery({
    queryKey: policyHubKeys.definition(name || '', version || ''),
    queryFn: () => client.getPolicyDefinition(name!, version!),
    enabled: enabled && isPolicyHubConfigured() && !!name && !!version,
    staleTime: 10 * 60 * 1000,
  });
};
