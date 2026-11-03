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

import { describe, expect, it } from 'vitest';

import { versionLabel } from './versionLabel';

describe('versionLabel', () => {
  it('returns empty string for missing input', () => {
    expect(versionLabel(undefined)).toBe('');
    expect(versionLabel('')).toBe('');
  });

  it('prepends "v" to a bare version', () => {
    expect(versionLabel('1.0')).toBe('v1.0');
    expect(versionLabel('2')).toBe('v2');
  });

  it('does not double up when the version already starts with "v" or "V"', () => {
    expect(versionLabel('v1.0')).toBe('v1.0');
    expect(versionLabel('V1.0')).toBe('V1.0');
  });
});
