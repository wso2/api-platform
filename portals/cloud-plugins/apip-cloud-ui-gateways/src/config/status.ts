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

import type { ConfigStatus } from '../types';
import { absoluteTime, relativeTime } from '../utils/time';

/**
 * The reconcile phase turned into the line of text at the right of the drawer's
 * header. The phase itself is the chip beside the gateway name, so `healthy`
 * spends this line on *when the configuration last landed*, and only a phase
 * that is still moving or has gone wrong spends it on a word.
 */

/** `error` is for a phase the reader has to act on; everything else is quiet. */
export type StatusTone = 'muted' | 'error';

export type StatusDisplay = {
  text: string;
  tone: StatusTone;
  /**
   * Hover detail — the absolute timestamp, or the platform's own message. The
   * message is kept OUT of the line itself: it is prose of unbounded length
   * (`Resource "apigateway" readyWhen returned false`) and it pushed the form
   * down the drawer. A tooltip costs no layout, so the detail is available
   * without the line growing.
   */
  detail?: string;
};

/**
 * `null` for "show nothing", which is a real outcome: a healthy gateway whose
 * transition time the platform has not recorded has nothing to say, and an
 * empty line beats "Updated —".
 *
 * `now` is a parameter for the tests; leave it unset in the component.
 */
export function describeStatus(
  status: ConfigStatus,
  now: number = Date.now()
): StatusDisplay | null {
  switch (status.phase) {
    case 'applying':
      // Not a failure and not an unfinished save. The write is committed; the
      // data plane is catching up, and the measured lag is minutes (10m07s on
      // 2026-08-31, with the gateway already ready underneath). The trailing
      // ellipsis is the whole signal that something is still in motion — a
      // spinner here would read as "the save has not finished", which is wrong.
      return { text: 'Applying…', tone: 'muted', detail: status.message };
    case 'healthy': {
      const when = status.lastTransitionTime;
      if (!when || Number.isNaN(Date.parse(when))) return null;
      return {
        text: `Updated ${relativeTime(when, now)}`,
        tone: 'muted',
        detail: absoluteTime(when),
      };
    }
    case 'failed':
      // The platform could not render or apply the spec, so nothing downstream
      // happens without another write. This is the one phase worth colouring.
      return { text: 'Failed', tone: 'error', detail: status.message };
    case 'unknown':
      // Core answered without a phase. Say so plainly rather than borrowing
      // `applying`'s copy, which would promise a settling that is not coming.
      return { text: 'Status unavailable', tone: 'muted', detail: status.message };
    default:
      // A phase this build has never heard of is a newer platform, not a
      // broken one. Say the word it sent; do not round it to healthy.
      return { text: status.phase, tone: 'muted', detail: status.message };
  }
}

/**
 * Whether a write should be held back.
 *
 * `applying` means the previous change has not reached the data plane yet, and a
 * second write on top of it re-renders the release from a document the gateway
 * has not finished picking up — so the phase gates the Save button.
 */
export const isApplying = (status: ConfigStatus | undefined): boolean =>
  status?.phase === 'applying';
