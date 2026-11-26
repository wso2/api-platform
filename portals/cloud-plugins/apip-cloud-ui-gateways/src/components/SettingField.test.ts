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

import { isTomlBlock } from './SettingField';

describe('isTomlBlock', () => {
  it('is a toml string, or a string from a platform that predates format', () => {
    // Both are gateway.config_toml, which the drawer leaves out.
    expect(isTomlBlock({ path: 'a', type: 'string', format: 'toml' })).toBe(true);
    expect(isTomlBlock({ path: 'a', type: 'string' })).toBe(true);
  });

  it('is not a header value or any other type', () => {
    expect(isTomlBlock({ path: 'a', type: 'string', format: 'header-value' })).toBe(false);
    expect(isTomlBlock({ path: 'a', type: 'string', format: 'text' })).toBe(false);
    expect(isTomlBlock({ path: 'a', type: 'boolean' })).toBe(false);
  });
});
