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
 * Role-to-scope grant table (auth.authorization.role_to_scope_mapping).
 *
 * The JS counterpart of platform-api's internal/middleware/role_scope_map.go.
 * One file maps a role name to the scopes that role grants; the roles claim of an
 * incoming token is expanded through it on every request when
 * auth.authorization.mode = "role".
 *
 * Why this exists: the portal's REST surface (/api/v0.9) is guarded by fine-grained
 * dp:* scopes, but an external OIDC IDP has no reason to mint them — it emits the
 * roles or groups its own estate is organized around. Without a grant table, the only
 * way an IDP-mode caller could reach the REST API was to bypass the per-operation
 * scope check entirely (authMiddleware.js's `preauthorized` session fast-path). Role
 * mode closes that gap: the IDP keeps emitting roles, and the portal decides what each
 * role may do.
 *
 * Kept free of any dependency on configLoader so configLoader can require it during
 * its own startup validation without a cycle. State lives here (the loaded map) but
 * the trigger is configLoader's fail-closed startup check — nothing reads an
 * unvalidated map, and expandRoles() before init() returns nothing rather than
 * silently granting.
 */

const fs = require('fs');
const path = require('path');
const yaml = require('../utils/yaml');

// This portal's own scope namespace — the scopes it mints AND enforces, so an
// unknown one is a configuration error rather than something to pass through.
const OWN_SCOPE_PREFIX = 'dp:';

// A grant table is a few hundred lines of YAML; the cap guards against pointing
// the setting at something enormous by mistake. Mirrors configLoader's MAX_FILE_BYTES.
const MAX_MAPPING_BYTES = 1 << 20; // 1 MiB

// Well-formedness for a scope in ANOTHER component's namespace (ap:*, or a future
// one). Segments may contain hyphens — a foreign namespace picks its own convention —
// and `*` is accepted only as a whole trailing segment, never as a free-floating
// character inside one. Matches platform-api's isWellFormedScope.
const SCOPE_SEGMENT_RE = /^[a-z0-9][a-z0-9_-]*$/;

/**
 * Reads the mapping file with the same file-access discipline platformJwt.js applies
 * to auth.local.public_key_path: null-byte and traversal rejection before the read,
 * and a size ceiling. The path is operator-supplied config, not request input, so it
 * is not confined to the {{ file }} allowlist — an operator may keep the grant table
 * wherever they mount it.
 */
function readMappingFile(filePath) {
    if (typeof filePath !== 'string' || !filePath || filePath.includes('\0')) {
        throw new Error('role_to_scope_mapping is not a usable file path');
    }
    // Checked on the RAW input, before normalization: path.normalize collapses
    // "/etc/api-portal/../../etc/passwd" to "/etc/passwd", which contains no ".." and
    // would pass a post-normalization check. There is no allowlist root to contain the
    // result against here — the grant table may legitimately live wherever an operator
    // mounts it — so rejecting the traversal itself is the control.
    const rawSegments = filePath.split(/[/\\]/);
    if (rawSegments.includes('..')) {
        throw new Error(`role_to_scope_mapping "${filePath}" must not contain traversal sequences`);
    }
    const cleaned = path.normalize(filePath);
    let stat;
    try {
        stat = fs.statSync(cleaned);
    } catch (_) {
        throw new Error(`role_to_scope_mapping file "${filePath}" could not be read`);
    }
    if (!stat.isFile()) {
        throw new Error(`role_to_scope_mapping "${filePath}" is not a file`);
    }
    if (stat.size > MAX_MAPPING_BYTES) {
        throw new Error(`role_to_scope_mapping file "${filePath}" exceeds the maximum allowed size`);
    }
    try {
        return fs.readFileSync(cleaned, 'utf8');
    } catch (_) {
        throw new Error(`role_to_scope_mapping file "${filePath}" could not be read`);
    }
}

/**
 * Parses the grant table into a Map of role name -> deduplicated scope list.
 *
 * Shape (identical to platform-api's role-to-scope-mapping.yaml, so one file can
 * serve both components):
 *
 *   roles:
 *     - name: dp_admin
 *       scopes:
 *         - dp:api:manage
 *
 * A duplicate role name is rejected rather than last-wins: two entries for one role
 * means one of them is silently inert, and which one depends on file order.
 */
function parseRoleScopeMap(contents, filePath) {
    let doc;
    try {
        doc = yaml.load(contents);
    } catch (err) {
        throw new Error(`role_to_scope_mapping file "${filePath}" is not valid YAML: ${err.message}`);
    }
    if (!doc || typeof doc !== 'object' || !Array.isArray(doc.roles)) {
        throw new Error(
            `role_to_scope_mapping file "${filePath}" must contain a top-level "roles" list`
        );
    }

    const map = new Map();
    doc.roles.forEach((entry, index) => {
        if (!entry || typeof entry !== 'object') {
            throw new Error(`role_to_scope_mapping "${filePath}": entry ${index} is not a mapping`);
        }
        const name = typeof entry.name === 'string' ? entry.name.trim() : '';
        if (!name) {
            throw new Error(`role_to_scope_mapping "${filePath}": entry ${index} has no "name"`);
        }
        if (map.has(name)) {
            throw new Error(
                `role_to_scope_mapping "${filePath}": role "${name}" is declared more than once`
            );
        }
        if (!Array.isArray(entry.scopes)) {
            throw new Error(
                `role_to_scope_mapping "${filePath}": role "${name}" has no "scopes" list`
            );
        }
        const scopes = [];
        for (const scope of entry.scopes) {
            if (typeof scope !== 'string' || !scope.trim()) {
                throw new Error(
                    `role_to_scope_mapping "${filePath}": role "${name}" has a non-string scope`
                );
            }
            const trimmed = scope.trim();
            if (!scopes.includes(trimmed)) scopes.push(trimmed);
        }
        map.set(name, scopes);
    });
    return map;
}

/**
 * Shape check for a scope this portal does not enforce. It cannot confirm or deny
 * such a scope's existence — it only mints it into nothing and passes it along — so
 * the check is deliberately weaker than the spec lookup applied to dp:* below.
 */
function isWellFormedScope(scope) {
    const segments = scope.split(':');
    if (segments.length < 2) return false;
    return segments.every((segment, i) => {
        if (segment === '*') return i === segments.length - 1; // trailing wildcard only
        return SCOPE_SEGMENT_RE.test(segment);
    });
}

/**
 * Extracts every dp:* scope the portal's OpenAPI spec declares, from each security
 * scheme's OAuth2 flows. This is the authority on what dp:* scopes exist: the same
 * document express-openapi-validator enforces per operation, so a scope absent here
 * can never satisfy any operation.
 */
function readDeclaredPortalScopes(specPath) {
    let doc;
    try {
        doc = yaml.load(fs.readFileSync(specPath, 'utf8'));
    } catch (err) {
        throw new Error(`API Portal OpenAPI spec "${specPath}" could not be read: ${err.message}`);
    }
    const declared = new Set();
    const schemes = doc?.components?.securitySchemes || {};
    for (const scheme of Object.values(schemes)) {
        for (const flow of Object.values(scheme?.flows || {})) {
            for (const scope of Object.keys(flow?.scopes || {})) {
                declared.add(scope);
            }
        }
    }
    if (declared.size === 0) {
        throw new Error(`API Portal OpenAPI spec "${specPath}" declares no OAuth2 scopes`);
    }
    return declared;
}

/**
 * Namespace-scoped validation, mirroring platform-api's ValidateRoleScopeMap:
 *
 *   dp:*  this portal's own scopes — must be declared in its OpenAPI spec. An
 *         unknown one fails startup rather than surfacing later as a role that
 *         authenticates fine and is then denied every request.
 *   other a namespace this portal neither declares nor enforces (ap:* from the
 *         Platform API) — checked for well-formedness only, so one shared grant
 *         table can carry another component's scopes without this one rejecting it.
 *
 * Collects every problem before throwing so an operator fixing a hand-written file
 * sees the whole list, not the first line that happens to be wrong.
 */
function validateRoleScopeMap(map, declaredScopes, filePath) {
    const problems = [];
    for (const [role, scopes] of map) {
        if (scopes.length === 0) {
            problems.push(`role "${role}" grants no scopes`);
            continue;
        }
        for (const scope of scopes) {
            if (scope.startsWith(OWN_SCOPE_PREFIX)) {
                if (!declaredScopes.has(scope)) {
                    problems.push(
                        `role "${role}" grants "${scope}", which the API Portal OpenAPI spec does not declare`
                    );
                }
                continue;
            }
            if (!isWellFormedScope(scope)) {
                problems.push(`role "${role}" grants malformed scope "${scope}"`);
            }
        }
    }
    if (problems.length > 0) {
        throw new Error(
            `role_to_scope_mapping file "${filePath}" is invalid:\n  - ${problems.join('\n  - ')}`
        );
    }
}

// The validated map, installed by init(). Null until then — expandRoles() treats
// that as "no grants", never as "grant everything".
let roleScopeMap = null;

/**
 * Loads, parses and validates the grant table. Throws on any problem; the caller
 * (configLoader) turns that into a fail-closed startup abort.
 */
function loadRoleScopeMap(filePath, specPath) {
    const map = parseRoleScopeMap(readMappingFile(filePath), filePath);
    validateRoleScopeMap(map, readDeclaredPortalScopes(specPath), filePath);
    return map;
}

function init(filePath, specPath) {
    roleScopeMap = loadRoleScopeMap(filePath, specPath);
    return roleScopeMap;
}

function isLoaded() {
    return roleScopeMap !== null;
}

function roleNames() {
    return roleScopeMap ? [...roleScopeMap.keys()] : [];
}

/**
 * Expands a token's roles claim into the union of the scopes those roles grant,
 * duplicates collapsed — most-permissive wins across multiple roles, matching
 * platform-api's effectiveScopes.
 *
 * Accepts the claim in any shape an IDP might emit it: an array, or a
 * space/comma-separated string. An unknown role contributes nothing rather than
 * being treated as a scope value in its own right — the failure mode is a denied
 * request, never an unintended grant.
 */
function expandRoles(rolesClaim) {
    if (!roleScopeMap || !rolesClaim) return [];
    const roles = Array.isArray(rolesClaim)
        ? rolesClaim
        : String(rolesClaim).split(/[\s,]+/);
    const scopes = [];
    for (const role of roles) {
        const name = typeof role === 'string' ? role.trim() : '';
        if (!name) continue;
        for (const scope of roleScopeMap.get(name) || []) {
            if (!scopes.includes(scope)) scopes.push(scope);
        }
    }
    return scopes;
}

module.exports = {
    init,
    isLoaded,
    roleNames,
    expandRoles,
    // Exported for tests and for configLoader's startup validation.
    loadRoleScopeMap,
    parseRoleScopeMap,
    validateRoleScopeMap,
    readDeclaredPortalScopes,
    isWellFormedScope,
};
