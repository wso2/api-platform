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

import { useEffect, useState } from 'react';

/**
 * `value` once it has stopped changing for `delayMs`.
 *
 * The first value passes through with no delay, so a page fetches on mount
 * rather than after a pause.
 *
 * `isEqual` decides what "changed" means. It defaults to identity, which is only
 * safe when the caller holds the value in state: a fresh object built on every
 * render would restart the timer faster than it can elapse and the hook would
 * never settle again. Pass a content comparison for anything else — it also
 * stops a round trip back to the current value (set a filter, undo it) from
 * counting as a change.
 */
export function useDebounced<T>(
  value: T,
  delayMs: number,
  isEqual: (a: T, b: T) => boolean = Object.is
): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    if (isEqual(settled, value)) return undefined;
    const timer = window.setTimeout(() => setSettled(value), delayMs);
    return () => window.clearTimeout(timer);
  }, [delayMs, isEqual, settled, value]);

  return settled;
}
