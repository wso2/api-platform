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

const TRIAL_URL = '/proxy/billing/organization?product=api-platform';

export async function getBillingOrganization(
  signal?: AbortSignal
): Promise<BillingOrganization> {
  const response = await fetch(TRIAL_URL, {
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
    signal,
  });

  if (!response.ok) {
    throw new Error(`Unable to load trial status (${response.status})`);
  }

  return response.json() as Promise<BillingOrganization>;
}
