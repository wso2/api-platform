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
 * KeyManager base class and the helpers every driver shares.
 *
 * `properties` throughout is the open bag from the REST API — RFC 7591 client
 * metadata plus key-manager-specific keys, exactly as
 * OAuth2KeyProperties defines it in docs/api-portal-openapi-spec-v0.9.yaml.
 * Those keys stay snake_case because they are RFC 7591 wire names handed to the
 * key manager verbatim; the envelope around them is camelCase like the rest of
 * this API.
 *
 * A driver implements only the operations it can actually perform. The base
 * class implements none of them: each unimplemented method rejects with
 * `unsupported_operation`, which the REST layer answers as 409. Capability is
 * therefore whatever the driver actually overrode — there is nothing to declare,
 * and so nothing that can claim an operation it cannot do.
 */

/**
 * Raised by a driver when the key manager rejected or failed a call.
 *
 * `publicReason` is a coarse, non-identifying classification the REST layer maps
 * to a status code. The upstream URL, status line and response body live in
 * `detail`, which is for the internal log only — per error-handling.md, none of
 * it may reach the client, since it names internal hosts and can echo back
 * whatever the key manager chose to include.
 */
class KeyManagerCallError extends Error {
    constructor(publicReason, detail, upstreamStatus) {
        super(publicReason);
        this.name = 'KeyManagerCallError';
        this.publicReason = publicReason;
        this.detail = detail;
        this.upstreamStatus = upstreamStatus;
    }
}

/** @abstract Every built-in driver extends this. */
class KeyManager {
    constructor(cfg) {
        this.id = cfg.id;
        this.displayName = cfg.displayName;
        this.description = cfg.description;
        this.tokenEndpoint = cfg.tokenEndpoint;
        this.authorizeEndpoint = cfg.authorizeEndpoint;
        this.registrationEndpoint = cfg.registrationEndpoint;
        /*
         * What creating a key here actually does, as the caller experiences it.
         *
         *   register  the portal creates the application and returns credentials
         *             the developer has never seen
         *   provide   the application already exists; the developer supplies its
         *             client id and no secret is issued
         *
         * Declared rather than inferred. A UI could branch on `type === 'provision'`,
         * but that hardcodes one driver's name into every client — and a second
         * provision-shaped driver added later would be silently misclassified. This
         * is the fact a caller actually needs; `type` is an implementation detail.
         */
        this.keyCreation = 'register';
    }

    /** @returns {object} one KeyManagerMetadataResponseSchema entry */
    metadata() {
        throw new Error(`${this.constructor.name}.metadata() not implemented`);
    }

    /*
     * A driver overrides only what its key manager can do. Anything left to the
     * base class rejects with `unsupported_operation`, which the REST layer maps
     * to 409 — so a caller reaching for an operation this key manager does not
     * offer gets a clean refusal rather than a 500, and no separate declaration
     * has to be kept in step with the implementation.
     */
    _unsupported(operation) {
        return new KeyManagerCallError(
            'unsupported_operation',
            `${this.type || this.constructor.name} does not implement ${operation}`,
            null
        );
    }

    async createKey(_properties) {
        throw this._unsupported('createKey');
    }
    async getKey(_consumerKey, _registration) {
        throw this._unsupported('getKey');
    }
    async updateKey(_consumerKey, _properties, _registration) {
        throw this._unsupported('updateKey');
    }
    async deleteKey(_consumerKey, _registration) {
        throw this._unsupported('deleteKey');
    }

    /**
     * Exchange this key's own credentials for an access token, via the
     * `client_credentials` grant at the key manager's token endpoint.
     *
     * Implemented once here rather than per driver: this is plain RFC 6749 §4.4
     * against a URL every key manager already declares, so there is nothing
     * vendor-specific to vary. A driver may still override it.
     *
     * The credential is the DEVELOPER'S — their consumer key and the secret they
     * supplied — not the portal's provisioning credential. Supplying an explicit
     * Authorization header is what selects it: the authenticator's requester
     * spreads a caller's headers last and does not mint its own token when one is
     * already present (the same mechanism RFC 7592 calls rely on). The request
     * still goes through the guarded client, so the address check, the redirect
     * ban and the size ceilings all apply.
     *
     * @param {string} consumerKey
     * @param {string} consumerSecret  used for this one request; never stored
     * @param {object} [opts]
     * @param {string[]} [opts.scopes]
     * @param {number} [opts.validityPeriod]
     * @param {string} [opts.authMethod] the client's own `token_endpoint_auth_method`.
     *        Defaults to client_secret_basic, which is what a server assumes when a
     *        registration does not say otherwise.
     * @returns {Promise<{accessToken: string, tokenType: string, expiresIn: number, scope: string}>}
     */
    async requestToken(consumerKey, consumerSecret, { scopes = [], validityPeriod, authMethod } = {}) {
        if (!this.tokenEndpoint) {
            throw this._unsupported('requestToken');
        }

        /*
         * Where the client's credential goes is the client's own choice, declared
         * when it was registered. Sending it the wrong way is refused with a plain
         * 401, indistinguishable from a wrong secret — so guessing is worse than
         * knowing, and the caller reads the method back before calling here.
         *
         * Two of the four registered methods cannot work through this endpoint at
         * all, and both are refused before anything is dialled:
         *
         *   none            a public client has no secret to present, and RFC 6749
         *                   §4.4 restricts client_credentials to confidential
         *                   clients. Its tokens come from an interactive login.
         *   private_key_jwt needs the client's private key. The portal holds no
         *                   client secrets by design, and a private key pasted
         *                   through a browser form is worse than the secret it
         *                   replaces.
         */
        const method = authMethod || 'client_secret_basic';
        if (method === 'none') {
            throw new KeyManagerCallError(
                'public_client_no_credentials',
                `client "${consumerKey}" is registered with token_endpoint_auth_method=none, so it has `
                + 'no secret and cannot use the client credentials grant',
                null
            );
        }
        if (method !== 'client_secret_basic' && method !== 'client_secret_post') {
            throw new KeyManagerCallError(
                'client_auth_method_unsupported',
                `client "${consumerKey}" authenticates with "${method}", which the portal cannot present `
                + 'on the developer\'s behalf',
                null
            );
        }

        const form = new URLSearchParams();
        form.set('grant_type', 'client_credentials');
        if (scopes.length) form.set('scope', scopes.join(' '));
        // A hint the authorization server is free to ignore; the response carries
        // the lifetime that actually applies.
        if (validityPeriod) form.set('expiry_time', String(validityPeriod));

        const headers = {
            'Content-Type': 'application/x-www-form-urlencoded',
            Accept: 'application/json',
        };
        if (method === 'client_secret_post') {
            form.set('client_id', consumerKey);
            form.set('client_secret', consumerSecret);
        } else {
            // client_secret_basic — preferred where the client allows it, because the
            // secret stays out of the body and so out of any access log on the way.
            headers.Authorization = 'Basic '
                + Buffer.from(`${consumerKey}:${consumerSecret}`).toString('base64');
        }

        let res;
        try {
            res = await this.authRequest(this.tokenEndpoint, {
                method: 'POST',
                headers,
                data: form.toString(),
                // This request carries the DEVELOPER's credential, never the
                // portal's — whichever of the two ways it travels. Without this the
                // authenticator would add its provisioning token to a
                // client_secret_post request, which sets no Authorization header of
                // its own, and the key manager would reject it as invalid_client.
                anonymous: true,
            });
        } catch (err) {
            throw new KeyManagerCallError(
                'unreachable',
                `token endpoint ${this.tokenEndpoint}: ${err.message}`,
                null
            );
        }

        if (res.status !== 200) {
            // The secret is redacted from the logged body: some servers echo a
            // submitted parameter back in an error.
            const body = describeUpstreamBody(res.data, [consumerSecret]);
            throw new KeyManagerCallError(
                // 400/401 here is the developer's own credential or scope being
                // refused — their request, their fix. Anything else is ours.
                res.status === 400 || res.status === 401
                    ? 'token_request_rejected'
                    : 'upstream_error',
                `token endpoint ${this.tokenEndpoint} returned HTTP ${res.status} for ` +
                `client_id "${consumerKey}"` + (body ? `: ${body}` : ' (no response body)'),
                res.status
            );
        }

        const data = res.data || {};
        if (!data.access_token) {
            throw new KeyManagerCallError(
                'upstream_error',
                `token endpoint ${this.tokenEndpoint} returned HTTP 200 with no access_token: ` +
                (describeUpstreamBody(res.data, [consumerSecret]) || '(empty body)'),
                res.status
            );
        }
        return {
            accessToken: data.access_token,
            tokenType: data.token_type || 'Bearer',
            expiresIn: Number(data.expires_in) > 0 ? Number(data.expires_in) : 0,
            scope: data.scope || '',
        };
    }

    /**
     * Assemble the metadata envelope; a driver supplies only its property
     * descriptors. Field names here are the spec's
     * (KeyManagerMetadataResponseSchema), so the REST layer can return this
     * as-is with no translation step to drift out of sync.
     */
    _metaEnvelope(properties) {
        const meta = {
            id: this.id,
            displayName: this.displayName,
            type: this.type,
            keyCreation: this.keyCreation,
            tokenEndpoint: this.tokenEndpoint,
            properties,
        };
        // Both are optional in the schema — omitted rather than sent empty, so a
        // key manager with no interactive grant simply has no authorizeEndpoint.
        if (this.description) meta.description = this.description;
        if (this.authorizeEndpoint) meta.authorizeEndpoint = this.authorizeEndpoint;
        return meta;
    }

    /**
     * Issue one authenticated call to the key manager and classify the outcome.
     *
     * Shared by every DCR-speaking driver, because the interesting differences
     * between key managers are in the URLs and the body shape, not in what an
     * HTTP 409 means. A driver overrides `_classify` if its key manager uses a
     * status code to mean something other than the reading below.
     *
     * `registration` carries the RFC 7592 registration access token when the key
     * manager issued one at registration; it then authenticates read/update/delete
     * for that one client, overriding the authenticator's own Authorization
     * header. A key manager that issues no such token simply never supplies it,
     * and the portal's provisioning credential is used throughout.
     */
    async _call(method, url, bodyObj, okStatuses, registration) {
        const headers = { Accept: 'application/json' };
        let data;
        if (bodyObj) {
            headers['Content-Type'] = 'application/json';
            data = JSON.stringify(bodyObj);
        }
        if (registration && registration.accessToken) {
            headers.Authorization = `Bearer ${registration.accessToken}`;
        }

        let res;
        try {
            res = await this.authRequest(url, { method, headers, data });
        } catch (err) {
            // An authenticator already classified its own failure (a rejected
            // provisioning credential, say) — re-throw it rather than relabelling
            // everything that comes out of authRequest as a connectivity problem.
            if (err instanceof KeyManagerCallError) {
                throw err;
            }
            // Transport-level: DNS failure, refused connection, TLS mismatch, or
            // the SSRF guard refusing the resolved address at dial time.
            throw new KeyManagerCallError(
                'unreachable',
                `${this.type} ${method} ${url}: ${err.message}`,
                null
            );
        }

        if (!okStatuses.includes(res.status)) {
            /*
             * Whether this call addressed one client or the collection. Some key
             * managers answer an unknown client id with a status that means
             * something quite different at the collection URL, so the reading of
             * a status can depend on which of the two was called.
             */
            const perClient = url !== this.registrationEndpoint;
            throw new KeyManagerCallError(
                this._classify(res.status, perClient),
                // Kept for the internal log only — never returned to a client.
                `${this.type} ${method} ${url}: HTTP ${res.status}: ` +
                (describeUpstreamBody(res.data, [registration && registration.accessToken]) ||
                    '(no response body)'),
                res.status
            );
        }
        return this._parse(res.data);
    }

    /**
     * @param {number} status        the upstream HTTP status
     * @param {boolean} [perClient]  true when the call addressed a single client
     *                               (`.../register/<client_id>`) rather than the
     *                               registration collection. A driver overrides
     *                               this where its key manager departs from the
     *                               reading below.
     */
    _classify(status, perClient) {
        if (status === 400 || status === 422) return 'rejected';
        if (status === 401 || status === 403) return 'provisioning_credential_rejected';
        if (status === 404) return 'client_not_found';
        if (status === 409) return 'conflict';
        if (status === 429) return 'rate_limited';
        return 'upstream_error';
    }

    _parse(data) {
        if (data === undefined || data === null || data === '') return null;
        if (typeof data === 'object') return data; // axios already parsed JSON
        try {
            return JSON.parse(data);
        } catch {
            throw new KeyManagerCallError(
                'upstream_error',
                `${this.type}: response body was not valid JSON`,
                null
            );
        }
    }
}

/* ---- upstream error body, for the internal log ---- */

// A key manager's error body is the most useful thing in a failure log: it is
// where the actual diagnosis lives (RFC 6749 `error`/`error_description`, or
// whatever non-standard shape a given server uses). Logged whole rather than
// field-allowlisted, because which field carries the answer differs per vendor
// and a narrower filter silently drops it.
const MAX_LOGGED_BODY_CHARS = 2048;

// Below this length a "secret" is too likely to occur incidentally in ordinary
// text, where redacting it would mangle the message it is meant to protect.
const MIN_REDACTABLE_SECRET_CHARS = 6;

/**
 * Render an upstream response body for the internal log: credentials we know we
 * sent are redacted, then the result is bounded.
 *
 * Redaction rather than omission is the point. The hazard is narrow — some
 * servers echo a submitted parameter back in an error — so neutralise that one
 * thing instead of discarding the whole body and with it the diagnosis
 * (error-handling.md directive 5: don't put the raw secret in the standard log;
 * it says nothing about dropping the rest of the message).
 *
 * Never reaches an API response: the service layer answers every server-side
 * cause with one generic message.
 *
 * @param {*} data                 the parsed or raw response body
 * @param {string[]} [secrets]     credential values to redact if present
 * @returns {string} '' when there is no body
 */
function describeUpstreamBody(data, secrets = []) {
    if (data === undefined || data === null || data === '') {
        return '';
    }
    let text;
    try {
        text = typeof data === 'string' ? data : JSON.stringify(data);
    } catch {
        return '[unserializable response body]';
    }
    if (typeof text !== 'string' || text === '') {
        return '';
    }
    for (const secret of secrets) {
        if (typeof secret !== 'string' || secret.length < MIN_REDACTABLE_SECRET_CHARS) continue;
        // split/join, not a regex: a secret may contain regex metacharacters.
        text = text.split(secret).join('[REDACTED]');
    }
    return text.length > MAX_LOGGED_BODY_CHARS
        ? `${text.slice(0, MAX_LOGGED_BODY_CHARS)}...[truncated]`
        : text;
}

/* ---- metadata property descriptor helpers ---- */

/**
 * One property descriptor. `description` and `options` are omitted when absent,
 * matching KeyManagerPropertyDescriptor's optional fields.
 */
function prop(name, label, type, description, required, options, requiredWhen) {
    const p = { name, label, type, required: Boolean(required) };
    if (description) p.description = description;
    if (options) p.options = options;
    // A property whose necessity depends on another choice sets this instead of
    // `required`, so a renderer can ask for it only when it is genuinely needed.
    // The two are mutually exclusive by the schema; `required` wins if both were
    // somehow set, which is the stricter reading.
    if (requiredWhen && !p.required) p.requiredWhen = requiredWhen;
    return p;
}

const opt = (value, label) => ({ value, label });

/* ---- DCR mapping helpers ---- */

/**
 * Turn the API `properties` bag into an RFC 7591 registration body.
 *
 * The only reshaping is `scope`: the API takes an array (easier to validate and
 * render), RFC 7591 wants the space-delimited string.
 */
function toDcrBody(properties) {
    const body = { ...properties };
    if (Array.isArray(body.scope)) {
        body.scope = body.scope.join(' ');
    }
    return body;
}

/**
 * Map a DCR registration response onto the fields the service layer persists.
 *
 * `keyId` is assigned above this layer — a key manager knows nothing about this
 * portal's own identifiers.
 *
 * registrationClientUri/registrationAccessToken are the RFC 7592 follow-up
 * credentials for later read/update/delete of this client. The access token is a
 * bearer credential: it must never be logged or returned to a client.
 */
function toKey(km, properties, dcr) {
    return {
        keyManagerId: km.id,
        keyManagerName: km.displayName,
        consumerKey: dcr.client_id || '',
        consumerSecret: dcr.client_secret || '',
        properties,
        registration: {
            clientUri: dcr.registration_client_uri || '',
            accessToken: dcr.registration_access_token || '',
        },
    };
}

module.exports = {
    KeyManager,
    KeyManagerCallError,
    describeUpstreamBody,
    prop,
    opt,
    toDcrBody,
    toKey,
};
