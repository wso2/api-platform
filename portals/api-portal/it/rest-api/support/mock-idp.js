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

// A minimal OIDC identity provider for the multi-tenancy specs: an authorize endpoint that
// logs in whoever the spec queued with nextLogin(), a token endpoint (authorization
// code + PKCE), and JWKS endpoints — one for the root issuer and one per organization,
// so tokens can come from a single issuer or, like WSO2 IS sub-organizations, from a
// per-organization issuer ".../o/<org>/oauth2/token" with its own signing key.
//
// Served over HTTPS with the certificate `make ensure-certs` issues for this container
// (the portal trusts it via NODE_EXTRA_CA_CERTS): organization provisioning refuses an
// IDP it reaches over plain http to a non-loopback host, and this is how the specs
// reach it for real rather than around that guard.
//
// JWTs are built with node:crypto directly so the suite needs no JOSE dependency.

const https = require('https');
const crypto = require('crypto');
const fs = require('fs');

function b64url(input) {
    return Buffer.from(input).toString('base64url');
}

function createMockIdp({ baseUrl, clientId, certFile, keyFile }) {
    const keys = new Map(); // 'root' | org -> { privateKey, jwk }
    const codes = new Map();
    let queuedLogin = null;
    let lastAuthorize = null;
    let server;

    function keyFor(name) {
        if (!keys.has(name)) {
            const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 });
            const kid = `${name}-key`;
            keys.set(name, { privateKey, jwk: { ...publicKey.export({ format: 'jwk' }), kid, alg: 'RS256', use: 'sig' } });
        }
        return keys.get(name);
    }

    function sign(claims, keyName) {
        const { privateKey, jwk } = keyFor(keyName);
        const header = b64url(JSON.stringify({ alg: 'RS256', typ: 'JWT', kid: jwk.kid }));
        const payload = b64url(JSON.stringify(claims));
        const signature = crypto.sign('RSA-SHA256', Buffer.from(`${header}.${payload}`), privateKey).toString('base64url');
        return `${header}.${payload}.${signature}`;
    }

    /**
     * Mints an ID token and access token for `login`:
     *   { sub, orgId, orgName, orgHandle, roles, perOrgIssuer, orgClaim, omitOrg, aud }
     * orgId is also the per-organization issuer segment; orgClaim, when given,
     * replaces the org_id claim's value verbatim (e.g. a list, or a mismatch).
     */
    function mint(login) {
        const perOrg = !!login.perOrgIssuer;
        const iss = perOrg ? `${baseUrl}/o/${login.orgId}/oauth2/token` : `${baseUrl}/oauth2/token`;
        const keyName = perOrg ? `org:${login.orgId}` : 'root';
        const now = Math.floor(Date.now() / 1000);
        // omitOrg drops org_id/org_name; an orgHandle given with it still goes in, for a
        // token that names its organization only by handle.
        let org = login.omitOrg && login.orgHandle ? { org_handle: login.orgHandle } : {};
        if (!login.omitOrg) {
            org = login.orgClaim !== undefined
                ? { org_id: login.orgClaim }
                : { org_id: login.orgId, ...(login.orgName && { org_name: login.orgName }), ...(login.orgHandle && { org_handle: login.orgHandle }) };
        }
        const base = { iss, aud: login.aud || clientId, sub: login.sub, iat: now, exp: now + 3600, roles: login.roles || [], ...org };
        return {
            id_token: sign({ ...base, given_name: login.sub, email: `${login.sub}@example.com` }, keyName),
            access_token: sign({ ...base, client_id: clientId, scope: 'openid profile email' }, keyName),
        };
    }

    function readBody(req) {
        return new Promise((resolve) => {
            let body = '';
            req.on('data', (c) => { body += c; });
            req.on('end', () => resolve(body));
        });
    }

    function json(res, status, obj) {
        res.writeHead(status, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(obj));
    }

    async function handle(req, res) {
        const url = new URL(req.url, baseUrl);
        if (url.pathname === '/oauth2/authorize') {
            lastAuthorize = Object.fromEntries(url.searchParams);
            const redirect = new URL(url.searchParams.get('redirect_uri'));
            redirect.searchParams.set('state', url.searchParams.get('state'));
            // prompt=none (silent SSO) succeeds only for an existing IDP session — the
            // queued login — and, like WSO2 IS, only in the organization an `org` hint
            // names. Otherwise it answers login_required instead of showing a form.
            const hint = url.searchParams.get('org');
            if (url.searchParams.get('prompt') === 'none'
                && (!queuedLogin || (hint && hint !== queuedLogin.orgId))) {
                redirect.searchParams.set('error', 'login_required');
                res.writeHead(302, { Location: redirect.toString() });
                return res.end();
            }
            const code = crypto.randomBytes(16).toString('hex');
            codes.set(code, { login: queuedLogin, challenge: url.searchParams.get('code_challenge') });
            redirect.searchParams.set('code', code);
            res.writeHead(302, { Location: redirect.toString() });
            return res.end();
        }
        if (url.pathname === '/oauth2/token' && req.method === 'POST') {
            const form = new URLSearchParams(await readBody(req));
            const entry = codes.get(form.get('code'));
            codes.delete(form.get('code'));
            if (!entry || !entry.login) return json(res, 400, { error: 'invalid_grant' });
            const verifier = form.get('code_verifier') || '';
            if (entry.challenge && crypto.createHash('sha256').update(verifier).digest('base64url') !== entry.challenge) {
                return json(res, 400, { error: 'invalid_grant', error_description: 'PKCE verification failed' });
            }
            return json(res, 200, { ...mint(entry.login), token_type: 'Bearer', expires_in: 3600 });
        }
        if (url.pathname === '/oauth2/jwks') return json(res, 200, { keys: [keyFor('root').jwk] });
        const perOrgJwks = /^\/o\/([^/]+)\/oauth2\/jwks$/.exec(url.pathname);
        if (perOrgJwks) return json(res, 200, { keys: [keyFor(`org:${decodeURIComponent(perOrgJwks[1])}`).jwk] });
        return json(res, 404, { error: 'not_found' });
    }

    function start(port) {
        return new Promise((resolve, reject) => {
            server = https.createServer(
                { cert: fs.readFileSync(certFile), key: fs.readFileSync(keyFile) },
                (req, res) => handle(req, res).catch((err) => json(res, 500, { error: err.message }))
            );
            server.on('error', reject);
            server.listen(port, () => resolve());
        });
    }

    function stop() {
        return new Promise((resolve) => (server ? server.close(() => resolve()) : resolve()));
    }

    return {
        start,
        stop,
        mint,
        /**
         * Sets the identity /oauth2/authorize logs in — the IDP-side session. null means
         * no session: interactive logins then fail, and prompt=none answers login_required.
         */
        nextLogin(login) { queuedLogin = login; },
        /** The query of the most recent /oauth2/authorize call. */
        lastAuthorize() { return lastAuthorize; },
    };
}

module.exports = { createMockIdp };
