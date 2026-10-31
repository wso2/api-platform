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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { PRODUCT } from './entitlementLimits';
import type { EntitlementSummary } from './entitlementTypes';

/**
 * Same-origin path the BFF proxies to billing's Entitlement API, mirroring how
 * this console already reaches `/proxy/billing/organization`. The BFF strips
 * the prefix and joins the rest onto the configured upstream base, so the real
 * billing URL never reaches the browser.
 */
const SUMMARY_PATH = `/proxy/billing-entitlement/summary?product=${PRODUCT}`;

/**
 * Read the organization's entitlement summary.
 *
 * The organization is resolved by billing from the bearer the BFF injects out
 * of the session cookie, so nothing identifying is sent from here and a caller
 * cannot ask about another organization.
 *
 * Resolves `null` on **any** failure — including the 404 a deployment with no
 * `billing-entitlement` upstream answers. Every consumer reads `null` as
 * "unknown", which means no limits and no banner: a failed read must never be
 * able to lock a user out of their own product.
 */
export async function fetchEntitlementSummary(): Promise<EntitlementSummary | null> {
  try {
    const response = await fetch(SUMMARY_PATH, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    });
    if (!response.ok) return null;
    return (await response.json()) as EntitlementSummary;
  } catch {
    return null;
  }
}
