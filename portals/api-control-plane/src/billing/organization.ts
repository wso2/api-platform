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

import { runtimeConfig } from '../config/runtime';

// Product code this console activates on first login. Must match the billing
// product whose subscription drives APIP gateway provisioning.
const PRODUCT = 'api-platform';

/**
 * The same-origin path billing is reached through: `<proxy base>/billing`, the
 * way the BFF names its upstreams (`{proxy_prefix}/{name}`) and a gateway in
 * front of the console routes them. The proxy base is `platformApiBaseUrl` when
 * that is a same-origin path (`/proxy` behind the BFF, `/api-management/proxy`
 * behind a gateway). Anything else, an absolute URL straight to a Platform API
 * or no value at all, has no billing route of its own, so billing stays on the
 * BFF's default `/proxy`.
 */
const billingBase = (): string => {
  const base = runtimeConfig.platformApiBaseUrl.replace(/\/+$/, '');
  const sameOrigin = base.startsWith('/') && !base.startsWith('//');
  return `${sameOrigin ? base : '/proxy'}/billing`;
};

/** The organization's billing record, as far as anything in this console reads it. */
export type BillingOrganization = {
  subscription?: {
    status?: string;
    trial?: {
      days_remaining: number;
      trial_end: string;
      /**
       * Full length of the trial in days, as the product is configured. Read
       * rather than assumed: the badge's progress ring is `elapsed / trial_days`,
       * and a hardcoded constant here pinned the ring at a full circle for the
       * first half of every trial.
       */
      trial_days?: number;
    } | null;
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
 * login as a side effect. Resolves `null` when the deployment has no billing
 * route (`billingProxyEnabled` off, every standalone deployment today), so a
 * caller needs no config of its own.
 *
 * A failed read clears the memo, so a later caller retries rather than being
 * stuck with the rejection for the rest of the session.
 */
export function getBillingOrganization(): Promise<BillingOrganization | null> {
  if (!runtimeConfig.billingProxyEnabled) return Promise.resolve(null);
  pending ??= fetch(`${billingBase()}/organization?product=${PRODUCT}`, {
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
  })
    .then((response) => {
      if (!response.ok) throw new Error(`Unable to read billing organization (${response.status})`);
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
