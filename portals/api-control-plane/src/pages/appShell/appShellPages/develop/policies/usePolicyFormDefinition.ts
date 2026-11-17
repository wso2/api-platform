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

import { useMemo } from 'react';

import {
  useAllGatewayCustomPolicies,
  type CustomPolicy,
} from '@/api/resources/gatewayCustomPolicies';
import {
  usePolicyDefinition,
  usePolicyVersions,
  type PolicySummary,
} from '@/api/resources/policyHub';
import { useCan } from '@/permissions';
import type { PolicyFormDefinition } from './parameterEditor';

/** Minimal reference needed to load a policy's definition. */
export type PolicyRef = Pick<PolicySummary, 'name' | 'version' | 'displayName'>;

export type PolicyFormDefinitionState = {
  /** Keeps its identity until the policy changes, since the editor keys its form on it. */
  definition?: PolicyFormDefinition;
  /** The full version the definition belongs to. */
  version?: string;
  isError: boolean;
};

/** The numeric major of "v1", "1" or "v1.2.3". */
const majorOf = (version: string): string | undefined => version.match(/^v?(\d+)/i)?.[1];

const isMajorOnly = (version: string): boolean => /^v?\d+$/i.test(version.trim());

/** Orders dotted versions numerically, so "1.10" outranks "1.9". */
const compareVersions = (a: string, b: string): number => {
  const parts = (v: string) =>
    v
      .replace(/^v/i, '')
      .split('.')
      .map((n) => Number.parseInt(n, 10) || 0);
  const [pa, pb] = [parts(a), parts(b)];
  for (let i = 0; i < Math.max(pa.length, pb.length); i += 1) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
};

const asSchema = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;

/** The organization's custom policy with this name and major version, if it holds one. */
const findCustomPolicy = (
  policies: readonly CustomPolicy[],
  ref: PolicyRef,
): CustomPolicy | undefined => {
  const name = ref.name.trim().toLowerCase();
  const major = majorOf(ref.version);
  return policies.find(
    (policy) => policy.name.toLowerCase() === name && majorOf(policy.version) === major,
  );
};

const fromCustomPolicy = (policy: CustomPolicy): PolicyFormDefinition => ({
  name: policy.name,
  version: policy.version,
  displayName: policy.displayName,
  description: policy.description,
  parameters: asSchema(policy.policyDefinition.parameters),
  systemParameters: asSchema(policy.policyDefinition.systemParameters),
});

/**
 * The definition a policy's parameter form is built from: the organization's
 * custom policy of that name and major version, or else the Policy Hub's.
 * A major-only reference ("v1") resolves to the newest hub version of that major.
 */
export function usePolicyFormDefinition(
  policy: PolicyRef | null,
  enabled: boolean,
): PolicyFormDefinitionState {
  const canReadCustomPolicies = useCan('ListGatewayCustomPolicies');
  const customQuery = useAllGatewayCustomPolicies({
    enabled: enabled && Boolean(policy) && canReadCustomPolicies,
  });
  const customSettled = !customQuery.isLoading;
  const customPolicy = useMemo(
    () =>
      policy && customQuery.data
        ? findCustomPolicy(customQuery.data.list ?? [], policy)
        : undefined,
    [policy, customQuery.data],
  );
  const customDefinition = useMemo(
    () => customPolicy && fromCustomPolicy(customPolicy),
    [customPolicy],
  );

  const fromHub = enabled && Boolean(policy) && customSettled && !customPolicy;
  const needsResolving = Boolean(policy && isMajorOnly(policy.version));
  const versionsQuery = usePolicyVersions(policy?.name, fromHub && needsResolving);
  const hubVersion = useMemo(() => {
    if (!policy) return undefined;
    if (!needsResolving) return policy.version;
    const major = majorOf(policy.version);
    return (versionsQuery.data ?? [])
      .filter((candidate) => majorOf(candidate.version) === major)
      .map((candidate) => candidate.version)
      .sort(compareVersions)
      .pop();
  }, [policy, needsResolving, versionsQuery.data]);
  const definitionQuery = usePolicyDefinition(policy?.name, hubVersion, fromHub);
  const hubDefinition = useMemo<PolicyFormDefinition | undefined>(() => {
    const data = definitionQuery.data;
    return (
      data && {
        name: data.name,
        version: data.version,
        displayName: data.displayName,
        description: data.description,
        parameters: data.parameters,
        systemParameters: data.systemParameters,
      }
    );
  }, [definitionQuery.data]);

  if (customPolicy) {
    return { definition: customDefinition, version: customPolicy.version, isError: false };
  }
  const versionUnresolved =
    needsResolving && (Boolean(versionsQuery.error) || (versionsQuery.isSuccess && !hubVersion));
  return {
    definition: hubDefinition,
    version: hubVersion,
    isError: Boolean(definitionQuery.error) || versionUnresolved,
  };
}
