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
 * A generic Dynamic Client Registration key manager — RFC 7591 registration with
 * RFC 7592 read, update and delete.
 *
 * Activated by `type = "custom"`. This is the driver to point at a key manager
 * that implements the specs and has no driver of its own here: it assumes nothing
 * beyond them, sends no vendor member, and declares the whole RFC 7591 §2 client
 * metadata set so the form can offer whatever that key manager actually supports.
 *
 * How it differs from the named drivers:
 *
 *   thunderid   the same protocol, but a deliberately short property list tuned to
 *               one key manager. Use that one for Thunder; use this where the
 *               fuller metadata set is wanted.
 *   wso2is      WSO2's DCR v1.1 API, which is RFC 7591-shaped but issues no
 *               registration access token and resets booleans on a partial update.
 *   asgardeo    the same, for the hosted product.
 *   provision   no registration at all.
 *
 * Standalone by construction: it imports nothing from another driver, so a change
 * made for one key manager cannot move this one, and vice versa. The only shared
 * code is `_call`, `_classify` and `_parse` on the `KeyManager` base — the HTTP
 * plumbing every driver here uses.
 *
 * Two things a key manager must do for this driver to work fully:
 *
 * 1. RETURN A REGISTRATION ACCESS TOKEN. RFC 7592 §3 authenticates read, update
 *    and delete of a client with the `registration_access_token` issued alongside
 *    it, at the `registration_client_uri` from the same response. Where a key
 *    manager returns neither, those three calls fall back to the portal's own
 *    provisioning credential against `<registration_endpoint>/<client_id>`, which
 *    is what most non-conforming implementations expect anyway — so this degrades
 *    rather than breaking.
 *
 * 2. TREAT PUT AS A REPLACEMENT. RFC 7592 §2.2 makes update wholly destructive:
 *    the request carries the client's complete metadata and any member left out is
 *    deleted. The portal's update path reads the client back and shows every
 *    current value in the form for exactly this reason. A key manager that instead
 *    merges, or that silently resets some members, needs its own driver rather than
 *    this one — see the note on wso2is.js for what that looks like in practice.
 */

const { register } = require('../core/registry');
const {
    KeyManager, prop, opt, toDcrBody, toKey,
} = require('../core/keyManager');

const TYPE = 'custom';

class CustomDcrKeyManager extends KeyManager {
    constructor(cfg, authRequest) {
        super(cfg);
        this.type = TYPE;
        this.authRequest = authRequest;
    }

    /*
     * The RFC 7591 §2 client metadata set, in full.
     *
     * Everything past `client_name` is optional, because the RFC makes it optional
     * and this driver cannot know which members a given key manager reads. A key
     * manager that ignores one simply ignores it; one that rejects an unknown
     * member answers 400, which surfaces as `rejected` with its own message in the
     * internal log.
     *
     * `jwks` is deliberately absent even though the RFC defines it: it is an inline
     * JWK Set, which is a JSON document rather than a form field. `jwks_uri` is the
     * member to use, and the two are mutually exclusive under §2 in any case.
     */
    metadata() {
        return this._metaEnvelope([
            prop('client_name', 'Application name', 'string',
                'Shown to end users on the consent screen.', true),
            // RFC 7591 §2 requires redirect_uris only for "clients using flows with
            // redirection". A client_credentials client never redirects, so demanding
            // one from it is wrong — hence the conditional rule rather than required.
            prop('redirect_uris', 'Callback URLs', 'string_list',
                'Where the key manager sends the user after login. Absolute URIs, no fragment. '
                + 'Needed only for the authorization code grant.', false, null,
                { property: 'grant_types', anyOf: ['authorization_code'] }),
            prop('grant_types', 'Grant types', 'multiselect',
                'OAuth2 grants this application may use.', true, [
                    opt('authorization_code', 'Authorization code'),
                    opt('refresh_token', 'Refresh token'),
                    opt('client_credentials', 'Client credentials'),
                    opt('implicit', 'Implicit'),
                    opt('password', 'Resource owner password'),
                ]),
            prop('response_types', 'Response types', 'multiselect',
                'Authorization endpoint response types. Must match the selected grants.', false, [
                    opt('code', 'code'),
                    opt('token', 'token'),
                    opt('id_token', 'id_token'),
                ]),
            prop('scope', 'Scopes', 'string_list',
                'Scopes this application may request. Sent space-delimited, as RFC 7591 defines it.',
                false),
            prop('token_endpoint_auth_method', 'Client authentication method', 'select',
                'How the application authenticates at the token endpoint.', false, [
                    opt('client_secret_basic', 'Client secret (Basic header)'),
                    opt('client_secret_post', 'Client secret (POST body)'),
                    opt('private_key_jwt', 'Private key JWT'),
                    opt('none', 'Public client (no secret)'),
                ]),
            prop('jwks_uri', 'JWKS URL', 'uri',
                "Where the key manager fetches this application's public keys. Needed for "
                + 'private_key_jwt.', false,
                null, { property: 'token_endpoint_auth_method', anyOf: ['private_key_jwt'] }),
            prop('contacts', 'Contact emails', 'string_list',
                'People responsible for this application.', false),
            prop('client_uri', 'Application home page', 'uri',
                'Public home page of the application.', false),
            prop('logo_uri', 'Logo URL', 'uri',
                'Logo shown on the consent screen.', false),
            prop('tos_uri', 'Terms of service URL', 'uri',
                'Link shown on the consent screen.', false),
            prop('policy_uri', 'Privacy policy URL', 'uri',
                'Link shown on the consent screen.', false),
            prop('software_id', 'Software ID', 'string',
                'Identifies this piece of software across all its installations. Unchanged between '
                + 'versions, unlike the client id, which is per registration.', false),
            prop('software_version', 'Software version', 'string',
                'Version of the software identified by Software ID.', false),
        ]);
    }

    async createKey(properties) {
        const dcr = await this._call('POST', this.registrationEndpoint, toDcrBody(properties), [200, 201]);
        return toKey(this, properties, dcr || {});
    }

    async getKey(consumerKey, registration) {
        const dcr = await this._call(
            'GET', this._clientUrl(consumerKey, registration), null, [200], registration
        );
        return toKey(this, this._propertiesFromDcr(dcr || {}), dcr || {});
    }

    async updateKey(consumerKey, properties, registration) {
        const dcr = await this._call(
            'PUT', this._clientUrl(consumerKey, registration), toDcrBody(properties), [200], registration
        );
        return toKey(this, properties, dcr || {});
    }

    async deleteKey(consumerKey, registration) {
        await this._call(
            'DELETE', this._clientUrl(consumerKey, registration), null, [200, 202, 204], registration
        );
    }

    /**
     * The RFC 7592 client configuration endpoint.
     *
     * The registration response's own `registration_client_uri` is authoritative
     * when present — RFC 7592 §3 says to use it rather than construct one, because
     * it may sit on a different host or path than the registration endpoint. The
     * `<registration_endpoint>/<client_id>` fallback covers a key manager that
     * omits it.
     */
    _clientUrl(consumerKey, registration) {
        if (registration && registration.clientUri) {
            return registration.clientUri;
        }
        return `${this.registrationEndpoint}/${encodeURIComponent(consumerKey)}`;
    }

    /**
     * A GET returns the client's current metadata; strip the response-only members
     * so what is left is the `properties` bag the API defines.
     */
    _propertiesFromDcr(dcr) {
        const {
            client_id: _clientId,
            client_secret: _clientSecret,
            client_id_issued_at: _issuedAt,
            client_secret_expires_at: _expiresAt,
            registration_client_uri: _clientUri,
            registration_access_token: _accessToken,
            ...properties
        } = dcr;
        // RFC 7591 carries scope as a space-delimited string; the API takes an
        // array, so convert back on the way out.
        if (typeof properties.scope === 'string') {
            properties.scope = properties.scope.split(/\s+/).filter(Boolean);
        }
        return properties;
    }
}

register(TYPE, (cfg, authRequest) => new CustomDcrKeyManager(cfg, authRequest), 'Custom');

module.exports = { CustomDcrKeyManager, TYPE };
