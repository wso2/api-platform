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
 * Webhook subscriber URLs are tenant input, so every delivery is subject to
 * [api_portal.server.network_security.access_control]: checked up front for an IP
 * literal and the host rule, and at dial time for a hostname's resolved addresses.
 */

const test = require('node:test');
const assert = require('node:assert');
const http = require('node:http');
const path = require('node:path');

const destinationGuard = require('../../utils/destinationGuard');

const r = (p) => path.resolve(__dirname, p);
const { DEFAULTS } = require('../../config/configDefaults');
const workerConfig = { httpClient: DEFAULTS.httpClient, webhooks: { delivery: {} } };
const STUBBED = {
    [r('../../config/configLoader.js')]: { config: workerConfig },
    [r('../../db/driver.js')]: {},
    [r('../../dao/eventDao.js')]: {},
    [r('./subscriberRegistry.js')]: { getSubscriber: async (id) => (id === 'sub-1' ? { id } : null) },
    [r('./signer.js')]: { sign: () => ({}) },
    [r('../../config/logger.js')]: { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} },
    [r('../../utils/orgContext.js')]: {},
};
const WORKER = r('./deliveryWorker.js');

function loadWorker() {
    for (const [p, exports] of Object.entries(STUBBED)) {
        require.cache[p] = { id: p, filename: p, loaded: true, exports };
    }
    delete require.cache[WORKER]; // fresh agents, built under the current access control
    try {
        return require(WORKER);
    } finally {
        for (const p of Object.keys(STUBBED)) delete require.cache[p];
    }
}

async function withSubscriber(fn) {
    const received = [];
    const server = http.createServer((req, res) => {
        received.push(req.url);
        res.writeHead(204).end();
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
        return await fn(server.address().port, received);
    } finally {
        server.close();
    }
}

function deliver(worker, targetUrl) {
    return worker.post(
        { uuid: 'd-1', subscriber_id: 'sub-1', target_url: targetUrl },
        { uuid: 'e-1', type: 'subscription.created', occurred_at: new Date().toISOString(), org_uuid: 'org-1', payload: {} },
    );
}

function useAccessControl(t, cfg) {
    destinationGuard.setAccessControl(destinationGuard.compileAccessControl(cfg));
    t.after(() => destinationGuard.setAccessControl(null));
}

test('a private subscriber is reached by default and refused with deny_private_networks', async (t) => {
    await withSubscriber(async (port, received) => {
        useAccessControl(t, { enabled: true });
        const ok = await deliver(loadWorker(), `http://127.0.0.1:${port}/hook`);
        assert.strictEqual(ok.ok, true, 'positive control: private networks are permitted by default');

        useAccessControl(t, { enabled: true, denyPrivateNetworks: true });
        const refused = await deliver(loadWorker(), `http://127.0.0.1:${port}/hook`);
        assert.deepStrictEqual(refused, { ok: false, error: 'Destination is not allowed' });
        assert.strictEqual(received.length, 1, 'nothing reaches the subscriber once refused');
    });
});

test('a hostname resolving to a refused address is stopped at dial time', async (t) => {
    await withSubscriber(async (port, received) => {
        useAccessControl(t, { enabled: true, denyPrivateNetworks: true });
        const refused = await deliver(loadWorker(), `http://localhost:${port}/hook`);
        assert.strictEqual(refused.ok, false);
        assert.strictEqual(refused.error, 'Destination is not allowed');
        assert.deepStrictEqual(received, []);
    });
});

test('the host rule applies to every delivery', async (t) => {
    await withSubscriber(async (port, received) => {
        useAccessControl(t, { enabled: true, mode: 'allow', hostnames: ['localhost'] });
        const worker = loadWorker();
        const listed = await deliver(worker, `http://localhost:${port}/hook`);
        assert.strictEqual(listed.ok, true, 'positive control: a listed host is delivered to');
        const unlisted = await deliver(worker, `http://127.0.0.1:${port}/hook`);
        assert.deepStrictEqual(unlisted, { ok: false, error: 'Destination is not allowed' });
        assert.strictEqual(received.length, 1);
    });
});

test('the metadata address and non-http schemes are always refused', async (t) => {
    useAccessControl(t, { enabled: true });
    const worker = loadWorker();
    assert.deepStrictEqual(await deliver(worker, 'http://169.254.169.254/latest'), { ok: false, error: 'Destination is not allowed' });
    assert.deepStrictEqual(await deliver(worker, 'ftp://hooks.example.com/x'), { ok: false, error: 'Destination is not allowed' });
});
