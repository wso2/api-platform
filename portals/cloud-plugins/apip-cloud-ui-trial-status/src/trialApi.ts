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


import type { BillingOrganization } from './types';

/**
 * The BFF's same-origin billing route, resolved against the portal's Vite `base`.
 *
 * The path cannot be a fixed "/proxy/..." string: each portal mounts the BFF under its
 * own base, so the console serves it at "/proxy/billing" while the AI Workspace serves
 * it at "/ai-workspace/proxy/billing". A root-relative path is correct only in the
 * console and 404s everywhere else.
 */
export function billingOrganizationUrl(viteBase = import.meta.env.BASE_URL): string {
  const base = String(viteBase ?? '/').replace(/\/$/, '');
  return `${base}/proxy/billing/organization?product=api-platform`;
}

export async function getBillingOrganization(
  signal?: AbortSignal
): Promise<BillingOrganization> {
  const response = await fetch(billingOrganizationUrl(), {
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
    signal,
  });

  if (!response.ok) {
    throw new Error(`Unable to load trial status (${response.status})`);
  }

  return response.json() as Promise<BillingOrganization>;
}
