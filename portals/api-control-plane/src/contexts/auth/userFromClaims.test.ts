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

import sharedCases from '../../../testdata/claim-mapping-cases.json';
import { parseClaimMappings } from '../../config/authContract';
import { hasScopeClaim, userFromClaims } from './userFromClaims';

type SharedCase = {
  name: string;
  mappings?: Record<string, string>;
  claims: Record<string, unknown>;
  expected: Record<string, unknown>;
};

// The same cases the BFF runs (bff/internal/session/claims_shared_test.go),
// so the console and the BFF resolve the same user from the same claims.
const cases = sharedCases.cases as SharedCase[];

describe('userFromClaims', () => {
  it('has shared cases to run', () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases.map((c) => [c.name, c] as const))('%s', (_name, testCase) => {
    const user = userFromClaims(testCase.claims, parseClaimMappings(testCase.mappings ?? {}));

    expect(user).toEqual(testCase.expected);
  });
});

describe('hasScopeClaim', () => {
  it('finds the claim at the configured path, even when empty', () => {
    expect(hasScopeClaim({ scope: 'openid' }, 'scope')).toBe(true);
    expect(hasScopeClaim({ scope: '' }, 'scope')).toBe(true);
    expect(hasScopeClaim({ authz: { scopes: [] } }, 'authz.scopes')).toBe(true);
  });

  it('falls back to the flat scp claim', () => {
    expect(hasScopeClaim({ scp: ['ap:api:read'] }, 'scope')).toBe(true);
  });

  it('is false when neither is there, as in an id_token', () => {
    expect(hasScopeClaim({ sub: '6c1f', name: 'Bob' }, 'scope')).toBe(false);
    expect(hasScopeClaim({ scope: null }, 'scope')).toBe(false);
  });
});
