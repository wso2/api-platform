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
 * WSO2 Identity Server and Asgardeo — Dynamic Client Registration via the DCR
 * v1.1 API both products expose.
 *
 * One driver, two registered types, because these are one API. Asgardeo is built
 * on Identity Server, and the two were verified alike on every axis this driver
 * depends on: the same `/api/identity/oauth2/dcr/v1.1/register` path, the same
 * response members, no registration access token from either, the same two
 * booleans reset by a partial update, and the same 401 for an unknown client id.
 * A second copy of this file would be a second place to fix each of those. If the
 * products diverge, split then — with a real difference to point at.
 *
 *   type = "wso2is"     WSO2 Identity Server 7.x, self-hosted
 *   type = "asgardeo"   Asgardeo, the hosted product
 *
 * Both take their endpoints from the deployment's OIDC discovery document, at
 * `https://<host>/oauth2/token/.well-known/openid-configuration`.
 *
 * What differs between the two is not the driver but the credential, which is
 * configuration: Asgardeo wants a machine-to-machine application's
 * `client_credentials` grant carrying the DCR management scopes, while a
 * self-hosted Identity Server commonly takes `basic` auth for an admin user. A
 * self-hosted instance usually also needs `allow_private_endpoints`, and with the
 * default self-signed certificate `insecure_skip_verify`.
 *
 * Two things make this different from thunderid.js, and they are the reason this
 * is a separate driver rather than a second config entry pointed at that one.
 *
 * 1. NO REGISTRATION ACCESS TOKEN. Asgardeo's DCR API is RFC 7591-shaped but
 *    does not implement RFC 7592: the registration response carries neither
 *    `registration_access_token` nor `registration_client_uri`. Read, update and
 *    delete are plain calls to `<registration_endpoint>/<client_id>`, all
 *    authenticated with the portal's own provisioning credential. That falls out
 *    for free — the shared `_call` only attaches a registration access token when
 *    one exists, so here the authenticator's Bearer is used throughout.
 *
 * 2. PUT PRESERVES EVERY OMITTED MEMBER EXCEPT THE BOOLEANS, WHICH RESET.
 *    RFC 7592 §2.2 makes update wholly destructive; Asgardeo is neither that nor
 *    a clean merge. Verified against the live API: a PUT carrying only
 *    `client_name` and `grant_types` left the lifetimes, `redirect_uris`,
 *    `token_type_extension` and the rest untouched — but silently flipped
 *    `ext_pkce_mandatory` and `ext_pkce_support_plain` from true back to false.
 *
 *    The cause is visible in the API's own model: its boolean members are Java
 *    primitives, so an absent one deserializes to `false`, while the boxed
 *    `Long`/`String`/`List` members arrive null and are read as "leave alone".
 *    An edit that only renamed the application would therefore turn PKCE
 *    enforcement off, with nothing in the request or the response saying so.
 *    `updateKey` below closes that by filling in any boolean the caller omitted
 *    from the client's current state.
 *
 * 3. A NARROWER, EXTENDED METADATA SET. Asgardeo accepts a subset of RFC 7591
 *    (no `scope`, no `response_types`, no `token_endpoint_auth_method`) plus its
 *    own `ext_*` members for things the RFC has no word for. Declaring RFC
 *    members Asgardeo ignores would be worse than omitting them: the form would
 *    collect values that silently do nothing.
 *
 * On Asgardeo the provisioning credential is a machine-to-machine application in
 * the same organisation, authorized for the DCR management scopes:
 *
 *   internal_dcr_create  internal_dcr_view  internal_dcr_update  internal_dcr_delete
 *
 * Without that authorization Asgardeo issues the token happily but drops every
 * scope, and the first DCR call comes back 403 — which this driver reports as
 * `provisioning_credential_rejected`.
 */

const { register } = require('../core/registry');
const { KeyManager, prop, opt, toDcrBody, toKey } = require('../core/keyManager');

/*
 * The registered names. An instance is told which one activated it, so metadata
 * and error detail name the type the operator actually configured rather than a
 * single hardcoded label that would be wrong half the time.
 */
const TYPE_WSO2IS = 'wso2is';
const TYPE_ASGARDEO = 'asgardeo';

/*
 * The members this API types as Java primitive booleans, which is why an update
 * that omits one silently sets it to false rather than leaving it alone. See the
 * note on updateKey.
 */
const BOOLEAN_MEMBERS = ['ext_pkce_mandatory', 'ext_pkce_support_plain', 'ext_public_client'];

class WSO2ISKeyManager extends KeyManager {
    constructor(cfg, authRequest, type) {
        super(cfg);
        this.type = type || TYPE_WSO2IS;
        this.authRequest = authRequest;
    }

    /*
     * Only members Asgardeo's DCR v1.1 API actually reads.
     *
     * This list drives the Add Key form, not what the API accepts — `toDcrBody`
     * passes the whole properties bag through, so an operator sending an `ext_*`
     * member not listed here still reaches Asgardeo. What the list decides is
     * what a developer is *offered*, and offering a field the key manager
     * discards is how a form comes to lie about what it configured.
     */
    metadata() {
        return this._metaEnvelope([
            prop('client_name', 'Application name', 'string',
                'Shown to end users on the consent screen.', true),
            // RFC 7591 §2 requires redirect_uris only for clients using a
            // redirect-based flow; a client_credentials-only client never
            // redirects, so this is conditional rather than required.
            prop('redirect_uris', 'Callback URLs', 'string_list',
                'Where the identity server sends the user after login. Absolute URIs, no fragment. '
                + 'Needed only for the authorization code grant.', false, null,
                { property: 'grant_types', anyOf: ['authorization_code'] }),
            prop('grant_types', 'Grant types', 'multiselect',
                'OAuth2 grants this application may use.', true, [
                    opt('authorization_code', 'Authorization code'),
                    opt('refresh_token', 'Refresh token'),
                    opt('client_credentials', 'Client credentials'),
                    opt('password', 'Password'),
                    opt('implicit', 'Implicit'),
                ]),
            prop('ext_public_client', 'Public client', 'boolean',
                'Register without a client secret. A public client cannot use the client '
                + 'credentials grant, so tokens for it cannot be generated from this portal.',
                false),
            prop('ext_pkce_mandatory', 'Require PKCE', 'boolean',
                'Rejects authorization code requests that carry no code challenge.', false),
            prop('ext_pkce_support_plain', 'Allow plain PKCE', 'boolean',
                'Permits the "plain" code challenge method as well as S256. S256 only is stronger.',
                false),
            prop('token_type_extension', 'Access token type', 'select',
                'The token format the identity server issues to this application.', false, [
                    opt('JWT', 'JWT (self-contained)'),
                    opt('Default', 'Opaque'),
                ]),
            prop('ext_application_token_lifetime', 'Application token lifetime', 'number',
                'Seconds an application (client credentials) access token stays valid.', false),
            prop('ext_user_token_lifetime', 'User token lifetime', 'number',
                'Seconds a user access token stays valid.', false),
            prop('ext_refresh_token_lifetime', 'Refresh token lifetime', 'number',
                'Seconds a refresh token stays valid.', false),
            prop('ext_id_token_lifetime', 'ID token lifetime', 'number',
                'Seconds an ID token stays valid.', false),
        ]);
    }

    async createKey(properties) {
        const dcr = await this._call('POST', this.registrationEndpoint, toDcrBody(properties), [200, 201]);
        return toKey(this, properties, dcr || {});
    }

    async getKey(consumerKey) {
        const dcr = await this._call('GET', this._clientUrl(consumerKey), null, [200]);
        return toKey(this, this._propertiesFromDcr(dcr || {}), dcr || {});
    }

    /**
     * Update, with the omitted booleans filled in first.
     *
     * A caller that sends only the members it wants to change is behaving
     * perfectly reasonably, and for every other member Asgardeo agrees. For the
     * booleans it does not: an omitted one is taken as `false`. Reading the
     * client back and restoring only those keeps a partial update from turning
     * off a security setting nobody touched.
     *
     * The read is skipped when the caller already supplied all of them, which is
     * the normal case from the portal's own form — a checkbox always has a state,
     * so the form never omits one.
     */
    async updateKey(consumerKey, properties) {
        const body = toDcrBody(properties);
        const missing = BOOLEAN_MEMBERS.filter((k) => body[k] === undefined);
        if (missing.length) {
            const current = (await this._call('GET', this._clientUrl(consumerKey), null, [200])) || {};
            missing.forEach((k) => {
                if (current[k] !== undefined) body[k] = current[k];
            });
        }
        const dcr = await this._call('PUT', this._clientUrl(consumerKey), body, [200]);
        return toKey(this, properties, dcr || {});
    }

    async deleteKey(consumerKey) {
        await this._call('DELETE', this._clientUrl(consumerKey), null, [200, 202, 204]);
    }

    /**
     * The client configuration endpoint.
     *
     * Always constructed, never read from the registration response: RFC 7592's
     * `registration_client_uri` is what a driver would normally prefer here, and
     * this API does not return one.
     */
    _clientUrl(consumerKey) {
        return `${this.registrationEndpoint}/${encodeURIComponent(consumerKey)}`;
    }

    /**
     * The DCR API answers an unknown client id with 401, not 404.
     *
     * Verified against the live API: the same provisioning token that had just
     * created and deleted a client returns 401 for a client id that never
     * existed. Left to the base reading, a developer opening a key whose client
     * was removed at the identity server would be told the portal's provisioning
     * credential was rejected — sending whoever reads that log to re-check a
     * credential that is fine.
     *
     * Only for a call addressing one client. At the registration collection a 401
     * really is about the credential, so that keeps the base meaning.
     */
    _classify(status, perClient) {
        if (status === 401 && perClient) {
            return 'client_not_found';
        }
        return super._classify(status, perClient);
    }

    /**
     * Turn a DCR read response back into the `properties` bag the API defines.
     *
     * Two response-only members to drop beyond the RFC's own: `id`, Asgardeo's
     * internal application identifier, and `client_secret_expires_at`.
     */
    _propertiesFromDcr(dcr) {
        const {
            id: _id,
            client_id: _clientId,
            client_secret: _clientSecret,
            client_id_issued_at: _issuedAt,
            client_secret_expires_at: _expiresAt,
            registration_client_uri: _clientUri,
            registration_access_token: _accessToken,
            ...properties
        } = dcr;

        /*
         * This API has no `token_endpoint_auth_method` — it says the same thing
         * with `ext_public_client`. Translating it here is what lets the rest of
         * the portal stay in RFC vocabulary: token generation already refuses a
         * client registered with `none` and explains why, instead of sending a
         * secret that does not exist and reporting the resulting 401 as a wrong
         * credential.
         */
        if (properties.ext_public_client === true) {
            properties.token_endpoint_auth_method = 'none';
        }
        return properties;
    }
}

register(TYPE_WSO2IS, (cfg, authRequest) => new WSO2ISKeyManager(cfg, authRequest, TYPE_WSO2IS), 'WSO2 Identity Server');
register(TYPE_ASGARDEO, (cfg, authRequest) => new WSO2ISKeyManager(cfg, authRequest, TYPE_ASGARDEO), 'Asgardeo');

module.exports = { WSO2ISKeyManager, TYPE_WSO2IS, TYPE_ASGARDEO };
