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

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-sub-test-'));
test.after(() => fs.rmSync(tmpDir, { recursive: true, force: true }));

const cfgFile = path.join(tmpDir, 'config.toml');
fs.writeFileSync(cfgFile, `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(path.join(tmpDir, 'portal.db'))}
`);
process.argv.push('--config', cfgFile);

const PATHS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    apiDao: path.resolve(__dirname, '../dao/apiDao.js'),
    subDao: path.resolve(__dirname, '../dao/subscriptionDao.js'),
    userIdpReferenceDao: path.resolve(__dirname, '../dao/userIdpReferenceDao.js'),
    eventPublisher: path.resolve(__dirname, './webhooks/eventPublisher.js'),
    auditLogger: path.resolve(__dirname, '../middlewares/auditLogger.js'),
    subscriptionService: path.resolve(__dirname, './subscriptionService.js'),
};

function loadService(stubs) {
    const defaultStubs = {
        driver: {
            withTransaction: async (fn) => fn({}),
            isDuplicateKeyError: (err) => Boolean(err && (err.code === '23505' || err.isDuplicate)),
        },
        apiDao: {
            getId: async () => 'api-uuid-1',
            get: async () => [{
                uuid: 'api-uuid-1',
                name: 'test-api',
                subscription_plans: [{ uuid: 'plan-uuid-1', handle: 'gold', display_name: 'Gold' }],
            }],
        },
        subDao: {
            findByApiAndUser: async () => null,
            create: async () => ({ uuid: 'sub-new-1', token: 'token-123' }),
            get: async () => ({ uuid: 'sub-new-1', token: 'token-123' }),
        },
        userIdpReferenceDao: {
            resolveDisplay: async (actor) => actor,
            buildSingleAuditFields: async () => ({}),
        },
        eventPublisher: {
            publish: async () => {},
        },
        auditLogger: {
            logUserAction: () => {},
        },
    };

    const merged = { ...defaultStubs, ...stubs };
    for (const [k, exports] of Object.entries(merged)) {
        if (PATHS[k]) {
            require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
        }
    }
    delete require.cache[PATHS.subscriptionService];
    return require(PATHS.subscriptionService);
}

function call(handler, body, user = { sub: 'user-1' }) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            status(code) { this.statusCode = code; return this; },
            json(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
            send(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
        };
        const req = {
            orgId: 'org-1',
            user,
            query: {},
            params: {},
            body,
            headers: {},
        };
        Promise.resolve(handler(req, res)).catch((error) => resolve({ status: 'THREW', body: error.message }));
    });
}

test('createSubscription successfully creates a new subscription when none exists', async () => {
    const service = loadService({});
    const res = await call(service.createSubscription, {
        artifactId: 'test-api',
        subscriptionPlanId: 'gold',
    });
    assert.strictEqual(res.status, 201);
});

test('createSubscription rejects duplicate subscription with 409 Conflict', async () => {
    const service = loadService({
        subDao: {
            findByApiAndUser: async () => ({ uuid: 'sub-existing-1' }),
        },
    });

    const res = await call(service.createSubscription, {
        artifactId: 'test-api',
        subscriptionPlanId: 'gold',
    });

    assert.strictEqual(res.status, 409);
    assert.strictEqual(res.body.errors[0].message, 'A subscription for this API already exists');
});

test('createSubscription rejects in-transaction duplicate check with 409 Conflict', async () => {
    let callCount = 0;
    const service = loadService({
        subDao: {
            findByApiAndUser: async () => {
                callCount++;
                if (callCount === 1) return null;
                return { uuid: 'sub-existing-concurrent' };
            },
        },
    });

    const res = await call(service.createSubscription, {
        artifactId: 'test-api',
        subscriptionPlanId: 'gold',
    });

    assert.strictEqual(res.status, 409);
    assert.strictEqual(res.body.errors[0].message, 'A subscription for this API already exists');
});

test('createSubscription converts database duplicate key error to 409 Conflict', async () => {
    const service = loadService({
        subDao: {
            findByApiAndUser: async () => null,
            create: async () => {
                const err = new Error('duplicate key value violates unique constraint');
                err.code = '23505';
                throw err;
            },
        },
    });

    const res = await call(service.createSubscription, {
        artifactId: 'test-api',
        subscriptionPlanId: 'gold',
    });

    assert.strictEqual(res.status, 409);
    assert.strictEqual(res.body.errors[0].message, 'A subscription for this API already exists');
});

test('createSubscription handles exhausted token collision with 500 error', async () => {
    const service = loadService({
        subDao: {
            findByApiAndUser: async () => null,
            create: async () => {
                const err = new Error('Failed to generate unique subscription token after multiple attempts');
                err.isTokenCollision = true;
                throw err;
            },
        },
    });

    const res = await call(service.createSubscription, {
        artifactId: 'test-api',
        subscriptionPlanId: 'gold',
    });

    assert.strictEqual(res.status, 500);
    assert.strictEqual(res.body.errors[0].message, 'Failed to generate subscription token');
});
