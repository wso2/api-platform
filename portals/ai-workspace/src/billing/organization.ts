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

import { BILLING_PROXY_ENABLED } from '../config.env';
import { BILLING_API_BASE_URL } from '../paths';

// Product code this workspace activates on first login. Must match the billing
// product whose subscription drives APIP gateway provisioning.
const PRODUCT = 'api-platform';

/** The organization's billing record, as far as anything in this workspace reads it. */
export type BillingOrganization = {
  subscription?: {
    status?: string;
    trial?: { days_remaining: number; trial_end: string } | null;
  } | null;
};

/**
 * The one in-flight-or-resolved read of the organization's billing record.
 *
 * This GET is not a plain read: it activates the api-platform subscription as a
 * server-side side effect when that subscription is still inactive, and that
 * activation is what emits subscription.activated and provisions the
 * organization's gateway. So it must have exactly one owner per session —
 * two unsynchronised callers race the activation, which is how a duplicate or
 * missed provisioning trigger happens.
 *
 * Everything that wants the billing record goes through here: the activation
 * hook, and (via the Port's `billing`) any cloud extension such as the trial
 * badge. Concurrent callers share this promise, so they cost one request.
 */
let pending: Promise<BillingOrganization | null> | null = null;

/**
 * Reads the organization's billing record, activating the subscription on first
 * login as a side effect. Resolves `null` when the BFF has no billing upstream
 * (every standalone deployment today), so a caller needs no config of its own.
 *
 * A failed read clears the memo, so a later caller — and the activation hook's
 * own retry — re-reads rather than being stuck with the rejection.
 */
export function getBillingOrganization(): Promise<BillingOrganization | null> {
  if (!BILLING_PROXY_ENABLED) return Promise.resolve(null);
  pending ??= fetch(`${BILLING_API_BASE_URL}/organization?product=${PRODUCT}`, {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
    .then((response) => {
      if (!response.ok) {
        throw new Error(`Unable to read billing organization (${response.status})`);
      }
      return response.json() as Promise<BillingOrganization>;
    })
    .catch((error: unknown) => {
      pending = null;
      throw error;
    });
  return pending;
}

/** Drops the memo so the next call re-reads. For tests. */
export function resetBillingOrganization(): void {
  pending = null;
}
