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
 * Publishes feature-usage actions to Moesif with moesif-browser-js, the way the
 * Bijira console does: the SDK is initialised with the Moesif application's
 * publishable key, identifies the user and organization, and sends one action
 * per tracked step.
 *
 * Off unless the deployment sets `featureUsageTrackingEnabled` and a
 * `moesifPublishableKey`; on-prem leaves both unset. When off, the SDK is not
 * even downloaded: it is imported on first use only.
 *
 * Every request the SDK sends goes through the PII filter (./piiFilter.ts)
 * first. It masks the page URL to its route template and drops anything that
 * is not an ID, code or enum. If the filter cannot be installed, nothing is
 * sent at all.
 *
 * Every failure is swallowed. Tracking must never break the interaction it
 * measures.
 */

import type { MoesifClient, MoesifMetadata } from 'moesif-browser-js';

import { runtimeConfig } from '@/config/runtime';
import { filterMoesifRequest } from './piiFilter';
import { featureMetadata, type ApiPlatformFeatureCode, type FeatureDimensions } from './taxonomy';

/** Who an action is attributed to. Only UUIDs are sent; anything else is left out. */
export type FeatureUsageIdentity = {
  /** The IdP user ID (`sub`). */
  userId?: string;
  /** The organization UUID. */
  companyId?: string;
};

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const asUuid = (value: string | undefined) =>
  value !== undefined && UUID_PATTERN.test(value) ? value : undefined;

/**
 * The identity last sent to Moesif from this browser, so it is sent once per
 * user and organization rather than on every page load. The SDK keeps its own
 * copy of the IDs in localStorage and reattaches them to every action.
 */
const IDENTITY_STORAGE_KEY = 'apip.featureUsage.identity';

export function isFeatureUsageTrackingEnabled(): boolean {
  return (
    runtimeConfig.featureUsageTrackingEnabled && runtimeConfig.moesifPublishableKey.trim() !== ''
  );
}

let clientPromise: Promise<MoesifClient | null> | undefined;
/** Mirrors IDENTITY_STORAGE_KEY for this page load, so a repeat track skips the storage read. */
let identifiedAs: string | undefined;

/** The SDK's single network write, which every request funnels through (v1.10.2). */
type SdkTransport = (
  this: unknown,
  url: string,
  data: unknown,
  options: unknown,
  callback?: (response?: unknown) => void,
) => unknown;

/**
 * Puts the PII filter in front of the SDK's network write. Every send path ends
 * there: track() and identify, direct or batched, and the batch flushed when
 * the page is hidden. It is an internal SDK method, so the version is pinned
 * and a contract test (moesif.sdk.test.ts) checks what reaches the network.
 * Returns false when the method is missing, so tracking stays off rather than
 * sending unfiltered data.
 */
export function installPiiFilter(client: MoesifClient): boolean {
  const sdk = client as MoesifClient & { _executeRequest?: SdkTransport };
  const send = sdk._executeRequest;
  if (typeof send !== 'function') return false;
  sdk._executeRequest = function (url, data, options, callback) {
    let safe: unknown = null;
    try {
      safe = filterMoesifRequest(url, data, runtimeConfig.appBasePath);
    } catch {
      // A filter that fails sends nothing.
    }
    if (safe === null) {
      // Nothing safe to send. Report success, so a batch is cleared from the
      // SDK's queue instead of being retried forever.
      callback?.();
      return undefined;
    }
    return send.call(this, url, safe, options, callback);
  };
  return true;
}

function getClient(): Promise<MoesifClient | null> {
  if (clientPromise === undefined) {
    clientPromise = import('moesif-browser-js')
      .then(({ default: moesif }) => {
        // Before init(), so the filter sees the very first request too.
        if (!installPiiFilter(moesif)) return null;
        return moesif.init({
          applicationId: runtimeConfig.moesifPublishableKey.trim(),
          batchEnabled: true,
          batchSize: 20,
          batchMaxTime: 5000,
          // start() is never called, so the SDK captures none of the
          // console's own API calls. Should anything start it, drop them all,
          // as Bijira does: only identify and track data belongs in Moesif.
          // A `function` expression, not an arrow: the SDK checks options with
          // /^\s*\bfunction\b/ against the function's source and throws on
          // anything else, which would leave tracking silently off.
          skip: function skip() {
            return true;
          },
          // Referrer and campaign parameters are not part of the taxonomy.
          disableGclid: true,
          disableReferrer: true,
          disableUtm: true,
        });
      })
      .catch(() => null);
  }
  return clientPromise;
}

function readStoredIdentity(): string | null {
  try {
    return localStorage.getItem(IDENTITY_STORAGE_KEY);
  } catch {
    return null;
  }
}

function storeIdentity(key: string): void {
  try {
    localStorage.setItem(IDENTITY_STORAGE_KEY, key);
  } catch {
    // Without storage the identity is simply sent again on the next page load.
  }
}

/**
 * Identifies the user and organization, once per change. IDs only: no name,
 * email or organization handle, since Moesif is a third party (the WSO2 Moesif
 * data rules). A different identity than the one last sent from this browser
 * means another user or organization signed in, so the SDK's stored state is
 * reset first and the two are never merged.
 */
function identify(client: MoesifClient, identity: FeatureUsageIdentity): void {
  const userId = asUuid(identity.userId);
  const companyId = asUuid(identity.companyId);
  if (!userId && !companyId) return;

  const key = `${userId ?? ''}|${companyId ?? ''}`;
  if (identifiedAs === key) return;
  const previous = readStoredIdentity();
  if (previous !== key) {
    if (previous) client.reset();
    // Company first, so the user record is created with its company_id.
    if (companyId) client.identifyCompany(companyId);
    if (userId) client.identifyUser(userId);
    storeIdentity(key);
  }
  identifiedAs = key;
}

/**
 * Sends one feature-usage action, attributed to `identity`. Returns at once;
 * the SDK batches and sends in the background.
 */
export function trackFeatureUsage<C extends ApiPlatformFeatureCode>(
  code: C,
  dimensions: FeatureDimensions<C>,
  identity: FeatureUsageIdentity,
): void {
  if (!isFeatureUsageTrackingEnabled()) return;
  let metadata: MoesifMetadata;
  try {
    metadata = featureMetadata(code, dimensions);
  } catch {
    return;
  }
  void getClient().then((client) => {
    if (!client) return;
    try {
      identify(client, identity);
      client.track(code, metadata);
    } catch {
      // Dropped: a broken tracking call must not surface anywhere.
    }
  });
}
