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

import type { PlanCondition } from '../../orgManage/components/planConditions';
import {
  attachedPolicyNames,
  CONDITION_POLICIES,
  planEnforcement,
  requiredPolicies,
  SUBSCRIPTION_VALIDATION,
} from './planPolicies';

const limit = (type: PlanCondition['type'], id: string): PlanCondition => ({
  id,
  params: { limits: [{ count: 10, timeAmount: 1, timeUnit: 'MINUTE' }], onExceed: 'reject' },
  type,
});
const custom = (id: string): PlanCondition => ({ id, params: { tier: 'gold' }, type: 'custom' });

describe('CONDITION_POLICIES', () => {
  it('maps request-count to subscription-validation and nothing else to a policy yet', () => {
    expect(CONDITION_POLICIES.builtIn).toEqual({
      'event-count': null,
      'request-count': SUBSCRIPTION_VALIDATION,
      'token-count': null,
      'tool-call-count': null,
    });
    expect(CONDITION_POLICIES.customById).toEqual({});
  });
});

describe('attachedPolicyNames', () => {
  it('collects global and per-operation policies, in either operation shape', () => {
    const names = attachedPolicyNames({
      operations: [
        { request: { policies: [{ name: 'subscription-validation', version: 'v1' }] } },
        { policies: [{ name: 'cors', version: 'v1' }] },
        {},
      ],
      policies: [{ name: 'jwt-auth', version: 'v1' }],
    });
    expect([...names].sort()).toEqual(['cors', 'jwt-auth', 'subscription-validation']);
  });

  it('is empty for an API with no policies', () => {
    expect(attachedPolicyNames({}).size).toBe(0);
  });
});

describe('requiredPolicies', () => {
  it('always needs subscription-validation, once, and nothing for conditions no policy enforces', () => {
    expect(
      requiredPolicies([
        { conditions: [limit('request-count', 'requests'), limit('token-count', 'tokens')] },
        { conditions: [custom('business-hours')] },
        {},
      ]),
    ).toEqual([SUBSCRIPTION_VALIDATION]);
  });
});

describe('planEnforcement', () => {
  const plans = [
    {
      conditions: [
        limit('token-count', 'tokens'),
        limit('request-count', 'requests'),
        custom('region'),
      ],
    },
    {
      conditions: [limit('request-count', 'requests'), custom('region'), custom('business-hours')],
    },
  ];

  it('lists each built-in type once in the API Portal order, then each custom ID once', () => {
    const { rows } = planEnforcement(plans, new Set([SUBSCRIPTION_VALIDATION]));
    expect(rows.map((row) => [row.type, row.id])).toEqual([
      ['request-count', 'requests'],
      ['token-count', 'tokens'],
      ['custom', 'region'],
      ['custom', 'business-hours'],
    ]);
  });

  it('marks request-count enforced by subscription-validation, and the rest as having no policy', () => {
    const result = planEnforcement(plans, new Set([SUBSCRIPTION_VALIDATION]));
    expect(result.validationMissing).toBe(false);
    expect(result.rows.map((row) => [row.id, row.status, row.policy])).toEqual([
      ['requests', 'enforced', SUBSCRIPTION_VALIDATION],
      ['tokens', 'noPolicy', null],
      ['region', 'noPolicy', null],
      ['business-hours', 'noPolicy', null],
    ]);
    expect(result.enforced).toBe(1);
  });

  it('marks request-count not enforced while subscription-validation is missing', () => {
    const result = planEnforcement(plans, new Set(['jwt-auth']));
    expect(result.validationMissing).toBe(true);
    expect(result.rows[0]).toMatchObject({ id: 'requests', status: 'notEnforced' });
    expect(result.enforced).toBe(0);
  });

  it('has no rows for plans with no conditions, but still says subscription-validation is missing', () => {
    const result = planEnforcement([{}, { conditions: [] }], new Set());
    expect(result.rows).toEqual([]);
    expect(result.validationMissing).toBe(true);
  });
});
