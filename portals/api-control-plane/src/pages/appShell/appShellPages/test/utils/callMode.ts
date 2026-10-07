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

import { useCallback, useState } from 'react';

/**
 * Controls how the Console sends try-out requests.
 *
 * proxy routes through the BFF, avoiding browser CORS, mixed-content, and
 * cluster-internal routing issues.
 * direct uses the browser's global fetch to call the gateway directly,
 * supporting self-hosted gateways reachable by the user's browser but not the
 * BFF. This is an explicit user choice; the gateway must allow CORS.
 */
export type TestCallMode = 'proxy' | 'direct';

/** The relay, matching the behaviour that shipped before the switch existed. */
export const DEFAULT_TEST_CALL_MODE: TestCallMode = 'proxy';

/**
 * One key for the whole portal rather than one per API or gateway.
 *
 * The choice tracks the user's network position — which gateways their browser
 * can reach — not the API in front of them, so scoping it per API would make
 * someone re-pick `direct` on every API they open from the same desk.
 */
const STORAGE_KEY = 'apicp.test.callMode';

const isCallMode = (value: unknown): value is TestCallMode =>
  value === 'proxy' || value === 'direct';

/** Reads the stored preference, treating anything unrecognised as the default. */
export function readStoredCallMode(): TestCallMode {
  if (typeof window === 'undefined') return DEFAULT_TEST_CALL_MODE;
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isCallMode(stored) ? stored : DEFAULT_TEST_CALL_MODE;
  } catch {
    return DEFAULT_TEST_CALL_MODE;
  }
}

export function writeStoredCallMode(mode: TestCallMode): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Ignore persistence failures: the switch still applies for this visit.
  }
}

/**
 * The selected mode, restored on mount and persisted on every change.
 *
 * Read once into state rather than on each render, so a write from another tab
 * cannot change the transport mid-session under the user.
 */
export function useTestCallMode(): [TestCallMode, (mode: TestCallMode) => void] {
  const [callMode, setCallMode] = useState<TestCallMode>(readStoredCallMode);

  const select = useCallback((mode: TestCallMode) => {
    setCallMode(mode);
    writeStoredCallMode(mode);
  }, []);

  return [callMode, select];
}
