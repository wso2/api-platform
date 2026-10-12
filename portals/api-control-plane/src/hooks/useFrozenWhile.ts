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

import { useState } from 'react';

/**
 * `value`, except that while `frozen` is set it stays what it was when the
 * freeze began.
 *
 * For a screen whose data is refetched by its own actions: an action that saves
 * in several steps would otherwise redraw the screen at each step, and show
 * states (a draft that now exists, a listing that is now live) that only matter
 * once the action has finished. Held back, they appear once, when it ends.
 */
export function useFrozenWhile<T>(value: T, frozen: boolean): T {
  const [held, setHeld] = useState(value);
  // Followed on every render that isn't frozen, so the held value is always the
  // last one shown. Set during render, which React allows for derived state.
  if (!frozen && held !== value) setHeld(value);
  return frozen ? held : value;
}
