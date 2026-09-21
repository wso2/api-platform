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
 * ThunderID identity server — RFC 7591 Dynamic Client Registration, with
 * RFC 7592 read/update/delete of an already-registered client.
 *
 * Activated by `type = "thunderid"` in an [[api_portal.key_manager]] entry.
 */

const { register } = require('../core/registry');
const {
    KeyManager, prop, opt, toDcrBody, toKey,
} = require('../core/keyManager');

class ThunderIdKeyManager extends KeyManager {
    constructor(cfg, authRequest) {
        super(cfg);
        this.type = 'thunderid';
        this.authRequest = authRequest;
    }

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
                    opt('none', 'Public client (no secret)'),
                ]),
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
     * when present — RFC 7592 §3 says to use it rather than construct one. The
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
     * A GET returns the client's current metadata; strip the response-only
     * members so what's left is the `properties` bag the API defines.
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

register('thunderid', (cfg, authRequest) => new ThunderIdKeyManager(cfg, authRequest), 'ThunderID');

module.exports = { ThunderIdKeyManager };
