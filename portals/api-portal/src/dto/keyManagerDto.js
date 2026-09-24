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
const { applyAudit } = require('./dtoUtils');

/**
 * Project a stored provisioning record into its response shape.
 *
 * The DAO record carries internals a caller has no business seeing — the row's
 * own uuid, the key manager uuid, its audit columns — and names its credential
 * fields after the columns. This is the one place that mapping happens, so the
 * response cannot drift into exposing a column just because someone added one.
 *
 * Empty values are omitted rather than sent as "": absent means "not set", and
 * an empty string would render as a configured-but-blank field in a form.
 */
function toProvisioningView(cfg) {
    if (!cfg) return undefined;
    const view = {
        type: cfg.type,
        registrationEndpoint: cfg.registrationEndpoint,
        authMethod: cfg.authMethod,
        hasClientSecret: Boolean(cfg.hasClientSecret),
        hasPassword: Boolean(cfg.hasPassword),
        hasApiKey: Boolean(cfg.hasApiKey),
    };
    if (cfg.authorizeEndpoint) view.authorizeEndpoint = cfg.authorizeEndpoint;
    if (cfg.authClientId) view.clientId = cfg.authClientId;
    if (cfg.authUsername) view.username = cfg.authUsername;
    if (cfg.authScopes && cfg.authScopes.length) view.scopes = cfg.authScopes;
    if (cfg.authResource) view.resource = cfg.authResource;
    if (cfg.authHeaderName) view.headerName = cfg.authHeaderName;
    if (cfg.authScheme) view.scheme = cfg.authScheme;
    return view;
}

/**
 * DTO for key manager responses.
 * Never exposes admin credentials in API responses.
 *
 * A key manager reaches here from one of two places — a `key_managers` row or an
 * `[[api_portal.key_manager]]` config entry — and `source` says which. It is
 * what tells a client whether the entry can be edited through this API at all:
 * a config-declared one answers 409 to PUT and DELETE. Emitted only when the
 * caller tagged the record (keyManagerRegistry does, and so do the create/update
 * paths), so an untagged record still produces the pre-existing response shape.
 *
 * `orgId` and the audit fields are absent for a config entry: it is declared per
 * portal in a file, by no user, so there is nothing truthful to put there.
 */
class KeyManagerDTO {
    constructor(km, audit) {
        this.id = km.handle;
        this.displayName = km.display_name;
        if (km.org_uuid !== undefined) this.orgId = km.org_uuid;
        this.enabled = !!km.enabled;
        this.tokenEndpoint = km.token_endpoint;
        if (km.source) this.source = km.source;
        // Whether the portal can register OAuth applications here, as opposed to
        // merely proxying token requests for one created elsewhere. Always true
        // for a config entry; for a stored one it depends on whether someone has
        // supplied its provisioning configuration.
        if (km.canGenerateKeys !== undefined) this.canGenerateKeys = !!km.canGenerateKeys;
        /*
         * Which driver handles this key manager, and the fact the settings table
         * shows: it is fixed at creation and decides whether keys are registered
         * here or imported from elsewhere.
         *
         * Two sources because the type lives in two places. A config-declared
         * entry carries `driver_type` projected off the built instance; a stored
         * one keeps it in its provisioning row. Neither means there is no
         * provisioning row at all, which is exactly how the factory resolves a
         * key manager to the provision driver — so `provision` is the honest
         * answer, not a missing value.
         */
        this.type = km.driver_type || (km.provisioning && km.provisioning.type) || 'provision';
        // Credential-free by construction: the DAO's public read never decrypts,
        // so there is no secret in scope here to leak even by mistake.
        if (km.provisioning) this.provisioning = toProvisioningView(km.provisioning);
        applyAudit(this, audit);
    }
}

/**
 * Minimal DTO for developer-facing key manager listing.
 * Only includes information developers need when selecting a KM.
 */
class KeyManagerPublicDTO {
    constructor(km) {
        this.id = km.handle;
        this.displayName = km.display_name;
        this.tokenEndpoint = km.token_endpoint;
        if (km.source) this.source = km.source;
    }
}

module.exports = { KeyManagerDTO, KeyManagerPublicDTO, toProvisioningView };
