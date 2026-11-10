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
  enforcerOf,
  planEnforcement,
  policyAttachment,
  requiredPolicies,
  SUBSCRIPTION_VALIDATION,
  unenforcedConditionTypes,
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

describe('enforcerOf', () => {
  it('finds a custom condition\'s policy among its own IDs only', () => {
    for (const id of ['constructor', '__proto__', 'toString', 'hasOwnProperty']) {
      expect(enforcerOf({ id, type: 'custom' })).toBeNull();
      expect(enforcerOf({ id, type: id })).toBeNull();
    }
    expect(enforcerOf({ id: 'requests', type: 'request-count' })).toBe(SUBSCRIPTION_VALIDATION);
  });

  it('gives no policy row for a custom ID named after an Object property', () => {
    const { rows } = planEnforcement(
      [{ conditions: [custom('constructor')] }],
      new Set([SUBSCRIPTION_VALIDATION]),
    );
    expect(rows).toEqual([
      { id: 'constructor', partial: false, policy: null, status: 'noPolicy', type: 'custom' },
    ]);
    expect(requiredPolicies([{ conditions: [custom('constructor')] }])).toEqual([
      SUBSCRIPTION_VALIDATION,
    ]);
  });
});

const sv = { name: SUBSCRIPTION_VALIDATION, version: 'v1' };

describe('policyAttachment', () => {
  it('counts a policy on the API, or on every operation in either shape, as attached', () => {
    const { attached, partial } = policyAttachment({
      operations: [
        { request: { policies: [sv, { name: 'cors', version: 'v1' }] } },
        { policies: [sv] },
      ],
      policies: [{ name: 'jwt-auth', version: 'v1' }],
    });
    expect([...attached].sort()).toEqual(['jwt-auth', SUBSCRIPTION_VALIDATION]);
    expect([...partial]).toEqual(['cors']);
  });

  it('counts a policy on some operations only as partial, not attached', () => {
    const api = { operations: [{ request: { policies: [sv] } }, {}] };
    expect(policyAttachment(api)).toEqual({
      attached: new Set(),
      partial: new Set([SUBSCRIPTION_VALIDATION]),
    });
    expect(attachedPolicyNames(api).has(SUBSCRIPTION_VALIDATION)).toBe(false);
  });

  it('counts a global policy as attached whatever the operations hold', () => {
    const api = { operations: [{ policies: [sv] }, {}], policies: [sv] };
    expect(policyAttachment(api)).toEqual({
      attached: new Set([SUBSCRIPTION_VALIDATION]),
      partial: new Set(),
    });
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

  it('marks request-count not enforced, and why, while subscription-validation is on some resources only', () => {
    const result = planEnforcement(plans, new Set(['jwt-auth']), new Set([SUBSCRIPTION_VALIDATION]));
    expect(result.validationMissing).toBe(false);
    expect(result.validationPartial).toBe(true);
    expect(result.rows[0]).toMatchObject({ id: 'requests', partial: true, status: 'notEnforced' });
    expect(result.enforced).toBe(0);
  });

  it('has no rows for plans with no conditions, but still says subscription-validation is missing', () => {
    const result = planEnforcement([{}, { conditions: [] }], new Set());
    expect(result.rows).toEqual([]);
    expect(result.validationMissing).toBe(true);
  });
});

describe('unenforcedConditionTypes', () => {
  it('lists the condition types no policy enforces yet, once each, custom last', () => {
    expect(
      unenforcedConditionTypes([
        { conditions: [custom('region'), limit('token-count', 'tokens')] },
        {
          conditions: [
            limit('request-count', 'requests'),
            limit('event-count', 'events'),
            limit('token-count', 'tokens'),
          ],
        },
        {},
      ]),
    ).toEqual(['event-count', 'token-count', 'custom']);
  });

  it('is empty for plans whose conditions are all enforced', () => {
    expect(unenforcedConditionTypes([{ conditions: [limit('request-count', 'requests')] }])).toEqual(
      [],
    );
  });
});
