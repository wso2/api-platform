/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

'use strict';

/*
 * The startup idp_ref_id reconcile policy (idpOrgIdPolicy.planIdpOrgIdReconcile),
 * applied by seederService.reconcileIdpOrgId on every boot.
 *
 * idp_ref_id is what incoming token org claims are matched against
 * (ensureAuthenticated.belongsToTargetOrg), and auth.idp_org_id in config is
 * its only writer — so the two failure modes worth pinning down are a boot that
 * silently rewrites a working value, and one that refuses a legitimate correction.
 */

const test = require('node:test');
const assert = require('node:assert');

const { planIdpOrgIdReconcile } = require('./idpOrgIdPolicy');

test('unset configuration leaves the stored value alone', () => {
    // The regression this guards: getIdpOrgId() falls back to the handle, so a
    // reconcile keyed on it would rewrite a deliberately-set 'ACME-PROD' back to
    // 'acme' on the first boot after the setting was dropped from config.
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: '', stored: 'ACME-PROD' }),
        { action: 'skip' }
    );
});

test('a value already in sync is not rewritten', () => {
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'ACME-PROD', stored: 'ACME-PROD' }),
        { action: 'skip' }
    );
});

test('a changed configured value is written', () => {
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'ACME-PROD', stored: 'acme' }),
        { action: 'update' }
    );
});

test('a case-only difference counts as a change', () => {
    // idp_ref_id is compared verbatim against the token claim, unlike the handle,
    // so 'acme' and 'ACME' are genuinely different matching keys.
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'ACME', stored: 'acme' }),
        { action: 'update' }
    );
});

test('an empty stored value is filled in from configuration', () => {
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'ACME-PROD', stored: '' }),
        { action: 'update' }
    );
});

test('a value another organization already answers to is refused', () => {
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'globex', stored: 'acme', conflictingOrgHandle: 'globex' }),
        { action: 'conflict' }
    );
});

test('a conflict on an already-in-sync value is still a no-op, not an error', () => {
    // Ordering matters here: the in-sync check comes first, so a neighbouring
    // organization that happens to share this value cannot turn every boot of an
    // already-correct deployment into a logged conflict.
    assert.deepStrictEqual(
        planIdpOrgIdReconcile({ configured: 'acme', stored: 'acme', conflictingOrgHandle: 'other' }),
        { action: 'skip' }
    );
});
