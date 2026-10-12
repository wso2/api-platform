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
const assert = require('node:assert/strict');

const { DEFAULTS } = require('./configDefaults');

function freshModule() {
    // buildOutboundAgents memoizes internally; re-require a fresh copy per test
    // so one test's cached agent doesn't leak into the next.
    delete require.cache[require.resolve('./httpClientOptions')];
    return require('./httpClientOptions');
}

test('buildOutboundAgents builds a keep-alive https.Agent with the configured TLS options', () => {
    const { buildOutboundAgents } = freshModule();
    const { httpAgent, httpsAgent, tlsOptions } = buildOutboundAgents({ httpClient: DEFAULTS.httpClient });

    assert.equal(httpAgent.keepAlive, true);
    assert.equal(httpsAgent.keepAlive, true);
    assert.equal(tlsOptions.minVersion, 'TLSv1.2');
    assert.equal(tlsOptions.maxVersion, 'TLSv1.3');
    assert.equal(tlsOptions.ecdhCurve, 'X25519:prime256v1');
    assert.equal(httpsAgent.options.minVersion, 'TLSv1.2');
    assert.equal(httpsAgent.options.ecdhCurve, 'X25519:prime256v1');
});

test('buildOutboundAgents memoizes across calls', () => {
    const { buildOutboundAgents } = freshModule();
    const cfg = { httpClient: DEFAULTS.httpClient };
    const first = buildOutboundAgents(cfg);
    const second = buildOutboundAgents(cfg);
    assert.equal(first, second);
    assert.equal(first.httpsAgent, second.httpsAgent);
});

test('buildOutboundAgents fails closed on an invalid cipher/curve/version', () => {
    const { buildOutboundAgents } = freshModule();
    assert.throws(() => buildOutboundAgents({
        httpClient: { ...DEFAULTS.httpClient, tls: { ...DEFAULTS.httpClient.tls, ecdhCurves: 'not-a-curve' } },
    }));
});
