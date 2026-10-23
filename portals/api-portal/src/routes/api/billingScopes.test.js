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
 * Who may call each billing operation, as shipped: the scopes each operation declares in the
 * OpenAPI spec (any one of them is enough, as OAuth2Security checks), against the scopes each role
 * of resources/role-to-scope-mapping.yaml grants. Administrators may call every billing operation;
 * subscribers only those on their own subscriptions, checkouts, invoices and usage, and reading
 * plans; a caller with no scope none. A billing operation added to the spec must be placed in one
 * of the lists below.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const yaml = require('../../utils/yaml');
const roleScopeMap = require('../../config/roleScopeMap');

const SPEC_PATH = path.join(__dirname, '..', '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml');
const MAPPING_PATH = path.join(__dirname, '..', '..', '..', 'resources', 'role-to-scope-mapping.yaml');

// Billing operations an administrator may call and a subscriber may not.
const ADMIN_ONLY = [
    'GET /billing-configuration', 'POST /billing-configuration', 'DELETE /billing-configuration',
    'PUT /billing-configuration/webhook-endpoint',
    'GET /billing-reports/revenue', 'GET /billing-reports/invoices',
    'GET /meters', 'POST /meters', 'GET /meters/{meterId}', 'PUT /meters/{meterId}', 'DELETE /meters/{meterId}',
    'POST /subscription-plans', 'PUT /subscription-plans', 'DELETE /subscription-plans/{planId}',
    'GET /webhook-events', 'GET /webhook-events/{eventId}',
];
// Billing operations a subscriber may call too (on their own records; the services filter by caller).
const SUBSCRIBER = [
    'POST /checkout-sessions', 'GET /checkout-sessions/{checkoutId}',
    'GET /usage', 'GET /billing/invoices', 'POST /billing/billing-portal-sessions',
    'POST /subscriptions', 'GET /subscriptions', 'GET /subscriptions/{subId}', 'PUT /subscriptions/{subId}',
    'DELETE /subscriptions/{subId}', 'POST /subscriptions/{subId}/change-plan',
    'POST /subscriptions/{subId}/regenerate-token', 'GET /subscriptions/{subId}/billing',
    'GET /subscription-plans', 'GET /subscription-plans/{planId}', 'GET /subscription-plans/{planId}/pricing',
];
const BILLING_PATH = /billing|meter|pricing|checkout|usage|revenue|invoice|webhook-events|subscription-plans|subscriptions/;

const spec = yaml.load(fs.readFileSync(SPEC_PATH, 'utf8'));
roleScopeMap.init(MAPPING_PATH, SPEC_PATH);

/** The scopes an operation accepts, any one of them being enough. */
function acceptedScopes(operation) {
    const [method, route] = operation.split(' ');
    const op = spec.paths[route] && spec.paths[route][method.toLowerCase()];
    assert.ok(op, `the spec has no ${operation}`);
    return (op.security || spec.security || []).flatMap((requirement) => Object.values(requirement).flat());
}

const allows = (scopes, operation) => acceptedScopes(operation).some((scope) => scopes.includes(scope));
const callable = (scopes) => [...ADMIN_ONLY, ...SUBSCRIBER].filter((operation) => allows(scopes, operation));

test('every billing operation in the spec is placed as administrator-only or open to subscribers', () => {
    const listed = new Set([...ADMIN_ONLY, ...SUBSCRIBER]);
    const unplaced = [];
    for (const [route, operations] of Object.entries(spec.paths)) {
        if (!BILLING_PATH.test(route)) continue;
        for (const method of Object.keys(operations)) {
            if (!['get', 'post', 'put', 'patch', 'delete'].includes(method)) continue;
            const operation = `${method.toUpperCase()} ${route}`;
            if (!listed.has(operation)) unplaced.push(operation);
        }
    }
    assert.deepEqual(unplaced, []);
});

test('every billing operation requires a scope', () => {
    for (const operation of [...ADMIN_ONLY, ...SUBSCRIBER]) {
        assert.ok(acceptedScopes(operation).length > 0, `${operation} accepts any signed-in caller`);
    }
});

for (const role of ['dp_admin', 'ap_admin']) {
    test(`${role} may call every billing operation`, () => {
        assert.deepEqual(callable(roleScopeMap.expandRoles([role])), [...ADMIN_ONLY, ...SUBSCRIBER]);
    });
}

for (const role of ['dp_subscriber', 'ap_subscriber']) {
    test(`${role} may call the operations on their own subscriptions and billing, and read plans, only`, () => {
        assert.deepEqual(callable(roleScopeMap.expandRoles([role])), SUBSCRIBER);
    });
}

test('a caller with no scope may call no billing operation', () => {
    assert.deepEqual(callable([]), []);
});

test('reading subscriptions lets a caller see invoices and usage but not change payment details', () => {
    const readOnly = ['dp:subscription:read'];
    assert.ok(allows(readOnly, 'GET /billing/invoices'));
    assert.ok(allows(readOnly, 'GET /usage'));
    assert.ok(!allows(readOnly, 'POST /billing/billing-portal-sessions'), "the billing page changes payment details");
    assert.ok(allows(['dp:subscription:update'], 'POST /billing/billing-portal-sessions'));
});
