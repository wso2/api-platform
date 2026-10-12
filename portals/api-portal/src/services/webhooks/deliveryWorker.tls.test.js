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
const path = require('node:path');

const r = (p) => path.resolve(__dirname, p);
const STUBBED = {
    [r('../../config/configLoader.js')]: { config: {} },
    [r('../../db/driver.js')]: {},
    [r('../../dao/eventDao.js')]: {},
    [r('./subscriberRegistry.js')]: { getSubscriber: () => null },
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
