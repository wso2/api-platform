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
 * webhookSubscriberService's admin-API refusal path (FR-012): when a self-seed
 * config block is active, the row with handle "platform-api" is treated as
 * config-owned and the admin API MUST refuse PUT and DELETE on it. Every
 * other subscriber id, and all operations when the config block is empty,
 * MUST continue to work as before.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const r = (p) => path.resolve(__dirname, p);
const PATHS = {
    db: r('../db/driver.js'),
    whDao: r('../dao/webhookSubscriberDao.js'),
    eventDao: r('../dao/eventDao.js'),
    dto: r('../dto/webhookSubscriberDto.js'),
    userIdpRef: r('../dao/userIdpReferenceDao.js'),
    constants: r('../utils/constants.js'),
    util: r('../utils/util.js'),
    logger: r('../config/logger.js'),
    audit: r('../middlewares/auditLogger.js'),
    seeder: r('./seederService.js'),
    svc: r('./webhookSubscriberService.js'),
};

// loadService replaces every require()-time dependency of
// webhookSubscriberService with a controllable stub so each test can drive
// the config-owned predicate without having to boot the real db, dao,
// configLoader, orgContext, or dto layers.
function loadService({ configOwned, whDao }) {
    const captured = { sendErrors: [], responses: [] };
    const stubs = {
        db: { isDuplicateKeyError: () => false },
        whDao: whDao || {
            get: async () => assert.fail('whDao.get should not be reached when the row is config-owned'),
            update: async () => assert.fail('whDao.update should not be reached when the row is config-owned'),
            delete: async () => assert.fail('whDao.delete should not be reached when the row is config-owned'),
        },
        eventDao: {},
        dto: { WebhookSubscriberDTO: class {} },
        userIdpRef: { buildSingleAuditFields: async () => ({}) },
        constants: { ERROR_MESSAGE: {} },
        util: {
            sendError: (res, status, message) => {
                captured.sendErrors.push({ status, message });
                res.statusCode = status;
                res._body = { error: message };
                return res;
            },
            resolveActor: () => 'tester',
            toPaginatedList: (arr) => ({ items: arr }),
        },
        logger: { error: () => {}, warn: () => {}, info: () => {} },
        audit: { logUserAction: () => {} },
        seeder: {
            isPlatformApiSubscriberConfigOwned: () => configOwned,
            PLATFORM_API_SUBSCRIBER_HANDLE: 'platform-api',
        },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.svc];
    try {
        return { svc: require(PATHS.svc), captured };
    } finally {
        for (const p of Object.values(PATHS)) delete require.cache[p];
    }
}

// mockRes implements the subset of Express's Response that
// webhookSubscriberService touches: status() returns `this` for chaining,
// json() captures the payload, and statusCode is set directly by sendError.
function mockRes() {
    const res = { statusCode: 0, _body: null };
    res.status = (s) => { res.statusCode = s; return res; };
    res.json = (b) => { res._body = b; return res; };
    res.send = () => res;
    return res;
}

test('PUT on platform-api subscriber is refused with 403 when config-owned', async () => {
    const { svc, captured } = loadService({ configOwned: true });
    const req = { orgId: 'u-acme', params: { subscriberId: 'platform-api' }, body: { targetUrl: 'https://attacker.example.com' } };
    const res = mockRes();
    await svc.updateWebhookSubscriber(req, res);
    assert.strictEqual(captured.sendErrors.length, 1, 'expected exactly one sendError call');
    assert.strictEqual(captured.sendErrors[0].status, 403);
    assert.match(captured.sendErrors[0].message, /managed by server configuration/);
});

test('DELETE on platform-api subscriber is refused with 403 when config-owned', async () => {
    const { svc, captured } = loadService({ configOwned: true });
    const req = { orgId: 'u-acme', params: { subscriberId: 'platform-api' } };
    const res = mockRes();
    await svc.deleteWebhookSubscriber(req, res);
    assert.strictEqual(captured.sendErrors.length, 1);
    assert.strictEqual(captured.sendErrors[0].status, 403);
    assert.match(captured.sendErrors[0].message, /managed by server configuration/);
});

test('PUT on platform-api subscriber passes through when NOT config-owned', async () => {
    // When isPlatformApiSubscriberConfigOwned() returns false, the admin PUT
    // reaches whDao.update exactly as it does for every other subscriber id.
    const whDao = {
        updateCalls: 0,
        get: async () => null,
        update: async function (orgId, handle, _payload, _actor) {
            this.updateCalls++;
            return [1, [{ uuid: 'row-1', handle, display_name: 'DN', target_url: 'https://new', enabled: 1, timeout_ms: 5000 }]];
        },
        delete: async () => {},
    };
    const { svc } = loadService({ configOwned: false, whDao });
    const req = { orgId: 'u-acme', params: { subscriberId: 'platform-api' }, body: { targetUrl: 'https://new' } };
    const res = mockRes();
    await svc.updateWebhookSubscriber(req, res);
    assert.strictEqual(whDao.updateCalls, 1, 'expected whDao.update to be reached');
});

test('PUT on any other subscriber id reaches whDao even when config-owned', async () => {
    // The config-owned refusal is scoped to the "platform-api" handle. A
    // different id (even one the operator typed by hand) stays mutable.
    const whDao = {
        updateCalls: 0,
        get: async () => null,
        update: async function (orgId, handle) {
            this.updateCalls++;
            return [1, [{ uuid: 'row-x', handle, display_name: 'DN', target_url: 'https://x', enabled: 1, timeout_ms: 5000 }]];
        },
        delete: async () => {},
    };
    const { svc } = loadService({ configOwned: true, whDao });
    const req = { orgId: 'u-acme', params: { subscriberId: 'my-other-subscriber' }, body: { targetUrl: 'https://x' } };
    const res = mockRes();
    await svc.updateWebhookSubscriber(req, res);
    assert.strictEqual(whDao.updateCalls, 1, 'the config-owned refusal must only apply to the "platform-api" handle');
});
