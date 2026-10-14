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
 * webhooks.delivery.tlsSkipVerify relaxes certificate checks for webhook delivery
 * only; every other outbound call keeps using the shared, verifying agent.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const r = (p) => path.resolve(__dirname, p);
// Mutated per test: deliveryWorker reads its config from this module's export.
const workerConfig = {};
const STUBBED = {
    [r('../../config/configLoader.js')]: { config: workerConfig },
    [r('../../db/driver.js')]: {},
    [r('../../dao/eventDao.js')]: {},
    [r('./subscriberRegistry.js')]: { getSubscriber: async (id) => (id === 'sub-1' ? { id } : null) },
    [r('./signer.js')]: { sign: () => ({}) },
    [r('../../config/logger.js')]: { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} },
    [r('../../utils/orgContext.js')]: {},
};
const { DEFAULTS } = require('../../config/configDefaults');
const WORKER = r('./deliveryWorker.js');
const HTTP_CLIENT_OPTIONS = r('../../config/httpClientOptions.js');

function loadWorker() {
    for (const [p, exports] of Object.entries(STUBBED)) {
        require.cache[p] = { id: p, filename: p, loaded: true, exports };
    }
    // Fresh copies: both modules memoize their agents.
    delete require.cache[WORKER];
    delete require.cache[HTTP_CLIENT_OPTIONS];
    try {
        return {
            worker: require(WORKER),
            shared: require(HTTP_CLIENT_OPTIONS),
        };
    } finally {
        for (const p of Object.keys(STUBBED)) delete require.cache[p];
    }
}

const cfg = (tlsSkipVerify) => ({ httpClient: DEFAULTS.httpClient, webhooks: { delivery: { tlsSkipVerify } } });

test('delivery uses the shared verifying agent by default', () => {
    const { worker, shared } = loadWorker();
    const agent = worker.deliveryHttpsAgent(cfg(false));
    assert.strictEqual(agent, shared.buildOutboundAgents(cfg(false)).httpsAgent);
    assert.notStrictEqual(agent.options.rejectUnauthorized, false);
});

test('tlsSkipVerify gives delivery its own agent and leaves the shared one verifying', () => {
    const { worker, shared } = loadWorker();
    const agent = worker.deliveryHttpsAgent(cfg(true));
    assert.strictEqual(agent.options.rejectUnauthorized, false);
    const sharedAgent = shared.buildOutboundAgents(cfg(true)).httpsAgent;
    assert.notStrictEqual(agent, sharedAgent);
    assert.notStrictEqual(sharedAgent.options.rejectUnauthorized, false);
    // Memoized: one insecure agent, reused across deliveries.
    assert.strictEqual(worker.deliveryHttpsAgent(cfg(true)), agent);
});

test('only a boolean true skips verification', () => {
    const { worker } = loadWorker();
    assert.notStrictEqual(worker.deliveryHttpsAgent(cfg('true')).options.rejectUnauthorized, false);
});

// A delivery to an HTTPS subscriber with a self-signed certificate: rejected by
// default, accepted with tlsSkipVerify. The certificate is generated per run
// rather than committed.
function selfSignedCert() {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-delivery-tls-'));
    const key = path.join(dir, 'key.pem');
    const cert = path.join(dir, 'cert.pem');
    const res = spawnSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
        '-subj', '/CN=localhost', '-keyout', key, '-out', cert], { encoding: 'utf8' });
    if (res.status !== 0) return null;
    const out = { key: fs.readFileSync(key), cert: fs.readFileSync(cert) };
    fs.rmSync(dir, { recursive: true, force: true });
    return out;
}

async function deliverToSelfSignedSubscriber(tlsSkipVerify) {
    const pem = selfSignedCert();
    if (!pem) return null;
    const received = [];
    const server = https.createServer(pem, (req, res) => {
        received.push(req.headers['x-api-portal-event']);
        res.writeHead(204).end();
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
        Object.assign(workerConfig, cfg(tlsSkipVerify));
        const { worker } = loadWorker();
        const result = await worker.post(
            { uuid: 'd-1', subscriber_id: 'sub-1', target_url: `https://127.0.0.1:${server.address().port}/hook` },
            { uuid: 'e-1', type: 'subscription.created', occurred_at: new Date().toISOString(), org_uuid: 'org-1', payload: {} },
        );
        return { result, received };
    } finally {
        server.close();
    }
}

test('delivery to a self-signed subscriber fails by default', async (t) => {
    const out = await deliverToSelfSignedSubscriber(false);
    if (!out) return t.skip('openssl not available');
    assert.strictEqual(out.result.ok, false);
    assert.deepStrictEqual(out.received, []);
});

test('delivery to a self-signed subscriber succeeds with tlsSkipVerify', async (t) => {
    const out = await deliverToSelfSignedSubscriber(true);
    if (!out) return t.skip('openssl not available');
    assert.strictEqual(out.result.ok, true);
    assert.strictEqual(out.result.status, 204);
    assert.deepStrictEqual(out.received, ['subscription.created']);
});

