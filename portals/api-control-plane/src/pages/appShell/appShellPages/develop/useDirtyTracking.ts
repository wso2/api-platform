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

import { useRef } from 'react';

/**
 * Tracks whether `snapshot` (the panel's saveable state — never UI-only state
 * like search text or which drawer is open) has drifted from the last saved
 * baseline, so a `SaveBar` can gate its button on an actual unsaved change
 * rather than on form validity alone.
 *
 * The baseline is captured once on mount and again whenever the caller invokes
 * `markSaved()` — call it from the save mutation's `onSuccess`, using the
 * snapshot value at that point, so the bar goes quiet immediately rather than
 * waiting for the refetched `api` to flow back into local state.
 */
export function useDirtyTracking<T>(snapshot: T) {
  const baseline = useRef(JSON.stringify(snapshot));
  const dirty = JSON.stringify(snapshot) !== baseline.current;
  const markSaved = () => {
    baseline.current = JSON.stringify(snapshot);
  };
  return { dirty, markSaved };
}
