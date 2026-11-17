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
 * selectAllowedTarget: a try-it target must be a registered endpoint of the API,
 * and must pass the address ranges and network access control.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const destinationGuard = require('../utils/destinationGuard');

const r = (p) => path.resolve(__dirname, p);
// The defaults stand in for a loaded config.toml (the try-it client reads its TLS
// settings from it); each test sets its own tryout section.
const portalConfig = { ...structuredClone(require('../config/configDefaults').DEFAULTS), tryout: {} };
const STUBBED = {
    [r('../config/configLoader.js')]: { config: portalConfig },
    [r('../config/logger.js')]: { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} },
    [r('../dao/organizationDao.js')]: {},
    [r('../dao/apiDao.js')]: {},
    [r('./apiMetadataService.js')]: {},
};
for (const [p, exports] of Object.entries(STUBBED)) {
    require.cache[p] = { id: p, filename: p, loaded: true, exports };
}
const { selectAllowedTarget } = require('./tryoutProxyService');

function setup(t, tryout, accessControl) {
    portalConfig.tryout = tryout;
    destinationGuard.setAccessControl(destinationGuard.compileAccessControl(accessControl));
    t.after(() => destinationGuard.setAccessControl(null));
}

const refused = (fn) => assert.throws(fn, (err) => err.statusCode === 422);

test('private endpoints stay refused unless allow_private_endpoints is set to true', (t) => {
    const endpoints = ['http://10.0.0.5:8080/pizza'];

    setup(t, {}, { enabled: true });
    refused(() => selectAllowedTarget('http://10.0.0.5:8080/pizza/menu', endpoints)); // unset denies (was fail-open)

    setup(t, { allowPrivateEndpoints: true }, { enabled: true });
    assert.equal(selectAllowedTarget('http://10.0.0.5:8080/pizza/menu', endpoints).hostname, '10.0.0.5');

    setup(t, { allowPrivateEndpoints: true }, { enabled: true, denyPrivateNetworks: true });
    refused(() => selectAllowedTarget('http://10.0.0.5:8080/pizza/menu', endpoints)); // access control tightens
});

test('the host rule applies on top of the registered-endpoint check', (t) => {
    const endpoints = ['https://gw.example.com/pizza', 'https://gw.internal.example/pizza'];
    setup(t, {}, { enabled: true, mode: 'deny', hostnames: ['[a-z.]+\\.internal\\.example'] });

    assert.equal(selectAllowedTarget('https://gw.example.com/pizza/menu', endpoints).hostname, 'gw.example.com');
    refused(() => selectAllowedTarget('https://gw.internal.example/pizza/menu', endpoints));
});

test('the metadata address is refused whatever the configuration', (t) => {
    setup(t, { allowPrivateEndpoints: true }, { enabled: false });
    refused(() => selectAllowedTarget('http://169.254.169.254/latest', ['http://169.254.169.254/']));
});

// A try-it target is connected to directly, so an HTTP(S)_PROXY from the environment
// cannot carry it past the guarded lookup (which would then check the proxy's address).
test('the try-it client is not routed through an environment proxy', async (t) => {
    const http = require('node:http');
    const listen = (body, onRequest = () => {}) => new Promise((resolve) => {
        const server = http.createServer((req, res) => { onRequest(req); res.end(body); });
        server.listen(0, '127.0.0.1', () => resolve(server.address().port));
        t.after(() => server.close());
    });
    let proxied = 0;
    const proxyPort = await listen('via proxy', () => { proxied += 1; });
    const targetPort = await listen('direct');
    const saved = { HTTP_PROXY: process.env.HTTP_PROXY, NO_PROXY: process.env.NO_PROXY, no_proxy: process.env.no_proxy };
    delete process.env.NO_PROXY;
    delete process.env.no_proxy;
    process.env.HTTP_PROXY = `http://127.0.0.1:${proxyPort}`;
    t.after(() => {
        for (const [name, value] of Object.entries(saved)) {
            if (value === undefined) delete process.env[name];
            else process.env[name] = value;
        }
    });
    setup(t, { allowPrivateEndpoints: true }, { enabled: false });

    const { getClient } = require('./tryoutProxyService');
    const res = await getClient().get(`http://localhost:${targetPort}/`);
    assert.equal(Buffer.from(res.data).toString(), 'direct');
    assert.equal(proxied, 0, 'the try-it request went to the proxy');
});
