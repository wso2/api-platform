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

import { useEffect, useRef } from 'react';

import { isReadOnly, toResourceLimits } from './entitlementLimits';
import { getEntitlementSummary } from './entitlementStore';
import type { EntitlementsHostPort } from './hostPort';
import ReadOnlyNotice from './ReadOnlyNotice';
import { upgradeUrl } from './runtimeConfig';

/**
 * Shortest gap between two focus-triggered reads, so alt-tabbing repeatedly
 * cannot hammer billing. Long enough to be cheap, short enough that coming back
 * to a tab after doing something elsewhere still refreshes.
 */
const MIN_FOCUS_REFETCH_MS = 60_000;

/**
 * How often to re-read as a backstop while a tab simply stays open. This is the
 * one signal that changes genuinely out of band — an upgrade completed in the
 * billing console, a trial lapsing at midnight, a suspension.
 */
const BACKSTOP_INTERVAL_MS = 30 * 60_000;

/**
 * Waits before re-reading when a refresh returned exactly what we already had.
 *
 * A count does not move the instant the user acts on it. The create or delete
 * returns as soon as the control plane has committed it, but the number behind
 * a meter is updated by a chain that runs on its own: the resource
 * event reaches core, core publishes it, billing consumes it off NATS and
 * forwards a delta, and only then does the metering projection update the
 * counter. Measured locally, the console's re-read reached billing about 100ms
 * after billing had merely *forwarded* the delta — comfortably ahead of the
 * write it was trying to observe. One immediate read therefore returns the old
 * number and nothing asks again, which is why a count only looked right after
 * a full page reload.
 *
 * So read again, a few times, backing off — and stop as soon as the answer
 * changes, which is the common case on the first retry. The budget is small and
 * strictly bounded: a refresh whose value never changes (a navigation, say)
 * costs these few cheap reads and then stops for good.
 */
const UNCHANGED_RETRY_DELAYS_MS = [1_200, 3_000, 6_000];

export type EntitlementsGateProps = {
  port: EntitlementsHostPort;
};

/**
 * Supplies the organization's creation limits and read-only verdict to the host.
 *
 * Renders nothing. A headless `app.gate` entry, which the host mounts on every
 * in-shell route — the right shape because entitlements are a cross-cutting
 * policy the portal has no opinion about: it owns a generic "may we create
 * another one of these", and the commercial answer comes from here.
 *
 * All this does is relay. Billing resolves the plan, applies its ceilings and
 * counts what exists; the shared-budget arithmetic lives in `entitlementLimits`
 * so no metric code or plan concept crosses into the host.
 *
 * **Never locks anyone out.** A failed read resolves `null`, which clears the
 * limits rather than imposing any — a wrong "allowed" costs a failed submit,
 * while a wrong "denied" would deny a user their own product. The control plane
 * refuses an over-limit create regardless, so this is a courtesy, not the guard.
 */
export default function EntitlementsGate({ port }: EntitlementsGateProps) {
  const organization = port.orgHandle;
  const projectHandle = port.projectHandle;
  const setLimits = port.resourceLimits.set;
  const setReadOnly = port.resourceLimits.setReadOnly;
  const refreshSignal = port.resourceLimits.refreshSignal;
  const refresh = port.resourceLimits.refresh;

  // Survives the re-render a retry causes, so the budget is per episode rather
  // than per read: `seen` is the answer we are trying to see move, `attempt`
  // how much of the budget this episode has already spent.
  const retryRef = useRef<{ seen: string | null; attempt: number }>({
    seen: null,
    attempt: 0,
  });

  useEffect(() => {
    if (!organization) {
      setLimits(null);
      setReadOnly(false);
      return;
    }

    let cancelled = false;
    let retryTimer: number | undefined;

    void (async () => {
      const summary = await getEntitlementSummary(organization, refreshSignal);
      if (cancelled) return;

      // `null` (unknown) clears the limits rather than capping anything.
      setLimits(toResourceLimits(summary));

      // A separate verdict from the per-component ceilings: whether the
      // organization may write at all. The sentence, and the banner with its
      // link, both come from this side — the host has no wording for either.
      const readOnly = isReadOnly(summary);
      setReadOnly(
        readOnly,
        'Your organization is read-only because it has no active plan.',
        readOnly ? <ReadOnlyNotice upgradeHref={upgradeUrl(organization)} /> : undefined
      );

      // Did this read actually show us anything new? The first read of a
      // session has nothing to compare against and so never counts as stale.
      const answer = JSON.stringify(summary);
      const retry = retryRef.current;
      if (retry.seen === null || answer !== retry.seen) {
        retry.seen = answer;
        retry.attempt = 0;
        return;
      }
      if (retry.attempt >= UNCHANGED_RETRY_DELAYS_MS.length) return;

      // Ask through `refresh` rather than re-reading here, so the badge's
      // usage panel moves to the same new generation and sees the new value
      // too — it reads the same summary and would otherwise keep the old one.
      const delay = UNCHANGED_RETRY_DELAYS_MS[retry.attempt];
      retry.attempt += 1;
      retryTimer = window.setTimeout(refresh, delay);
    })();

    return () => {
      cancelled = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    };
  }, [organization, projectHandle, refresh, refreshSignal, setLimits, setReadOnly]);

  // A navigation is also a cheap signal that counts may be stale, so ask for a
  // re-read when the project changes. This bumps the same counter everything
  // else uses rather than clearing the store directly: the store is keyed on
  // the counter, so every consumer moves to the new generation together. An
  // earlier version cleared the entry here and relied on this effect running
  // before the badge's — which the tree order does not guarantee, and the
  // badge mounts first, so it could re-read and keep the stale value.
  const lastProjectRef = useRef(projectHandle);
  useEffect(() => {
    if (lastProjectRef.current === projectHandle) return;
    lastProjectRef.current = projectHandle;
    refresh();
  }, [projectHandle, refresh]);

  const lastFocusReadAt = useRef(0);

  useEffect(() => {
    if (!organization) return undefined;

    // Re-read on focus: an upgrade, trial lapse or suspension happens out of
    // band, and none of them originate here. Keep it.
    const onFocus = () => {
      const now = Date.now();
      if (now - lastFocusReadAt.current < MIN_FOCUS_REFETCH_MS) return;
      lastFocusReadAt.current = now;
      refresh();
    };

    const backstop = window.setInterval(() => {
      lastFocusReadAt.current = Date.now();
      refresh();
    }, BACKSTOP_INTERVAL_MS);

    window.addEventListener('focus', onFocus);
    return () => {
      window.removeEventListener('focus', onFocus);
      window.clearInterval(backstop);
    };
  }, [organization, refresh]);

  return null;
}
