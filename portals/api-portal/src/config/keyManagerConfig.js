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
 * Validates and normalizes the operator-supplied key-manager config —
 * the [[api_portal.key_manager]] array — and builds one Authenticator per entry.
 *
 * Config shape (snake_case in TOML; configLoader camelCases every key before
 * this module sees it, so `registration_endpoint` arrives as
 * `registrationEndpoint`):
 *
 *   [[api_portal.key_manager]]
 *   id                    = "thunder-local"
 *   name                  = "ThunderID"
 *   type                  = "thunderid"
 *   registration_endpoint = "https://localhost:8090/oauth2/dcr/register"
 *   token_endpoint        = "https://localhost:8090/oauth2/token"
 *   authorize_endpoint    = "https://localhost:8090/oauth2/authorize"
 *   insecure_skip_verify  = true
 *
 *     [api_portal.key_manager.auth]
 *     method        = "client_credentials"
 *     client_id     = "my-system-app"
 *     client_secret = '{{ env "THUNDER_CLIENT_SECRET" }}'
 *
 * Secrets use the portal's own {{ env "NAME" }} interpolation (resolved by
 * configLoader before this runs, and fail-closed when the variable is unset),
 * not the ${NAME} form the standalone POC implemented — one interpolation
 * mechanism for the whole file rather than two with different failure modes.
 *
 * Deliberately free of any dependency on configLoader or logger: configLoader
 * requires this module during its own bootstrap, and requiring either back
 * would be a load cycle. Same constraint roleScopeMap.js documents. Everything
 * here therefore throws rather than logging, and the caller decides that a
 * throw aborts startup.
 *
 * Only I/O-free checks happen here; the async work (minting a provisioning
 * token, reading mTLS certificate files) is deferred to the factory's first
 * use, since this runs inside a synchronous bootstrap.
 */

const { registeredTypes, getDriver } = require('../keymanagers/core/registry');
const { ApiKey, BasicAuth, ClientCredentials, MutualTLS } = require('../keymanagers/core/authenticators');

const AUTH_METHODS = ['client_credentials', 'basic', 'mtls', 'api_key'];

// Flags that describe one key manager's host. Each belongs on the
// [[api_portal.key_manager]] entry, not in [api_portal.key_manager.auth] and not
// in the global [api_portal.key_manager_client] section.
const CLIENT_POLICY_KEYS = [
    { camel: 'insecureSkipVerify', toml: 'insecure_skip_verify' },
    { camel: 'allowPrivateEndpoints', toml: 'allow_private_endpoints' },
    { camel: 'allowHttpEndpoints', toml: 'allow_http_endpoints' },
];

// Same handle rule the rest of the portal uses for operator-chosen identifiers,
// so a key manager id is safe in a URL path segment and in a log line.
const ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

/**
 * Normalize and validate every configured key manager.
 *
 * @param {object} cfg  the resolved config tree (configLoader's `config`)
 * @returns {Array<object>} normalized instance configs, ready for buildFactory
 * @throws {Error} on any invalid entry — the caller aborts startup
 */
function validateKeyManagerConfig(cfg) {
    const raw = cfg.keyManager;
    if (raw === undefined || raw === null) {
        return [];
    }
    if (!Array.isArray(raw)) {
        throw new Error(
            'key_manager must be an array of tables — declare each key manager with ' +
            '[[api_portal.key_manager]] (double brackets), not [api_portal.key_manager]'
        );
    }

    // allow_private_endpoints / allow_http_endpoints used to live in the global
    // key_manager_client section. Left there they would be silently ignored,
    // quietly re-imposing the deny-by-default posture on a key manager the
    // operator believes they opted in — so say so instead.
    const globalClient = cfg.keyManagerClient || {};
    for (const { camel, toml } of CLIENT_POLICY_KEYS) {
        if (Object.prototype.hasOwnProperty.call(globalClient, camel)) {
            throw new Error(
                `key_manager_client."${toml}" has moved: set it on the individual ` +
                '[[api_portal.key_manager]] entry it applies to. It describes one key ' +
                "manager's host, so a global value would grant it to every key manager. " +
                'key_manager_client now carries only timeout_ms, max_request_bytes and ' +
                'max_response_bytes.'
            );
        }
    }

    const seenIds = new Set();
    return raw.map((entry, i) => {
        const instance = normalizeInstance(entry, i);
        if (seenIds.has(instance.id)) {
            throw new Error(
                `key_manager[${i}]: duplicate id "${instance.id}" — ids are how a caller ` +
                'selects a key manager, so they must be unique'
            );
        }
        seenIds.add(instance.id);
        return instance;
    });
}

function normalizeInstance(entry, i) {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) {
        throw new Error(`key_manager[${i}] is not a table`);
    }
    const where = entry.id ? `key_manager "${entry.id}"` : `key_manager[${i}]`;

    const need = (key, tomlName) => {
        const value = entry[key];
        if (value === undefined || value === null || value === '') {
            throw new Error(`${where} is missing required field "${tomlName}"`);
        }
        if (typeof value !== 'string') {
            throw new Error(`${where}: "${tomlName}" must be a string`);
        }
        return value;
    };

    const id = need('id', 'id');
    if (!ID_PATTERN.test(id)) {
        throw new Error(
            `${where}: "id" must start with a letter or digit and contain only ` +
            'letters, digits, dots, underscores or hyphens'
        );
    }

    const type = need('type', 'type');
    if (!getDriver(type)) {
        const known = registeredTypes();
        throw new Error(
            `${where}: unknown type "${type}". Built-in types: ` +
            `${known.length ? known.join(', ') : '(none registered)'}`
        );
    }

    // The three client-policy flags describe how to reach THIS key manager's
    // host, so they live on the key manager entry. Each is rejected rather than
    // ignored if it turns up in the wrong table: silently dropping a
    // security-relevant flag would leave an operator believing a restriction was
    // relaxed when it was not, or the reverse.
    for (const misplaced of CLIENT_POLICY_KEYS) {
        if (entry.auth && Object.prototype.hasOwnProperty.call(entry.auth, misplaced.camel)) {
            throw new Error(
                `${where}: "${misplaced.toml}" belongs directly under ` +
                '[[api_portal.key_manager]], not under [api_portal.key_manager.auth] — ' +
                'it applies to every call to this key manager, not just the token request'
            );
        }
    }
    const clientPolicy = {
        // Skips TLS verification for every call to this key manager.
        insecureSkipVerify: asBool(entry.insecureSkipVerify, false, where, 'insecure_skip_verify'),
        // Permits this key manager to sit on a private/loopback address. Off by
        // default, per key manager: a local Thunder opting in must not also
        // grant a public key manager configured alongside it the same reach.
        allowPrivateEndpoints: asBool(
            entry.allowPrivateEndpoints, false, where, 'allow_private_endpoints'
        ),
        // Permits plain http:// for this key manager. On by default — an identity
        // server commonly sits behind a TLS-terminating ingress.
        allowHttpEndpoints: asBool(
            entry.allowHttpEndpoints, true, where, 'allow_http_endpoints'
        ),
    };
    const tokenEndpoint = need('tokenEndpoint', 'token_endpoint');

    const authTable = entry.auth;
    if (!authTable || typeof authTable !== 'object' || Array.isArray(authTable)) {
        throw new Error(
            `${where} is missing its [api_portal.key_manager.auth] table — the portal needs ` +
            'a credential to register clients on this key manager'
        );
    }

    return {
        id,
        type,
        displayName: need('name', 'name'),
        description: typeof entry.description === 'string' ? entry.description : '',
        registrationEndpoint: need('registrationEndpoint', 'registration_endpoint'),
        tokenEndpoint,
        // Optional: a key manager issuing only client_credentials tokens has no
        // interactive authorization endpoint.
        authorizeEndpoint: typeof entry.authorizeEndpoint === 'string' ? entry.authorizeEndpoint : '',
        clientPolicy,
        auth: buildAuthenticator(authTable, where, clientPolicy, tokenEndpoint),
    };
}

function buildAuthenticator(auth, where, clientPolicy, kmTokenEndpoint) {
    const method = auth.method;
    const need = (key, tomlName) => {
        const value = auth[key];
        if (value === undefined || value === null || value === '') {
            throw new Error(`${where}: auth.method = "${method}" requires "auth.${tomlName}"`);
        }
        if (typeof value !== 'string') {
            throw new Error(`${where}: "auth.${tomlName}" must be a string`);
        }
        return value;
    };

    switch (method) {
        case 'client_credentials':
            return new ClientCredentials({
                // Defaults to the key manager's own token endpoint: the common
                // case is the same URL, and repeating it invites the two
                // drifting apart.
                tokenEndpoint: typeof auth.tokenEndpoint === 'string' && auth.tokenEndpoint
                    ? auth.tokenEndpoint
                    : kmTokenEndpoint,
                clientId: need('clientId', 'client_id'),
                clientSecret: need('clientSecret', 'client_secret'),
                scopes: asStringArray(auth.scopes, where, 'auth.scopes'),
                resource: typeof auth.resource === 'string' ? auth.resource : '',
                sendCredentialsInBody: asBool(
                    auth.sendCredentialsInBody, false, where, 'auth.send_credentials_in_body'
                ),
                clientPolicy,
            });
        case 'basic':
            return new BasicAuth({
                username: need('username', 'username'),
                password: need('password', 'password'),
                clientPolicy,
            });
        case 'api_key':
            return new ApiKey({
                // Defaults to Authorization, which is where most key managers
                // want it. The scheme is configured separately rather than typed
                // into the key, so rotating the key leaves the scheme alone.
                headerName: typeof auth.headerName === 'string' && auth.headerName
                    ? auth.headerName
                    : 'Authorization',
                scheme: typeof auth.scheme === 'string' ? auth.scheme : '',
                key: need('apiKey', 'api_key'),
                clientPolicy,
            });
        case 'mtls':
            return new MutualTLS({
                certFile: need('certFile', 'cert_file'),
                keyFile: need('keyFile', 'key_file'),
                caFile: typeof auth.caFile === 'string' ? auth.caFile : '',
                clientPolicy,
            });
        default:
            throw new Error(
                `${where}: unknown auth.method ${JSON.stringify(method)} ` +
                `(expected one of: ${AUTH_METHODS.join(', ')})`
            );
    }
}

function asBool(value, fallback, where, field) {
    if (value === undefined || value === null) return fallback;
    if (typeof value === 'boolean') return value;
    throw new Error(`${where}: "${field}" must be a boolean (true or false), got ${JSON.stringify(value)}`);
}

function asStringArray(value, where, field) {
    if (value === undefined || value === null) return [];
    if (!Array.isArray(value) || value.some((v) => typeof v !== 'string')) {
        throw new Error(`${where}: "${field}" must be an array of strings`);
    }
    return value;
}

// buildAuthenticator is exported for the database-backed path: a key manager
// created through the REST API has to produce the same Authenticator this
// builds from TOML, and duplicating the per-method construction would let the
// two drift.
module.exports = { validateKeyManagerConfig, buildAuthenticator, AUTH_METHODS, ID_PATTERN };
