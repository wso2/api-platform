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
 * Asserting which feature-usage actions a component reports, without the
 * Moesif SDK. A test file opts in with one line, which Vitest hoists:
 *
 *   vi.mock('@/featureUsage/moesif');
 *
 * That replaces `trackFeatureUsage` with a mock, so `useTrackFeature()` calls
 * land here instead of being dropped (tracking is off in tests). Then:
 *
 *   beforeEach(() => resetTrackedFeatures());
 *   expect(trackedFeatures()).toEqual([['apip.manage-apis.delete-api', { api_type: 'rest-api' }]]);
 *
 * The SDK path itself is covered by src/featureUsage/*.test.ts.
 */

import { vi } from 'vitest';

import { trackFeatureUsage } from '@/featureUsage/moesif';

const mocked = () => {
  const fn = vi.mocked(trackFeatureUsage);
  if (!vi.isMockFunction(fn)) {
    throw new Error(
      "trackedFeatures() needs vi.mock('@/featureUsage/moesif') at the top of the test file.",
    );
  }
  return fn;
};

/** The actions reported so far, as [code, dimensions] pairs, in order. */
export const trackedFeatures = (): Array<[string, Record<string, unknown>]> =>
  mocked().mock.calls.map(([code, dimensions]) => [code, { ...(dimensions ?? {}) }]);

/** The codes reported so far, in order. */
export const trackedCodes = (): string[] => trackedFeatures().map(([code]) => code);

/** Forgets the actions reported so far; call it in `beforeEach`. */
export const resetTrackedFeatures = (): void => {
  mocked().mockClear();
};
