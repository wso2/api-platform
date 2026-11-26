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
 * The developer token proxy posts the developer's client secret to the token
 * endpoint of a key manager created through the API, so it goes through the
 * guarded key-manager client: network access control, no redirects.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');

const destinationGuard = require('../utils/destinationGuard');
const { DEFAULTS } = require('../config/configDefaults');

const r = (p) => path.resolve(__dirname, p);
const STUBBED = {
    [r('../config/configLoader.js')]: { config: { httpClient: DEFAULTS.httpClient, keyManagerClient: {} } },
    [r('../config/logger.js')]: { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} },
    // Created through the API, private addresses permitted by provisioning config.
    [r('./keyManagerDriverBuilder.js')]: {
        DB_CLIENT_POLICY: { userSupplied: true, allowPrivateEndpoints: true, allowHttpEndpoints: true },
    },
};

function loadService() {
    for (const [p, exports] of Object.entries(STUBBED)) {
        require.cache[p] = { id: p, filename: p, loaded: true, exports };
    }
    delete require.cache[r('./oauthTokenService.js')]; // a fresh client per access-control setting
    return require('./oauthTokenService');
}

async function withTokenEndpoint(handler, fn) {
    const server = http.createServer(handler);
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
        return await fn(`http://127.0.0.1:${server.address().port}/oauth2/token`);
    } finally {
        server.close();
    }
}

const issueToken = (req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ access_token: 't0k3n', expires_in: 3600, token_type: 'Bearer' }));
};

function useAccessControl(t, cfg) {
    destinationGuard.setAccessControl(destinationGuard.compileAccessControl(cfg));
    t.after(() => destinationGuard.setAccessControl(null));
}

test('a permitted token endpoint issues a token', async (t) => {
    useAccessControl(t, { enabled: true });
    await withTokenEndpoint(issueToken, async (endpoint) => {
        const token = await loadService().generateToken(endpoint, 'id', 'secret', ['default'], 3600);
        assert.equal(token.accessToken, 't0k3n');
    });
});

test('a refused token endpoint is a generic 422', async (t) => {
    useAccessControl(t, { enabled: true, denyPrivateNetworks: true });
    await withTokenEndpoint(issueToken, async (endpoint) => {
        await assert.rejects(loadService().generateToken(endpoint, 'id', 'secret', [], 0), (err) => {
            assert.equal(err.statusCode ?? err.status ?? err.code, 422);
            assert.doesNotMatch(err.message, /127\.0\.0\.1/);
            return true;
        });
    });
});

test('the metadata address is refused and a redirect is not followed', async (t) => {
    useAccessControl(t, { enabled: true });
    const service = loadService();
    await assert.rejects(service.generateToken('http://169.254.169.254/token', 'id', 'secret', [], 0));

    await withTokenEndpoint((req, res) => {
        res.writeHead(302, { Location: 'http://169.254.169.254/steal' });
        res.end();
    }, async (endpoint) => {
        await assert.rejects(service.generateToken(endpoint, 'id', 'secret', [], 0), (err) => {
            assert.equal(err.statusCode ?? err.status ?? err.code, 500);
            return true;
        });
    });
});
