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

// Minimal OAuth2 client-credentials token endpoint used by the application
// key/token round-trip test. A key manager's tokenEndpoint is pointed here, and
// oauthTokenService (in the devportal container) POSTs to it server-side with
// HTTP Basic auth (clientId:clientSecret). Mirrors the in-process approach the
// REST suite uses in it/rest-api/key-managers/token-generation.spec.js.

const http = require('node:http');
const os = require('node:os');

const MOCK_TOKEN_PORT = 4599;
const MOCK_TOKEN_SECRET = 'it-mock-consumer-secret';
const MOCK_ACCESS_TOKEN = 'it-mock-access-token';
// A client-credentials token request body is tiny; cap it so a runaway/hostile
// caller can't force the mock to buffer an unbounded stream in memory.
const MAX_TOKEN_BODY_BYTES = 8 * 1024;

// This process's docker-network IPv4, so the devportal container (which makes
// the token request server-side) can reach the mock. Using the address rather
// than a service-name alias keeps it working under both `compose up` and the
// one-off `compose run` used while iterating.
function containerIpv4() {
    for (const list of Object.values(os.networkInterfaces())) {
        for (const iface of list || []) {
            if (iface.family === 'IPv4' && !iface.internal) {
                return iface.address;
            }
        }
    }
    return '127.0.0.1';
}

// Singleton so the cy.task start/stop pair is idempotent — only the tests that
// need a token endpoint start it (via cy.task), and it is stopped afterwards, so
// it never listens during unrelated specs.
let server = null;
let meta = null;

// Starts the mock token server if it isn't already running and resolves with its
// { endpoint, secret, accessToken }. Resolves only once the socket is listening,
// so a spec can seed a key manager against a reachable endpoint immediately.
function startMockTokenServer() {
    if (server) {
        // Already running — meta is the live endpoint's metadata, not stale.
        return Promise.resolve(meta);
    }
    return new Promise((resolve, reject) => {
        let listening = false;
        const srv = http.createServer((req, res) => {
            if (req.method !== 'POST') {
                res.writeHead(405);
                return res.end();
            }
            let body = '';
            let aborted = false;
            req.on('data', (chunk) => {
                if (aborted) {
                    return;
                }
                body += chunk;
                if (body.length > MAX_TOKEN_BODY_BYTES) {
                    // Stop accumulating and reject with a generic 413 rather than
                    // buffering an unbounded body.
                    aborted = true;
                    res.writeHead(413, { 'Content-Type': 'application/json' });
                    res.end(JSON.stringify({ error: 'payload_too_large' }));
                    req.destroy();
                }
            });
            req.on('end', () => {
                if (aborted) {
                    return;
                }
                const [, encoded] = (req.headers.authorization || '').split(' ');
                const [, secret] = Buffer.from(encoded || '', 'base64').toString('utf8').split(':');
                if (secret !== MOCK_TOKEN_SECRET) {
                    res.writeHead(401, { 'Content-Type': 'application/json' });
                    return res.end(JSON.stringify({ error: 'invalid_client' }));
                }
                const params = new URLSearchParams(body);
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({
                    access_token: MOCK_ACCESS_TOKEN,
                    token_type: 'Bearer',
                    expires_in: Number(params.get('expiry_time')) || 3600,
                    scope: params.get('scope') || '',
                }));
            });
        });
        srv.on('error', (err) => {
            console.error('[mock-token-server] server error:', err.message);
            // Only a pre-listen failure (e.g. EADDRINUSE) means startup failed —
            // reject so the caller sees it instead of receiving stale/null metadata.
            if (!listening) {
                server = null;
                meta = null;
                reject(err);
            }
        });
        srv.listen(MOCK_TOKEN_PORT, '0.0.0.0', () => {
            listening = true;
            server = srv;
            meta = {
                endpoint: `http://${containerIpv4()}:${MOCK_TOKEN_PORT}/token`,
                secret: MOCK_TOKEN_SECRET,
                accessToken: MOCK_ACCESS_TOKEN,
            };
            resolve(meta);
        });
    });
}

function stopMockTokenServer() {
    if (!server) {
        return Promise.resolve(null);
    }
    const srv = server;
    // Resolve only once the socket has actually closed, and clear state after
    // shutdown completes so callers can await full termination.
    return new Promise((resolve) => {
        srv.close(() => {
            server = null;
            meta = null;
            resolve(null);
        });
    });
}

module.exports = { startMockTokenServer, stopMockTokenServer };
