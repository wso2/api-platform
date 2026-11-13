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
 * Sends feature-usage actions to Moesif, batched, through the console's own
 * origin.
 *
 * The browser never talks to Moesif and holds no Moesif Application ID. It
 * posts each batch to `featureUsageCollectorPath/v1/actions/batch`, a gateway
 * session route: the gateway swaps the session cookie for the user's token and
 * forwards to the in-cluster moesif-collector, which adds the environment's
 * Application ID.
 *
 * Off unless the deployment sets `featureUsageTrackingEnabled` and a
 * `featureUsageCollectorPath`; on-prem leaves both unset and nothing is sent.
 *
 * Actions are queued and sent every 5 seconds or 20 actions, and when the page
 * is hidden or closed. Every request uses `keepalive`, so a batch sent as the
 * page goes away still reaches the collector. A batch that fails is dropped:
 * nothing is retried.
 *
 * Every failure is swallowed. Tracking must never break the interaction it
 * measures.
 */

import { runtimeConfig } from '@/config/runtime';
import { buildAction, type FeatureUsageIdentity, type MoesifAction } from './actions';
import type { ApiPlatformFeatureCode, FeatureDimensions } from './taxonomy';

export type { FeatureUsageIdentity } from './actions';

/** Actions per request. */
export const BATCH_SIZE = 20;
/** The longest an action waits in the queue before it is sent. */
export const BATCH_MAX_WAIT_MS = 5000;
/** The Collection API route, below the collector path. */
const ACTIONS_BATCH_ROUTE = '/v1/actions/batch';
/** Where this tab's session ID is kept, so it survives a reload of the tab. */
const SESSION_STORAGE_KEY = 'apip.featureUsage.session';

export function isFeatureUsageTrackingEnabled(): boolean {
  return (
    runtimeConfig.featureUsageTrackingEnabled && runtimeConfig.featureUsageCollectorPath !== ''
  );
}

const queue: MoesifAction[] = [];
let timer: ReturnType<typeof setTimeout> | undefined;
let listening = false;
let sessionToken: string | undefined;

/** A UUID for this tab's session, from sessionStorage when it is available. */
function getSessionToken(): string | undefined {
  if (sessionToken) return sessionToken;
  try {
    sessionToken = sessionStorage.getItem(SESSION_STORAGE_KEY) ?? undefined;
  } catch {
    // Without storage the session lasts for this page load.
  }
  if (!sessionToken) {
    try {
      sessionToken = crypto.randomUUID();
      sessionStorage.setItem(SESSION_STORAGE_KEY, sessionToken);
    } catch {
      // Either no randomUUID (an insecure context) or no storage; the ID, if
      // any, still serves this page load.
    }
  }
  return sessionToken;
}

function send(batch: MoesifAction[]): void {
  try {
    void fetch(`${runtimeConfig.featureUsageCollectorPath}${ACTIONS_BATCH_ROUTE}`, {
      body: JSON.stringify(batch),
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      keepalive: true,
      method: 'POST',
    }).catch(() => undefined);
  } catch {
    // Dropped: a broken send must not surface anywhere.
  }
}

/** Sends everything queued, in batches of at most BATCH_SIZE. */
export function flushFeatureUsage(): void {
  if (timer !== undefined) {
    clearTimeout(timer);
    timer = undefined;
  }
  while (queue.length > 0) {
    send(queue.splice(0, BATCH_SIZE));
  }
}

/** Sends what is queued when the page is hidden or closed, once per page load. */
function listenForPageExit(): void {
  if (listening) return;
  listening = true;
  window.addEventListener('pagehide', flushFeatureUsage);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushFeatureUsage();
  });
}

/**
 * Queues one feature-usage action, attributed to `identity`. Returns at once;
 * the action is sent with the next batch.
 */
export function trackFeatureUsage<C extends ApiPlatformFeatureCode>(
  code: C,
  dimensions: FeatureDimensions<C> | undefined,
  identity: FeatureUsageIdentity,
): void {
  if (!isFeatureUsageTrackingEnabled()) return;
  try {
    queue.push(
      buildAction(code, dimensions, identity, {
        basePath: runtimeConfig.appBasePath,
        pageUrl: window.location.href,
        sessionToken: getSessionToken(),
        time: new Date(),
        userAgent: navigator.userAgent,
      }),
    );
    listenForPageExit();
    if (queue.length >= BATCH_SIZE) {
      flushFeatureUsage();
    } else if (timer === undefined) {
      timer = setTimeout(flushFeatureUsage, BATCH_MAX_WAIT_MS);
    }
  } catch {
    // Dropped: a broken tracking call must not surface anywhere.
  }
}
