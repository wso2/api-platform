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
