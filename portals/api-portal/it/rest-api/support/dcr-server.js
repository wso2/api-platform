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
 * A minimal Dynamic Client Registration server, for pointing a key manager at.
 *
 * The portal's whole purpose here is to register OAuth2 clients on an identity
 * server over RFC 7591 and manage them afterwards over RFC 7592. Testing that
 * against a live Asgardeo or WSO2 IS tenant would make the suite depend on an
 * external account; testing it against a stubbed driver would prove only that
 * the stub was called. So this implements the protocol instead — the portal
 * makes real HTTP requests and gets real responses, and the assertions can be
 * about what it actually sent.
 *
 * Same shape and reasoning as support/webhook-sink.js: a tiny real server the
 * portal reaches by container name on the compose network.
 *
 * Conforming behaviour worth knowing, because tests assert on it:
 *  - registration returns `registration_access_token` and `registration_client_uri`
 *    (RFC 7592 §3), so the portal stores them and authenticates later calls with
 *    the token rather than its own provisioning credential;
 *  - read/update/delete require that token and answer 401 without it;
 *  - the configuration endpoint lives on a DIFFERENT path than registration
 *    (`/clients/{id}`, not `/register/{id}`), which is what makes
 *    `registration_client_uri` load-bearing — a portal that reconstructed the URL
 *    from the registration endpoint would miss it, as Keycloak's layout does.
 */

const http = require('http');

// The credential a key manager is configured to provision with. Exported so a
// spec configures the key manager with the same pair this server will accept.
const PROVISIONING = Object.freeze({ clientId: 'portal', clientSecret: 'portal-secret' });

const REGISTER_PATH = '/register';
const CLIENT_PATH = '/clients';
const TOKEN_PATH = '/token';

function createDcrServer() {
    let server;
    const clients = new Map();          // client_id -> { metadata, token }
    const requests = [];                // every request, for assertions
    let seq = 0;

    const json = (res, status, body) => {
        res.writeHead(status, { 'Content-Type': 'application/json' });
        res.end(body === undefined ? '' : JSON.stringify(body));
    };

    const bearer = (req) => {
        const auth = req.headers.authorization || '';
        const [scheme, value] = auth.split(' ');
        return (scheme || '').toLowerCase() === 'bearer' ? value : '';
    };

    /*
     * DCR bodies are JSON; a token request is form-encoded
     * (application/x-www-form-urlencoded), which is what KeyManager.requestToken
     * sends. Parsing by content type rather than assuming JSON -- an assumption
     * that made this server answer 500 to every token request.
     */
    function parseBody(req, body) {
        if (!body) return null;
        const type = (req.headers['content-type'] || '').toLowerCase();
        if (type.includes('json')) return JSON.parse(body);
        if (type.includes('x-www-form-urlencoded')) return Object.fromEntries(new URLSearchParams(body));
        try { return JSON.parse(body); } catch { return { raw: body }; }
    }

    function handle(req, res, body) {
        const url = new URL(req.url, 'http://mock');
        const parsed = parseBody(req, body);
        requests.push({ method: req.method, path: url.pathname, body: parsed, auth: req.headers.authorization || '' });

        // --- RFC 7591 registration ------------------------------------------
        if (req.method === 'POST' && url.pathname === REGISTER_PATH) {
            const id = `mock-client-${++seq}`;
            const token = `rat-${id}`;
            clients.set(id, { metadata: parsed || {}, token });
            return json(res, 201, {
                ...(parsed || {}),
                client_id: id,
                client_secret: `secret-${id}`,
                registration_access_token: token,
                registration_client_uri: `${baseUrl()}${CLIENT_PATH}/${id}`,
            });
        }

        // --- RFC 7592 client configuration ----------------------------------
        if (url.pathname.startsWith(`${CLIENT_PATH}/`)) {
            const id = decodeURIComponent(url.pathname.slice(CLIENT_PATH.length + 1));
            const held = clients.get(id);
            if (!held) return json(res, 404, { error: 'invalid_client' });
            if (bearer(req) !== held.token) return json(res, 401, { error: 'invalid_token' });

            if (req.method === 'GET') {
                return json(res, 200, { ...held.metadata, client_id: id });
            }
            if (req.method === 'PUT') {
                held.metadata = { ...(parsed || {}) };
                return json(res, 200, { ...held.metadata, client_id: id });
            }
            if (req.method === 'DELETE') {
                clients.delete(id);
                res.writeHead(204);
                return res.end();
            }
        }

        /*
         * A token endpoint, so one key manager serves both roles.
         *
         * The secret is actually checked, because that is what makes the failure
         * path testable: the portal maps an upstream 401 here to
         * `token_request_rejected`, which the API answers 400. Accepting any
         * secret would make the happy path pass and leave that mapping unproven.
         *
         * Both RFC 6749 placements are honoured — client_secret_basic in the
         * Authorization header, client_secret_post in the body — since which one
         * the portal uses is driven by the client's own registered
         * token_endpoint_auth_method.
         */
        if (req.method === 'POST' && url.pathname === TOKEN_PATH) {
            const form = parsed || {};
            let clientId = form.client_id;
            let secret = form.client_secret;
            const basic = (req.headers.authorization || '').split(' ');
            if ((basic[0] || '').toLowerCase() === 'basic') {
                const [u, p] = Buffer.from(basic[1] || '', 'base64').toString('utf8').split(':');
                clientId = clientId || u;
                secret = secret || p;
            }
            /*
             * Two different callers reach this endpoint, and conflating them was
             * a real mistake while writing these tests:
             *
             *   the PORTAL, minting its own provisioning token with the
             *   client_credentials credential a key manager is configured with —
             *   that client was never registered here, so checking it against an
             *   issued secret rejected the portal from its own key manager;
             *
             *   a DEVELOPER, exchanging the secret of a client this server
             *   actually issued.
             *
             * Both are accepted, each against the credential it legitimately holds.
             */
            const isProvisioning = clientId === PROVISIONING.clientId
                && secret === PROVISIONING.clientSecret;
            const held = clients.get(clientId);
            if (!isProvisioning && (!held || secret !== `secret-${clientId}`)) {
                return json(res, 401, { error: 'invalid_client' });
            }
            return json(res, 200, {
                access_token: 'mock-access-token',
                token_type: 'Bearer',
                expires_in: 3600,
                scope: form.scope || '',
            });
        }

        return json(res, 404, { error: 'not_found' });
    }

    let port;
    const baseUrl = () => `${process.env.MOCK_DCR_ENDPOINT_URL || `http://localhost:${port}`}`;

    function start(listenPort) {
        return new Promise((resolve, reject) => {
            server = http.createServer((req, res) => {
                let raw = '';
                req.on('data', (c) => { raw += c; });
                req.on('end', () => {
                    try {
                        handle(req, res, raw || null);
                    } catch (err) {
                        json(res, 500, { error: 'mock_failure', detail: String(err && err.message) });
                    }
                });
            });
            server.on('error', reject);
            server.listen(listenPort, () => { port = server.address().port; resolve(port); });
        });
    }

    function stop() {
        return new Promise((resolve) => (server ? server.close(resolve) : resolve()));
    }

    return {
        start,
        stop,
        /** Every request the portal made, in order. */
        requests: () => requests.slice(),
        /** The requests for one method, e.g. 'POST'. */
        requestsOf: (method) => requests.filter((r) => r.method === method),
        /** Clients the server currently holds — what "registered" means here. */
        clientIds: () => Array.from(clients.keys()),
        registrationEndpoint: () => `${baseUrl()}${REGISTER_PATH}`,
        tokenEndpoint: () => `${baseUrl()}${TOKEN_PATH}`,
    };
}

module.exports = { createDcrServer, PROVISIONING };
