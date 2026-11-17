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

import { customPolicySyncStatus, parsePolicyVersion } from './customPolicySync';

const reported = (version: string, name = 'hmac-signer') => ({ name, version });

describe('parsePolicyVersion', () => {
  it('reads MAJOR.MINOR.PATCH with or without a leading v', () => {
    expect(parsePolicyVersion('v1.2.3')).toEqual({ major: 1, minor: 2, patch: 3 });
    expect(parsePolicyVersion('1.10.0')).toEqual({ major: 1, minor: 10, patch: 0 });
  });

  it('rejects anything the server would not sync', () => {
    expect(parsePolicyVersion('v1')).toBeUndefined();
    expect(parsePolicyVersion('1.2')).toBeUndefined();
    expect(parsePolicyVersion('1.2.3-beta')).toBeUndefined();
  });
});

describe('customPolicySyncStatus', () => {
  it('is not synced when the organization holds no record of that major', () => {
    expect(customPolicySyncStatus(reported('v1.2.0'), [])).toEqual({ state: 'not-synced' });
    expect(customPolicySyncStatus(reported('v2.0.0'), [reported('v1.4.0')])).toEqual({
      state: 'not-synced',
    });
  });

  it('offers an update when the gateway has a higher minor of a held major', () => {
    expect(customPolicySyncStatus(reported('v1.4.0'), [reported('v1.2.0')])).toEqual({
      state: 'update-available',
      organizationVersion: 'v1.2.0',
    });
  });

  it('is synced for the same version, a patch-only difference, or a newer held minor', () => {
    expect(customPolicySyncStatus(reported('v1.2.0'), [reported('v1.2.0')])).toEqual({
      state: 'synced',
      organizationVersion: 'v1.2.0',
    });
    expect(customPolicySyncStatus(reported('v1.2.3'), [reported('v1.2.0')])).toEqual({
      state: 'synced',
      organizationVersion: 'v1.2.0',
    });
    expect(customPolicySyncStatus(reported('v1.2.0'), [reported('v1.4.0')])).toEqual({
      state: 'synced',
      organizationVersion: 'v1.4.0',
    });
  });

  it('compares minors numerically, not as text', () => {
    expect(customPolicySyncStatus(reported('v1.10.0'), [reported('v1.9.0')])).toEqual({
      state: 'update-available',
      organizationVersion: 'v1.9.0',
    });
  });

  it('matches names case-insensitively and ignores other policies', () => {
    expect(
      customPolicySyncStatus(reported('v1.2.0', 'HMAC-Signer'), [
        reported('v1.2.0', 'other-policy'),
        reported('v1.2.0', 'hmac-signer'),
      ]),
    ).toEqual({ state: 'synced', organizationVersion: 'v1.2.0' });
  });

  it('only counts an identical record for a version the server cannot parse', () => {
    expect(customPolicySyncStatus(reported('2026.11'), [reported('2026.11')])).toEqual({
      state: 'synced',
      organizationVersion: '2026.11',
    });
    expect(customPolicySyncStatus(reported('2026.11'), [reported('2026.10')])).toEqual({
      state: 'not-synced',
    });
  });
});
