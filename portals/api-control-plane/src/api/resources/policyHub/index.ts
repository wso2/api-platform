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

/**
 * Public surface of the Policy Hub resource module.
 * Import from `api/resources/policyHub` only; deeper imports skip the
 * `enabled` gating that makes an unconfigured hub a no-op.
 * @see ./policyHub.hooks.ts for the hook contract.
 * @see ./policyHub.endpoints.ts for why this resource is not spec-derived.
 */

// ─── Types ──────────────────────────────────────────────────────────────────

export type { PolicyDefinition, PolicyListResult, PolicySummary } from './policyHub.endpoints';

export type { ParameterSchema, ParameterValues } from './policySchema';

// ─── Parameter-schema helpers ───────────────────────────────────────────────
//
// Pure functions over a `ParameterSchema`, with no transport of their own. They
// live in this module because the schema they operate on is the Policy Hub's
// wire shape, and the config form is their only caller.

export {
  defaultForSchema,
  getByPath,
  initValues,
  setByPath,
  topLevelRequiredMissing,
} from './policySchema';

// ─── Hooks ──────────────────────────────────────────────────────────────────

export {
  useIsPolicyHubConfigured,
  usePolicyDefinition,
  usePolicyHubCategories,
  usePolicyHubPolicies,
  usePolicyVersions,
} from './policyHub.hooks';
