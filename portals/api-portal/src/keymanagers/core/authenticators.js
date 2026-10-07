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
 * How the portal authenticates itself TO a key manager when registering clients
 * on it. One mechanism -> one authenticated request(url, opts) function.
 *
 * This is the portal acting as a privileged provisioning client, distinct from
 * the end-user token in the inbound request: registering an OAuth application
 * over DCR needs the key manager's own admin/system credential.
 *
 * Vendor quirks live as config on the authenticator (RFC 8707 `resource`,
 * client_secret_post vs client_secret_basic, dev TLS skip), never in the shared
 * request path.
 *
 * No credential is ever logged, here or by callers: a client secret, basic
 * password, or minted access token in a log line becomes an attack surface the
 * moment log storage is read (js-authentication-authorization.md JS-AUTH-003).
 */

const { readFile } = require('node:fs/promises');

const { buildClient } = require('./httpClient');
const { KeyManagerCallError, describeUpstreamBody } = require('./keyManager');

/*
 * A header name must be an RFC 7230 token, and no header value may carry CR or
 * LF. Both are enforced where the credential is constructed rather than where it
 * is configured, so every entry point — TOML at startup and the REST API at
 * runtime — passes the same check (GO-AUTH-015, applied to this subsystem).
 *
 * The deny-list is narrower than it looks: these four headers do not carry a
 * credential, they decide where the request goes and how its body is framed.
 * Letting an admin-configured value set `Host` would let a key manager be
 * reached at one address while presenting as another, which is precisely what
 * the address guard exists to prevent.
 */
const HEADER_NAME_TOKEN = /^[A-Za-z0-9!#$%&'*+.^_`|~-]+$/;
const RESERVED_HEADERS = new Set(['host', 'content-length', 'transfer-encoding', 'connection']);

function assertUsableHeader(name, value, where) {
    if (typeof name !== 'string' || !HEADER_NAME_TOKEN.test(name)) {
        throw new Error(`${where}: header name must be a single HTTP token, e.g. "Authorization" or "X-API-Key"`);
    }
    if (RESERVED_HEADERS.has(name.toLowerCase())) {
        throw new Error(`${where}: "${name}" cannot be set as a credential header`);
    }
    if (typeof value !== 'string' || value === '') {
        throw new Error(`${where}: the API key is required`);
    }
    if (/[\r\n]/.test(value)) {
        throw new Error(`${where}: the API key cannot contain a line break`);
    }
}

/*
 * Merge a default credential header with whatever the caller supplied, letting
 * the caller win on a case-insensitive name match.
 *
 * The case-insensitivity is the point. A driver overrides `Authorization` to
 * present an RFC 7592 registration access token; if the configured header were
 * spelled `authorization`, a plain object spread would keep both keys and the
 * two would race. Matched properly, the caller's header replaces ours — and a
 * genuinely different header, `X-API-Key` alongside `Authorization`, is left
 * alone, which is also correct.
 */
function withDefaultHeader(name, value, callerHeaders = {}) {
    const lower = name.toLowerCase();
    const overridden = Object.keys(callerHeaders).some((key) => key.toLowerCase() === lower);
    return overridden ? { ...callerHeaders } : { [name]: value, ...callerHeaders };
}

/*
 * Whether this request carries its own credential and must not be given the
 * portal's.
 *
 * The usual signal is a caller-supplied Authorization header, which every
 * authenticator already steps aside for. That is not enough on its own: a token
 * request using client_secret_post puts the developer's credential in the BODY
 * and sets no header at all, so without this flag the authenticator would see an
 * unauthenticated request and attach the portal's provisioning token — leaving a
 * Bearer where the key manager expects client authentication, and a request that
 * fails as invalid_client no matter how good the developer's secret is.
 *
 * The guarded HTTP client is untouched by this. Only the credential is withheld.
 */
function carriesOwnCredential(opts) {
    if (opts && opts.anonymous) return true;
    const headers = (opts && opts.headers) || {};
    return Object.keys(headers).some((h) => h.toLowerCase() === 'authorization');
}

/** @abstract */
class Authenticator {
    /** @returns {Promise<(url: string, opts?: object) => Promise<object>>} */
    async requester() {
        throw new Error(`${this.constructor.name}.requester() not implemented`);
    }
}

/**
 * HTTP Basic against the key manager's registration endpoint.
 */
class BasicAuth extends Authenticator {
    constructor({ username, password, clientPolicy = {} }) {
        super();
        this.username = username;
        this.password = password;
        // How to reach this key manager's host — TLS trust, address range and
        // scheme. Carried whole rather than unpacked, so adding a policy field
        // does not mean touching every authenticator.
        this.clientPolicy = clientPolicy;
    }

    async requester() {
        const header = 'Basic ' + Buffer.from(`${this.username}:${this.password}`).toString('base64');
        const client = buildClient(this.clientPolicy);
        return (url, opts = {}) => {
            if (carriesOwnCredential(opts)) {
                return client.request({ url, ...opts });
            }
            return client.request({
                url,
                ...opts,
                headers: { Authorization: header, ...opts.headers },
            });
        };
    }
}

/**
 * client_credentials against the key manager's token endpoint, with the minted
 * access token cached until shortly before it expires.
 */
class ClientCredentials extends Authenticator {
    constructor({
        tokenEndpoint,
        clientId,
        clientSecret,
        scopes = [],
        resource = '',
        sendCredentialsInBody = false,
        clientPolicy = {},
    }) {
        super();
        this.tokenEndpoint = tokenEndpoint;
        this.clientId = clientId;
        this.clientSecret = clientSecret;
        this.scopes = scopes;
        this.resource = resource;
        this.sendCredentialsInBody = sendCredentialsInBody;
        this.clientPolicy = clientPolicy;

        // One client for both the token fetch and the DCR calls, so keep-alive
        // and the TLS posture are shared.
        this.client = null;
        this._token = null;
        this._expiryMs = 0;
        this._inflight = null;
    }

    _clientOrBuild() {
        if (!this.client) {
            this.client = buildClient(this.clientPolicy);
        }
        return this.client;
    }

    async _fetchToken() {
        const form = new URLSearchParams();
        form.set('grant_type', 'client_credentials');
        if (this.scopes.length) form.set('scope', this.scopes.join(' '));
        // RFC 8707 resource indicator — some key managers require it to issue a
        // token accepted by their registration endpoint.
        if (this.resource) form.set('resource', this.resource);

        const headers = { 'Content-Type': 'application/x-www-form-urlencoded' };
        const requestOpts = {
            url: this.tokenEndpoint,
            method: 'POST',
            headers,
            data: form.toString(),
        };
        if (this.sendCredentialsInBody) {
            // client_secret_post
            form.set('client_id', this.clientId);
            form.set('client_secret', this.clientSecret);
            requestOpts.data = form.toString();
        } else {
            // client_secret_basic (the default). axios's `auth` option does the
            // base64 itself and keeps the secret out of the URL.
            requestOpts.auth = { username: this.clientId, password: this.clientSecret };
        }

        let res;
        try {
            res = await this._clientOrBuild().request(requestOpts);
        } catch (err) {
            // Transport-level: the token endpoint itself is unreachable.
            throw new KeyManagerCallError(
                'unreachable',
                `token endpoint ${this.tokenEndpoint}: ${err.message}`,
                null
            );
        }
        if (res.status !== 200) {
            // Classified rather than thrown as a bare Error: a driver wraps any
            // plain throw from here as 'unreachable', which would report a
            // rejected credential as a connectivity problem and send whoever is
            // reading the log looking for the wrong thing.
            // The key manager's own body is logged in full (bounded, with our
            // client secret redacted if it was echoed back). It is where the
            // actual cause is: "invalid_target / no resource parameter supplied"
            // points at RFC 8707 `resource`, "invalid_scope" at `scopes`,
            // "invalid_client" at the credential — three different fixes that a
            // status code alone cannot distinguish.
            const body = describeUpstreamBody(res.data, [this.clientSecret]);
            throw new KeyManagerCallError(
                res.status === 400 || res.status === 401 || res.status === 403
                    ? 'provisioning_credential_rejected'
                    : 'upstream_error',
                `token endpoint ${this.tokenEndpoint} returned HTTP ${res.status} for ` +
                `client_id "${this.clientId}"` +
                (body ? `: ${body}` : ' (no response body)'),
                res.status
            );
        }
        const body = res.data || {};
        if (!body.access_token) {
            throw new KeyManagerCallError(
                'upstream_error',
                `token endpoint ${this.tokenEndpoint} returned HTTP 200 with no access_token: ` +
                (describeUpstreamBody(res.data, [this.clientSecret]) || '(empty body)'),
                res.status
            );
        }

        this._token = body.access_token;
        // Refresh 30s early so an in-flight DCR call can't be the one that
        // discovers the token just expired.
        const ttlSec = Number(body.expires_in) > 0 ? Number(body.expires_in) : 300;
        this._expiryMs = Date.now() + Math.max(ttlSec - 30, 1) * 1000;
        return this._token;
    }

    async _accessToken() {
        if (this._token && Date.now() < this._expiryMs) return this._token;
        // Collapse concurrent misses into one token request rather than
        // stampeding the key manager when several keys are generated at once.
        if (!this._inflight) {
            this._inflight = this._fetchToken().finally(() => {
                this._inflight = null;
            });
        }
        return this._inflight;
    }

    /**
     * The authenticated request function handed to a driver.
     *
     * A driver may override the Authorization header for a specific call, and
     * when it does, this must not clobber it: RFC 7592 says the client
     * configuration endpoint (read/update/delete of one registered client) is
     * authenticated with the registration access token issued for that client,
     * NOT with the portal's provisioning credential. So opts.headers is spread
     * last, and the provisioning token is not even minted when the caller has
     * already supplied one.
     */
    async requester() {
        const client = this._clientOrBuild();
        return async (url, opts = {}) => {
            if (carriesOwnCredential(opts)) {
                // No provisioning token is minted either — there is nothing here for
                // it to authenticate, and requesting one would be a wasted round trip.
                return client.request({ url, ...opts });
            }
            const token = await this._accessToken();
            return client.request({
                url,
                ...opts,
                headers: { Authorization: `Bearer ${token}`, ...opts.headers },
            });
        };
    }
}

/**
 * Mutual TLS: the portal presents a client certificate to the key manager.
 */
class MutualTLS extends Authenticator {
    constructor({ certFile, keyFile, caFile, clientPolicy = {} }) {
        super();
        this.certFile = certFile;
        this.keyFile = keyFile;
        this.caFile = caFile;
        this.clientPolicy = clientPolicy;
    }

    async requester() {
        const [cert, key, ca] = await Promise.all([
            readFile(this.certFile),
            readFile(this.keyFile),
            this.caFile ? readFile(this.caFile) : Promise.resolve(undefined),
        ]);
        // The cert material is handed to buildClient rather than assembled here,
        // so this path keeps the same SSRF-guarded lookup, pooling and TLS
        // tuning as every other key-manager client.
        const client = buildClient({ ...this.clientPolicy, clientCert: { cert, key, ca } });
        return (url, opts = {}) => client.request({ url, ...opts });
    }
}

/**
 * A fixed API key in a named header — `Authorization: <scheme> <key>`, or a bare
 * `X-API-Key: <key>`.
 *
 * The simplest mechanism here and the only one with no token endpoint behind it:
 * nothing is minted, cached or refreshed, because the credential IS the header.
 * That also makes it the one authenticator whose credential never expires on its
 * own, so revoking it is an action someone has to take at the key manager.
 *
 * `scheme` is separate from the key so the stored credential stays the secret
 * alone — a scheme word is not a secret, and keeping it out of the encrypted
 * column means a key can be rotated without re-typing the scheme.
 */
class ApiKey extends Authenticator {
    constructor({ headerName = 'Authorization', scheme = '', key, clientPolicy = {} }) {
        super();
        assertUsableHeader(headerName, key, 'key manager API key authentication');
        this.headerName = headerName;
        this.scheme = typeof scheme === 'string' ? scheme.trim() : '';
        this.key = key;
        this.clientPolicy = clientPolicy;
    }

    async requester() {
        const value = this.scheme ? `${this.scheme} ${this.key}` : this.key;
        const client = buildClient(this.clientPolicy);
        return (url, opts = {}) => {
            if (opts.anonymous) {
                return client.request({ url, ...opts });
            }
            return client.request({
                url,
                ...opts,
                headers: withDefaultHeader(this.headerName, value, opts.headers),
            });
        };
    }
}

module.exports = { Authenticator, ApiKey, BasicAuth, ClientCredentials, MutualTLS };
