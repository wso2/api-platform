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

import { fetchEntitlementSummary } from './entitlementsApi';
import type { EntitlementSummary } from './entitlementTypes';

/**
 * One organization's entitlement summary, fetched once per generation and
 * shared.
 *
 * Two consumers need the same answer — the limits gate and the badge's usage
 * panel — and they mount separately, so without this they would each issue the
 * same request on every page load.
 *
 * The entry is keyed on a `generation` as well as the organization. That is
 * what makes a re-read safe between two consumers that live in different parts
 * of the tree: the host bumps one counter, both ask for that generation, and
 * whichever renders first performs the single fetch the other then shares.
 * Keying it this way deliberately replaces an earlier arrangement where the
 * gate called `resetEntitlementSummary()` and both sides then re-read — that
 * depended on the gate's effect running before the badge's, which the tree
 * order does not guarantee (the header mounts first), so the badge could
 * re-read the stale entry and keep showing it.
 *
 * Holds exactly one entry, replaced whenever a different organization or
 * generation is asked for, so it cannot grow over a session.
 */

type Entry = {
  organization: string;
  generation: number;
  promise: Promise<EntitlementSummary | null>;
};

let entry: Entry | null = null;

/**
 * The organization's summary, resolving `null` on any failure. A failure is not
 * held onto: the entry is dropped so the next caller retries rather than being
 * stuck with it for the rest of the session.
 */
export function getEntitlementSummary(
  organization: string,
  generation = 0
): Promise<EntitlementSummary | null> {
  if (entry && entry.organization === organization && entry.generation === generation) {
    return entry.promise;
  }

  const promise = fetchEntitlementSummary().then((summary) => {
    if (
      summary === null &&
      entry?.organization === organization &&
      entry.generation === generation
    ) {
      entry = null;
    }
    return summary;
  });

  entry = { organization, generation, promise };
  return promise;
}

/** Forget what is held, so the next read re-fetches. */
export function resetEntitlementSummary(): void {
  entry = null;
}
