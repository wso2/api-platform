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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

'use strict';

/*
 * A user-supplied destination is connected to directly, so an HTTP(S)_PROXY from the
 * environment cannot carry it past the guarded lookup: through a proxy, axios would dial
 * the proxy and the lookup would check the proxy's address, not the target's. A
 * destination from config.toml keeps axios's proxy handling.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');

// The defaults stand in for a loaded config.toml: buildClient reads only the TLS and
// pooling settings from it.
const { DEFAULTS } = require('../../config/configDefaults');
const CONFIG_PATH = path.resolve(__dirname, '../../config/configLoader.js');
require.cache[CONFIG_PATH] = { id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true, exports: { config: structuredClone(DEFAULTS) } };

const { buildClient } = require('./httpClient');
const { compileAccessControl, setAccessControl } = require('../../utils/destinationGuard');

const LIMITS = { timeoutMs: 3000, maxResponseBytes: 4096, maxRequestBytes: 4096 };

function listen(t, body, onRequest = () => {}) {
    return new Promise((resolve) => {
        const server = http.createServer((req, res) => {
            onRequest(req);
            res.end(body);
        });
        server.listen(0, '127.0.0.1', () => resolve(server.address().port));
        t.after(() => server.close());
    });
}

// Routes every axios request in the test through the given proxy, the way an
// HTTP_PROXY set for the process would.
function useEnvironmentProxy(t, port) {
    const saved = {};
    for (const name of ['HTTP_PROXY', 'http_proxy', 'NO_PROXY', 'no_proxy']) {
        saved[name] = process.env[name];
        delete process.env[name];
    }
    process.env.HTTP_PROXY = `http://127.0.0.1:${port}`;
    t.after(() => {
        for (const [name, value] of Object.entries(saved)) {
            if (value === undefined) delete process.env[name];
            else process.env[name] = value;
        }
    });
}

test('a user-supplied key manager is not routed through an environment proxy', async (t) => {
    let proxied = 0;
    const proxyPort = await listen(t, 'via proxy', () => { proxied += 1; });
    const targetPort = await listen(t, 'direct');
    useEnvironmentProxy(t, proxyPort);
    setAccessControl(compileAccessControl({ enabled: true, denyPrivateNetworks: true }));
    t.after(() => setAccessControl(null));

    // localhost resolves to a private address, which this destination may not reach.
    const tenant = buildClient({ userSupplied: true, limits: LIMITS });
    await assert.rejects(tenant.get(`http://localhost:${targetPort}/`));
    assert.equal(proxied, 0, 'the refused request went to the proxy');

    // Positive control: a config.toml destination still honours the proxy, so the proxy
    // above was live and would have carried the user-supplied request.
    const configured = buildClient({ allowPrivateEndpoints: true, limits: LIMITS });
    const res = await configured.get(`http://localhost:${targetPort}/`);
    assert.equal(res.data, 'via proxy');
    assert.equal(proxied, 1);
});
