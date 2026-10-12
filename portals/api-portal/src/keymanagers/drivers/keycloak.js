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

/*
 * NOT REGISTERED IN THIS BUILD. ./index.js deliberately does not require this
 * module, so `type = "keycloak"` is refused at startup and the type is absent
 * from the key manager form. The code is kept because it was written and
 * verified against a live Keycloak 26.7.4; re-enable it by restoring the
 * require line in ./index.js.
 *
 * Nothing else gates it — a driver is only ever reached through the registry,
 * and the registry is filled by that require.
 */



'use strict';

/*
 * Keycloak — RFC 7591 Dynamic Client Registration with RFC 7592 read, update and
 * delete, against its client-registration endpoint.
 *
 * Activated by `type = "keycloak"`. Endpoints come from the realm's OIDC
 * discovery document:
 *
 *   https://<host>/realms/<realm>/.well-known/openid-configuration
 *   registration_endpoint ->
 *     https://<host>/realms/<realm>/clients-registrations/openid-connect
 *
 * Every behaviour below was measured against Keycloak 26.7.4, not assumed.
 *
 * 1. IT ISSUES A REGISTRATION ACCESS TOKEN, AND ROTATES IT ON UPDATE. The
 *    registration response carries `registration_access_token` and
 *    `registration_client_uri`. A read returns the SAME token; an update returns a
 *    NEW one and the previous token is refused from that moment — a GET with it
 *    answers 401 immediately after. So the portal must write back the token from
 *    every response, which `oauth2KeyService._withRegistration` does. This is the
 *    key manager that makes that machinery necessary rather than theoretical.
 *
 * 2. ITS CONFIGURATION URI IS NOT DERIVABLE. The client id is a UUID and the
 *    configuration endpoint is `<registration_endpoint>/<client_id>` — the same
 *    shape, but the portal has no business assuming that, which is why RFC 7592 §3
 *    makes the URI REQUIRED in the response and why `_clientUrl` prefers it.
 *
 * 3. THE PROVISIONING CREDENTIAL ALSO WORKS THERE. An admin bearer is accepted at
 *    the configuration endpoint, so a key whose stored token was lost or rotated
 *    away is recoverable rather than stranded — which is what makes the
 *    clear-and-retry fallback worth having.
 *
 * 4. AN UNKNOWN CLIENT IS A CLEAN 404, unlike the WSO2 DCR API's 401. The base
 *    `_classify` already reads that correctly, so there is no override here.
 *
 * Standalone by construction: it imports nothing from another driver. The shared
 * code is `_call`, `_classify` and `_parse` on the `KeyManager` base — the HTTP
 * plumbing every driver here uses.
 */

const { register } = require('../core/registry');
const {
    KeyManager, prop, opt, toDcrBody, toKey,
} = require('../core/keyManager');

const TYPE = 'keycloak';

class KeycloakKeyManager extends KeyManager {
    constructor(cfg, authRequest) {
        super(cfg);
        this.type = TYPE;
        this.authRequest = authRequest;
    }

    /*
     * RFC 7591 client metadata. Keycloak reads the standard members, so this is
     * the standard set rather than a vendor one — the difference from custom.js is
     * the documentation above, not the fields.
     */
    metadata() {
        return this._metaEnvelope([
            prop('client_name', 'Application name', 'string',
                'Shown to end users on the consent screen.', true),
            prop('redirect_uris', 'Callback URLs', 'string_list',
                'Where Keycloak sends the user after login. Absolute URIs, no fragment. '
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
                'Scopes this application may request.', false),
            prop('token_endpoint_auth_method', 'Client authentication method', 'select',
                'How the application authenticates at the token endpoint.', false, [
                    opt('client_secret_basic', 'Client secret (Basic header)'),
                    opt('client_secret_post', 'Client secret (POST body)'),
                    opt('client_secret_jwt', 'Client secret JWT'),
                    opt('private_key_jwt', 'Private key JWT'),
                    opt('none', 'Public client (no secret)'),
                ]),
            prop('jwks_uri', 'JWKS URL', 'uri',
                "Where Keycloak fetches this application's public keys. Needed for "
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

    /**
     * Update, carrying the client's own id.
     *
     * RFC 7592 §2.2 requires `client_id` in the request, and Keycloak's own PUT
     * expects it. Taken from the key being updated rather than from `properties`,
     * so a caller cannot move a client onto another id by putting one in the bag.
     */
    async updateKey(consumerKey, properties, registration) {
        const body = { ...toDcrBody(properties), client_id: consumerKey };
        const dcr = await this._call(
            'PUT', this._clientUrl(consumerKey, registration), body, [200], registration
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
     * The server-supplied `registration_client_uri` is authoritative; the
     * constructed form is the fallback for a call made before one was stored, or
     * after a rejected token was cleared.
     */
    _clientUrl(consumerKey, registration) {
        if (registration && registration.clientUri) {
            return registration.clientUri;
        }
        return `${this.registrationEndpoint}/${encodeURIComponent(consumerKey)}`;
    }

    /** Strip the response-only members, leaving the `properties` bag the API defines. */
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
        if (typeof properties.scope === 'string') {
            properties.scope = properties.scope.split(/\s+/).filter(Boolean);
        }
        return properties;
    }
}

register(TYPE, (cfg, authRequest) => new KeycloakKeyManager(cfg, authRequest), 'Keycloak');

module.exports = { KeycloakKeyManager, TYPE };
