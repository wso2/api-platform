// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// POST /applications/{applicationId}/oauth-keys/{keyMappingId}/generate-token
// (src/controllers/devportalController.js -> src/services/oauthTokenService.js
// -> axios.post(tokenEndpoint, ..., { auth: { username: clientId, password: clientSecret } })).
// Request: { consumerSecret, scopes?, validityPeriod? }
// Response: { accessToken, validityTime, tokenScopes }
//
// Rather than requiring a live Asgardeo/WSO2IS tenant, this stands up a minimal
// OAuth2 client-credentials token endpoint locally (like support/webhook-sink.js
// does for webhook delivery) and points a key manager at it — oauthTokenService.js
// just POSTs to whatever tokenEndpoint the key manager is configured with.
// `admin` creates the key manager; `developer` owns the application and its
// key mapping (appDao scopes application lookups by the caller's created_by).

const http = require('http');
const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const VALID_SECRET = 'valid-consumer-secret';

function createMockTokenServer() {
    let server;
    let lastRequestParams = new URLSearchParams();
    function start(port) {
        return new Promise((resolve, reject) => {
            server = http.createServer((req, res) => {
                let rawBody = '';
                req.on('data', (chunk) => { rawBody += chunk; });
                req.on('end', () => {
                    lastRequestParams = new URLSearchParams(rawBody);
                    const auth = req.headers.authorization || '';
                    const [, encoded] = auth.split(' ');
                    const [, password] = Buffer.from(encoded || '', 'base64').toString('utf8').split(':');
                    if (password !== VALID_SECRET) {
                        res.writeHead(401, { 'Content-Type': 'application/json' });
                        return res.end(JSON.stringify({ error: 'invalid_client' }));
                    }
                    // Echo back what was actually posted (oauthTokenService.js's
                    // generateToken form-encodes `scope`/`expiry_time`) rather than a
                    // fixed response, so tests can prove the request round-trip instead
                    // of just checking a value that would look identical either way.
                    res.writeHead(200, { 'Content-Type': 'application/json' });
                    res.end(JSON.stringify({
                        access_token: 'mock-access-token',
                        expires_in: Number(lastRequestParams.get('expiry_time')) || 3600,
                        scope: lastRequestParams.get('scope') || '',
                    }));
                });
            });
            server.on('error', reject);
            server.listen(port, () => resolve(server.address().port));
        });
    }
    function stop() {
        return new Promise((resolve) => (server ? server.close(resolve) : resolve()));
    }
    function getLastRequestParams() {
        return lastRequestParams;
    }
    return { start, stop, getLastRequestParams };
}

describe('OAuth token generation', () => {
    let tokenServer;
    // Same reachability pattern as support/webhook-sink.js's WEBHOOK_SINK_URL —
    // devportal must be able to reach this test container at this address.
    const tokenUrl = new URL(process.env.MOCK_TOKEN_ENDPOINT_URL || 'http://localhost:4504');

    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');
        tokenServer = createMockTokenServer();
        await tokenServer.start(Number(tokenUrl.port));
    });

    afterAll(async () => {
        await tokenServer.stop();
    });

    async function setupAppWithKeyMapping() {
        const kmId = uniqueHandle('km');
        await client.as('admin').post('/key-managers', {
            id: kmId, displayName: 'Mock KM', tokenEndpoint: tokenUrl.href,
        });

        const appId = uniqueHandle('app');
        await client.as('developer').post('/applications', { id: appId, displayName: 'Token Test App', description: 'd' });

        const mapping = await client.as('developer').post(`/applications/${appId}/generate-keys`, {
            keyManager: kmId,
            type: 'PRODUCTION',
            consumerKey: 'mock-consumer-key',
        });
        expect(mapping.status).toBe(200);
        return { appId, keyMappingId: mapping.body.keyMappingId };
    }

    it('generates a token for an application via its key manager mapping', async () => {
        const { appId, keyMappingId } = await setupAppWithKeyMapping();

        const res = await client.as('developer').post(
            `/applications/${appId}/oauth-keys/${keyMappingId}/generate-token`,
            { consumerSecret: VALID_SECRET }
        );
        expect(res.status).toBe(200);
        expect(res.body.accessToken).toBe('mock-access-token');
        expect(res.body.validityTime).toBe(3600);
    });

    it('rejects generation with an incorrect consumer secret', async () => {
        const { appId, keyMappingId } = await setupAppWithKeyMapping();

        const res = await client.as('developer').post(
            `/applications/${appId}/oauth-keys/${keyMappingId}/generate-token`,
            { consumerSecret: 'wrong-secret' }
        );
        // The mock key manager returns 401 for a bad secret; the devportal must
        // propagate that client error, not mask it as a 500.
        expect([400, 401]).toContain(res.status);
    });

    it('applies the default scope/validity period when omitted', async () => {
        const { appId, keyMappingId } = await setupAppWithKeyMapping();

        const res = await client.as('developer').post(
            `/applications/${appId}/oauth-keys/${keyMappingId}/generate-token`,
            { consumerSecret: VALID_SECRET }
        );
        expect(res.status).toBe(200);

        // devportalController.js's generateOAuthKeys defaults omitted scopes/
        // validityPeriod to ['default']/3600 before calling generateToken, which
        // form-encodes them as scope=default&expiry_time=3600 in the upstream POST —
        // assert on what the mock endpoint actually received, since the response
        // alone would look identical whether or not the defaults were ever sent.
        const params = tokenServer.getLastRequestParams();
        expect(params.get('scope')).toBe('default');
        expect(params.get('expiry_time')).toBe('3600');
        expect(res.body.tokenScopes).toEqual(['default']);
        expect(res.body.validityTime).toBe(3600);
    });
});
