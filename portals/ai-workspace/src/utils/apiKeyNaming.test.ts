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
import { buildApiKeyResourceName } from './apiKeyNaming';

describe('buildApiKeyResourceName', () => {
  it('narrows a name a person would write to the identifier the API accepts', () => {
    expect(buildApiKeyResourceName('Production Key')).toBe('production-key');
    expect(buildApiKeyResourceName('  Staging / EU  ')).toBe('staging-eu');
    expect(buildApiKeyResourceName('key_2026')).toBe('key-2026');
  });

  it('never yields an empty identifier', () => {
    // A name that narrows to nothing still needs one, or the request is
    // rejected for a field the user cannot see.
    expect(buildApiKeyResourceName('!!!')).toBe('api-key');
    expect(buildApiKeyResourceName('   ')).toBe('api-key');
  });
});
