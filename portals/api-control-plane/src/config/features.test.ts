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

import { afterEach, describe, expect, it, vi } from 'vitest';

const load = async (disableFeatures?: string[]) => {
  vi.resetModules();
  window.__RUNTIME_CONFIG__ = disableFeatures ? { disableFeatures } : {};
  return import('./features');
};

afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
  vi.resetModules();
});

describe('isFeatureDisabled', () => {
  it('leaves every feature on when nothing is disabled', async () => {
    const { FEATURES, isFeatureDisabled } = await load();

    expect(FEATURES.some(isFeatureDisabled)).toBe(false);
  });

  it('disables only the features listed', async () => {
    const { isFeatureDisabled } = await load(['metrics']);

    expect(isFeatureDisabled('metrics')).toBe(true);
    expect(isFeatureDisabled('logs')).toBe(false);
  });
});
