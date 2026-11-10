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
 * Render tests for partials/api-subscription-plans.hbs: a consumer may hold several
 * subscriptions to one API (same or different plans), so every plan keeps a Subscribe
 * button and every subscription gets its own row with a Manage action.
 *
 * handlebarsHelpers loads the app config at require time and exits without --config,
 * so a minimal fixture is pushed onto process.argv before it is required.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..', '..');
const configFile = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'ap-subplans-')), 'config.toml');
fs.writeFileSync(configFile, `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.auth.claim_mappings]
roles = "roles"

[api_portal.auth.authorization]
mode = "role"
role_to_scope_mapping = ${JSON.stringify(path.join(PROJECT_ROOT, 'resources', 'role-to-scope-mapping.yaml'))}
`);
process.argv.push('--config', configFile);

const Handlebars = require('handlebars');
const { registerHelpers } = require('../../../helpers/handlebarsHelpers');

registerHelpers();
const render = Handlebars.compile(fs.readFileSync(path.join(__dirname, 'partials', 'api-subscription-plans.hbs'), 'utf8'));

const PLANS = [
    { id: 'Gold', displayName: 'Gold', requestCount: 5 },
    { id: 'Silver', displayName: 'Silver', requestCount: 2 },
];

function sub(id, plan, status = 'ACTIVE') {
    return { subscriptionId: id, policyName: plan, subscriptionPlanName: plan, status, maskedToken: 'abcd****wxyz' };
}

function html(subscriptions, isAuthenticated = true) {
    return render({
        isAuthenticated, orgId: 'org-1', basePath: '/b', apiMetadata: { id: 'api-1' },
        subscriptionPlans: PLANS, subscriptions,
    });
}

const count = (haystack, needle) => haystack.split(needle).length - 1;

test('every plan offers Subscribe even when the consumer already holds a subscription to it', () => {
    const out = html([sub('s1', 'Gold')]);
    assert.ok(out.includes('id="subscribe-btn-Gold"'), 'Gold must stay subscribable');
    assert.ok(out.includes('id="subscribe-btn-Silver"'));
});

test('lists one row per subscription, including repeated plans', () => {
    const out = html([sub('s1', 'Gold'), sub('s2', 'Silver'), sub('s3', 'Gold')]);
    assert.strictEqual(count(out, 'class="api-view-sub-btn'), 3);
    for (const id of ['s1', 's2', 's3']) {
        assert.ok(out.includes(`data-subscription-id="${id}"`), `row for ${id}`);
    }
});

test('shows status and masked token per subscription row', () => {
    const out = html([sub('s1', 'Gold', 'INACTIVE')]);
    assert.ok(out.includes('abcd****wxyz'));
    assert.ok(out.includes('Suspended'));
});

test('renders no subscriptions table when the consumer has none', () => {
    const out = html([]);
    assert.ok(!out.includes('id="apiSubscriptionsTable"'));
    assert.ok(!out.includes('api-view-sub-btn'));
});

test('unauthenticated visitors see login links and no subscriptions table', () => {
    const out = html([], false);
    assert.ok(out.includes('/login'));
    assert.ok(!out.includes('id="apiSubscriptionsTable"'));
});

test('every script the partial loads is on the theme upload allowlist', () => {
    const { validateScripts } = require('../../../utils/util');
    const source = fs.readFileSync(path.join(__dirname, 'partials', 'api-subscription-plans.hbs'), 'utf8');
    assert.doesNotThrow(() => validateScripts(source));
});
