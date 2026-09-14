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

import { isPolicyHubConfigured } from './policyHub.endpoints';
import { policyHubQueries } from './policyHub.queries';

/**
 * The public hook surface for the Policy Hub catalog.
 *
 * The hub is optional: a deployment without `policyHubBaseUrl` has no catalog
 * and the Policies page says so. Rather than make every caller remember that,
 * `isPolicyHubConfigured()` is folded into each `enabled` gate — an unconfigured
 * hub yields a query that never fires and never errors, so the UI branches on
 * the config flag alone and never on a failed request.
 *
 * As everywhere else, `query.error` is always an `ApiError` with `.kind` and
 * `.code`; components never see a raw `fetch` rejection.
 */

/** Whether an operator has configured a Policy Hub for this deployment. */
export const useIsPolicyHubConfigured = (): boolean => isPolicyHubConfigured();

/**
 * One page of the policy catalog, optionally narrowed to categories.
 *
 * `keepPreviousData` for the same reason the REST API list uses it: paging
 * changes the key, and without it the catalog would unmount into a spinner on
 * every Next click.
 */
export const usePolicyHubPolicies = (
  page: number,
  pageSize: number,
  categories: string[] = [],
  enabled = true,
) =>
  useQuery({
    ...policyHubQueries.policies(page, pageSize, categories),
    enabled: enabled && isPolicyHubConfigured(),
    placeholderData: keepPreviousData,
  });

/** Category names, for the catalog's filter chips. */
export const usePolicyHubCategories = (enabled = true) =>
  useQuery({
    ...policyHubQueries.categories(),
    enabled: enabled && isPolicyHubConfigured(),
  });

/** Every published version of one policy. */
export const usePolicyVersions = (name: string | undefined, enabled = true) =>
  useQuery({
    ...policyHubQueries.versions(name!),
    enabled: enabled && isPolicyHubConfigured() && Boolean(name),
  });

/**
 * One policy version's definition — the parameter schema the config drawer
 * renders its form from. Gated on both parts of the identity, so the drawer can
 * mount before a policy has been picked.
 */
export const usePolicyDefinition = (
  name: string | undefined,
  version: string | undefined,
  enabled = true,
) =>
  useQuery({
    ...policyHubQueries.definition(name!, version!),
    enabled: enabled && isPolicyHubConfigured() && Boolean(name) && Boolean(version),
  });
