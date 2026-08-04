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

const path = require('path');
const fs = require('fs');
const crypto = require('crypto');
const toml = require('smol-toml');
const Handlebars = require('handlebars');
const { DEFAULTS } = require('./configDefaults');
const { snakeToCamelDeep, mergeOver, parseConfigPaths } = require('./configMerge');
// Requires nothing from this module in return, so loading the grant table from the
// startup validation below cannot cycle.
const roleScopeMap = require('./roleScopeMap');

// Load api-platform.env if present (silently ignored if absent)
try {
    require('dotenv').config({ path: path.join(process.cwd(), 'api-platform.env') });
} catch (_) {}

/**
 * Load and deep-merge one or more config.toml files, in the order given, with
 * last-wins precedence — the layered pattern the Go components already use via a
 * repeatable `-config` flag. Nested tables deep-merge; list/array values are
 * replaced wholesale (see mergeOver in configMerge.js). Interpolation of
 * {{ env }} / {{ file }} tokens is deliberately NOT done here — it runs once on
 * the fully merged tree (see below), so a token declared in a base file can be
 * overridden by a later overlay before either is resolved.
 *
 * Every key lives under the single [api_portal] table. That wrapper is
 * unwrapped here so the in-code config tree stays flat (config.server,
 * config.security, …); anything outside the [api_portal] table is ignored.
 *
 * There is NO silent fallback: a missing, unreadable, or unparseable file throws
 * — the module bootstrap turns that into a fatal, non-zero exit at startup
 * rather than booting on pure built-in DEFAULTS.
 */
function loadConfigFiles(paths) {
    let merged = {};
    for (const p of paths) {
        const resolvedPath = path.resolve(p);
        let raw;
        try {
            raw = fs.readFileSync(resolvedPath, 'utf8');
        } catch (err) {
            throw new Error(`config file "${p}" could not be read: ${err.message}`);
        }
        let parsed;
        try {
            parsed = toml.parse(raw);
        } catch (err) {
            throw new Error(`config file "${p}" is not valid TOML: ${err.message}`);
        }
        const tree = snakeToCamelDeep(parsed).apiPortal || {};
        merged = mergeOver(merged, tree);
    }
    // Every --config file layered and nothing came out: no file carried a
    // [api_portal] table at all. Returning {} here would boot on pure
    // built-in DEFAULTS — the exact silent fallback this loader exists to
    // prevent — so fail here, where the cause is still nameable.
    if (Object.keys(merged).length === 0) {
        throw new Error(
            `config file(s) ${paths.map(p => `"${p}"`).join(', ')} produced an empty ` +
            'configuration: no [api_portal] table found. Refusing to start on ' +
            'built-in defaults alone.'
        );
    }
    return merged;
}

// ---------------------------------------------------------------------------
// Config value interpolation: {{ env "NAME" ["fallback"] }} / {{ file "/path" }}
// ---------------------------------------------------------------------------
//
// This is the JS counterpart to common/configinterpolate — the Go package
// platform-api uses for the same purpose (see platform-api/config/config.go) —
// and follows the exact same contract so the two config files read the same
// way to an operator:
//
//   {{ env "NAME" }}             -> value of NAME; FAILS CLOSED (aborts startup)
//                                    if NAME is unset or empty. A set-but-empty
//                                    variable counts as unset (bash ${NAME:?}
//                                    semantics).
//   {{ env "NAME" "fallback" }}  -> value of NAME if set and non-empty, else the
//                                    literal "fallback".
//   {{ file "/path" }}           -> the trimmed contents of /path. Always
//                                    required — missing, unreadable, oversize, or
//                                    disallowed is a hard startup error.
//
// There is no automatic APIP_AP_* prefix mapping anymore (removed the same way
// platform-api removed its koanf env-prefix mapping) — a variable only takes
// effect where config.toml explicitly references it via {{ env "..." }}.
//
// A dedicated Handlebars instance (Handlebars.create()) is used rather than the
// shared singleton src/helpers/handlebarsHelpers.js registers page-rendering
// helpers on, so config interpolation and page templates never share helpers.

const hb = Handlebars.create();

// Directories a {{ file "..." }} path may read from by default. Overridable via
// the APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var — shared across every
// api-platform component (see common/configinterpolate.EnvFileSourceAllowlist),
// read directly rather than through {{ env }} since it gates interpolation
// itself and so can't be one of its own references.
const DEFAULT_FILE_ALLOWLIST = ['/etc/api-portal', '/secrets/api-portal'];
const FILE_ALLOWLIST_ENV_VAR = 'APIP_CONFIG_FILE_SOURCE_ALLOWLIST';

// Secret files (tokens, keys, passwords) are far smaller than this; the cap
// guards against accidentally reading a huge file into memory.
const MAX_FILE_BYTES = 1 << 20; // 1 MiB

function getFileAllowlist() {
    const raw = process.env[FILE_ALLOWLIST_ENV_VAR];
    if (!raw || !raw.trim()) return DEFAULT_FILE_ALLOWLIST;
    const dirs = raw.split(',').map(d => d.trim()).filter(Boolean);
    return dirs.length ? dirs : DEFAULT_FILE_ALLOWLIST;
}

function isAllowed(candidatePath, allowlist) {
    return allowlist.some(dir => candidatePath.startsWith(path.normalize(dir) + path.sep));
}

// Resolves an allowlist root's symlinks so it can be compared against a
// symlink-resolved candidate file path. A root that doesn't exist falls back to
// its cleaned form — harmless, since no readable file could live under it.
function resolveAllowlistRoot(dir) {
    const cleaned = path.resolve(dir);
    try {
        return fs.realpathSync(cleaned);
    } catch (_) {
        return cleaned;
    }
}

function isAllowedResolved(resolvedPath, allowlist) {
    return allowlist.some(dir => resolvedPath.startsWith(resolveAllowlistRoot(dir) + path.sep));
}

/**
 * Reads an allowlisted file for {{ file "..." }}, enforcing this project's
 * file-access rules: null-byte/traversal rejection, allowlist containment on the
 * input path, symlink resolution and a second containment check against the
 * resolved path (prevents a TOCTOU swap between the check and the read), and a
 * size cap. Error messages name the operator-supplied path only — never the
 * file contents or the allowlist.
 */
function readAllowedFile(inputPath) {
    if (inputPath.includes('\0')) {
        throw new Error(`file "${inputPath}" is not in an allowed source directory`);
    }
    const cleaned = path.normalize(inputPath);
    if (cleaned.includes('..')) {
        throw new Error(`file "${inputPath}" is not in an allowed source directory`);
    }

    const allowlist = getFileAllowlist();
    if (!allowlist.length) {
        throw new Error('file interpolation not permitted: no allowlist configured');
    }
    if (!isAllowed(cleaned, allowlist)) {
        throw new Error(`file "${inputPath}" is not in an allowed source directory`);
    }

    let resolved;
    try {
        resolved = fs.realpathSync(cleaned);
    } catch (_) {
        throw new Error(`required file "${inputPath}" is not found`);
    }
    if (!isAllowedResolved(resolved, allowlist)) {
        throw new Error(`file "${inputPath}" is not in an allowed source directory`);
    }

    let stat;
    try {
        stat = fs.statSync(resolved);
    } catch (_) {
        throw new Error(`required file "${inputPath}" is not found`);
    }
    if (stat.size > MAX_FILE_BYTES) {
        throw new Error(`file "${inputPath}" exceeds the maximum allowed size`);
    }

    let data;
    try {
        data = fs.readFileSync(resolved, 'utf8');
    } catch (_) {
        throw new Error(`required file "${inputPath}" is not found`);
    }
    return data.replace(/[ \t\r\n]+$/, '');
}

hb.registerHelper('env', function envHelper(...args) {
    args.pop(); // discard the Handlebars options object, always the last argument
    const [name, fallback] = args;
    if (typeof name !== 'string' || !name) {
        throw new Error('{{ env }} requires a variable name, e.g. {{ env "APIP_AP_X" }}');
    }
    envRefCount += 1;
    const value = process.env[name];
    if (value !== undefined && value !== '') return value;
    if (typeof fallback === 'string') return fallback;
    throw new Error(`required env var "${name}" is not found`);
});

hb.registerHelper('file', function fileHelper(...args) {
    args.pop();
    const [filePath] = args;
    if (typeof filePath !== 'string' || !filePath) {
        throw new Error('{{ file }} requires a path, e.g. {{ file "/secrets/x" }}');
    }
    fileRefCount += 1;
    return readAllowedFile(filePath);
});

/**
 * Coerce a string value (post-interpolation) to the most appropriate JS type.
 * Only ever applied to leaves that were actually templated — a plain TOML
 * literal keeps its native TOML type and is never passed through this.
 */
function coerceValue(value) {
    if (value === 'true') return true;
    if (value === 'false') return false;
    if (value !== '' && !isNaN(Number(value))) return Number(value);
    return value;
}

let envRefCount = 0;
let fileRefCount = 0;
let fieldCount = 0;

/**
 * Interpolate a single TOML leaf. fieldPath is a dotted/bracketed path (e.g.
 * "security.encryptionKey" or "idp.scopes[0]") used only to point at which
 * field failed if interpolation throws.
 */
function interpolateLeaf(value, fieldPath) {
    if (typeof value !== 'string' || !value.includes('{{')) {
        // No template syntax at all — a plain literal, passed through with its
        // native TOML type/value, never coerced.
        return value;
    }
    fieldCount += 1;
    try {
        const compiled = hb.compile(value, { noEscape: true, strict: true });
        return coerceValue(compiled({}));
    } catch (err) {
        throw new Error(`config interpolation failed at "${fieldPath}": ${err.message}`);
    }
}

/**
 * Recursively interpolate every string leaf of a parsed config.toml object
 * (including array elements), before it's merged over DEFAULTS.
 */
function interpolateTree(value, fieldPath) {
    if (Array.isArray(value)) {
        return value.map((item, i) => interpolateTree(item, `${fieldPath}[${i}]`));
    }
    if (value !== null && typeof value === 'object') {
        const out = {};
        for (const [k, v] of Object.entries(value)) {
            out[k] = interpolateTree(v, fieldPath ? `${fieldPath}.${k}` : k);
        }
        return out;
    }
    return interpolateLeaf(value, fieldPath);
}

// ---------------------------------------------------------------------------
// Startup config resolution
// ---------------------------------------------------------------------------
//
// Resolved once, at module load, from the repeatable `--config` flag — the JS
// counterpart to the Go components' `-config`. At least one `--config` is
// REQUIRED: there is no default path and no silent fallback to built-in
// DEFAULTS. Because env vars reach config only through explicit {{ env }} tokens
// in a file (there is no APIP_AP_* env-prefix provider), a missing config file
// means "running on pure built-in defaults" — unacceptable for a portal handling
// auth/session/secret config, so it fails fast with a non-zero exit here, before
// the server binds. process.exit is used (not a thrown error) because the logger
// is not yet initialised and several consumers require this module for its side
// effect of producing a ready `config`.

function fatalConfig(message) {
    // logger is not yet initialised at this point — write to stderr directly.
    process.stderr.write(`[FATAL] ${message}\n`);
    process.exit(1);
}

let configPaths;
try {
    configPaths = parseConfigPaths(process.argv.slice(2));
} catch (err) {
    fatalConfig(err.message);
}

if (configPaths.length === 0) {
    fatalConfig(
        'no configuration file provided. Pass at least one --config <path> ' +
        '(repeatable; later files override earlier ones, key by key), e.g. ' +
        '--config configs/config.toml. There is no default path and no ' +
        'silent-defaults fallback.'
    );
}

let rawTomlConfig;
try {
    rawTomlConfig = loadConfigFiles(configPaths);
} catch (err) {
    fatalConfig(err.message);
}

// {{ env }} / {{ file }} interpolation runs ONCE, here, on the fully merged tree
// — after every --config file has been layered — so a token can be declared in a
// base file and left in place (or overridden) by a later overlay before it is
// resolved.
let interpolatedTomlConfig;
try {
    interpolatedTomlConfig = interpolateTree(rawTomlConfig, '');
} catch (err) {
    fatalConfig(err.message);
}

// Precedence: DEFAULTS (source of truth) → merged --config files, with {{ env }}/
// {{ file }} references resolved before this final merge.
const config = mergeOver(JSON.parse(JSON.stringify(DEFAULTS)), interpolatedTomlConfig);

if (fieldCount > 0) {
    process.stderr.write(
        `[INFO] Config: resolved ${envRefCount} env reference(s), ${fileRefCount} file reference(s) across ${fieldCount} field(s).\n`
    );
}

/**
 * Fail-closed startup check: required security secrets must be present and
 * valid before the application is allowed to start. There is no ephemeral/
 * generated fallback — a missing or malformed secret aborts the process
 * immediately rather than starting with a weaker, silently-regenerated one.
 */
function requireHexSecret(value, fieldName) {
    if (!value || !/^[0-9a-fA-F]{64}$/.test(value)) {
        process.stderr.write(
            `[FATAL] security.${fieldName} did not resolve to a 64-character hex string. ` +
            'Refusing to start with a missing or malformed secret. ' +
            'Generate one with: openssl rand -hex 32 — then reference it from configs/config.toml, ' +
            `e.g. ${fieldName === 'encryptionKey' ? 'encryption_key' : 'session_secret'} = '{{ file "/etc/api-portal/keys/${fieldName === 'encryptionKey' ? 'encryption.key' : 'session-secret'}" }}'.\n`
        );
        process.exit(1);
    }
}

// Design mode renders entirely from disk: no database, so there are no stored
// secrets to encrypt (encryptionKey stays unused — createCryptoUtil is lazy and
// only throws if an encrypt/decrypt actually runs, which the disabled
// webhook/subscription paths never do here), and sessions use an in-memory store
// (see sessionStoreConfig.js). The session cookie is still signed, so mint an
// ephemeral secret when none was supplied rather than forcing an operator to
// configure secret files a local preview never persists anything with. This is
// the one place the fail-closed secret requirement is waived, gated on the
// explicit, off-by-default design_mode.enabled flag.
if (config.designMode?.enabled) {
    if (!config.security.sessionSecret) {
        config.security.sessionSecret = crypto.randomBytes(32).toString('hex');
    }
    // Validate whatever we ended up with — the freshly minted value passes, and an
    // operator-supplied one is held to the same format check as every other mode
    // rather than silently bypassing it. encryptionKey stays unchecked: it is
    // genuinely unused in design mode (no database, createCryptoUtil is lazy).
    requireHexSecret(config.security.sessionSecret, 'sessionSecret');
} else {
    requireHexSecret(config.security.encryptionKey, 'encryptionKey');
    requireHexSecret(config.security.sessionSecret, 'sessionSecret');
}

/**
 * Fail-closed startup check: database connection-pool settings must resolve to
 * sane numbers before the application is allowed to start. coerceValue() only
 * converts a leaf to a Number when the *entire* string is numeric — a
 * malformed override (e.g. APIP_AP_DATABASE_MAX_OPEN_CONNS="abc") is left as
 * that raw string rather than becoming NaN, and would otherwise reach
 * pg.Pool()/mssql.ConnectionPool() unvalidated (see postgresAdapter.js /
 * mssqlAdapter.js), producing a silently broken or uncapped pool.
 */
function validateDatabasePoolConfig(database) {
    if (database.driver !== 'postgres' && database.driver !== 'mssql') return;

    const nonNegativeFields = [
        'poolIdleTimeoutMs', 'poolConnectionTimeoutMs', 'poolRequestTimeoutMs', 'minOpenConns',
    ];
    for (const field of nonNegativeFields) {
        const value = database[field];
        if (!Number.isInteger(value) || value < 0) {
            process.stderr.write(
                `[FATAL] database.${field} must resolve to a non-negative integer, got ${JSON.stringify(value)}. ` +
                'Refusing to start with an invalid database connection-pool setting.\n'
            );
            process.exit(1);
        }
    }

    const maxOpenConns = database.maxOpenConns;
    if (!Number.isInteger(maxOpenConns) || maxOpenConns < 1) {
        process.stderr.write(
            `[FATAL] database.maxOpenConns must resolve to an integer >= 1, got ${JSON.stringify(maxOpenConns)}. ` +
            'Refusing to start with an invalid database connection-pool setting.\n'
        );
        process.exit(1);
    }

    if (database.minOpenConns > maxOpenConns) {
        process.stderr.write(
            `[FATAL] database.minOpenConns (${database.minOpenConns}) must not exceed database.maxOpenConns (${maxOpenConns}).\n`
        );
        process.exit(1);
    }
}

validateDatabasePoolConfig(config.database);

/**
 * Resolves organization.handle and fails closed when it is missing or unusable.
 *
 * This portal instance serves exactly one organization, so an unresolvable handle
 * is not a degraded mode — every page route and REST request would reject with no
 * clear cause. Catch it at startup instead, following the same fail-closed pattern
 * as the secret/pool checks above.
 *
 * Design mode is the one exception: it renders from disk and never touches the
 * organization tables (see app.js's designMode branch), so it has no organization
 * to pin. That makes this a narrow, explicit opt-out rather than an implicit
 * fallback to "no organization configured".
 *
 * `tomlOrg` is the raw [api_portal.organization] table — needed to tell an
 * explicitly-configured handle apart from the DEFAULTS one, which is what makes
 * the deprecated default_name alias resolvable.
 */
// Permissive enough for any handle orgDao.create() would have accepted (it only
// lowercases), strict enough that the value is safe to interpolate into the
// redirect targets built from it — a handle containing '/' or '\' could otherwise
// turn a relative redirect into an off-site one, the same hazard authController's
// SAFE_HANDLE guards against for the URL-supplied value.
const ORG_HANDLE_PATTERN = /^[a-z0-9][a-z0-9._-]*$/;

// Root path segments the portal owns. `/:orgName` is matched at the true root, so a
// handle equal to any of these would shadow — or be shadowed by — a real endpoint: a
// handle mounted before the org router makes the org silently unreachable, one mounted
// after silently shadows the platform path (BASE_PATH, the API base, health/metrics,
// the static mounts, the MCP registry, dev/well-known endpoints). Enforced fail-closed
// at startup so the collision surfaces as a refused boot rather than a page that simply
// never loads. Compared against the already-lowercased handle. (View handles need no
// such list — they sit behind a literal `views/` segment and so can't collide.)
const RESERVED_ORG_HANDLES = new Set([
    'api', 'api-portal', 'health', 'metrics', 'favicon.ico', 'styles', 'images',
    'scripts', 'technical-styles', 'technical-scripts', 'mock', 'registry', 'portal',
    '__dev_reload', '.well-known', 'robots.txt',
]);

function resolveOrganizationConfig(cfg, tomlOrg) {
    const org = cfg.organization;
    // Read `handle` from the raw config.toml table, not from cfg: DEFAULTS always
    // supplies a handle, so the merged value can't distinguish "the operator set
    // this" from "nobody set anything". Only an explicitly-configured handle may
    // take precedence over the deprecated default_name.
    const trimmed = (v) => (typeof v === 'string' ? v.trim() : '');
    const explicit = trimmed(tomlOrg?.handle);
    const legacy = trimmed(tomlOrg?.defaultName);

    if (!explicit && legacy) {
        process.stderr.write(
            `[WARN] Config: organization.default_name ("${legacy}") is deprecated — rename it to ` +
            'organization.handle. Using its value for this run.\n'
        );
        org.handle = legacy;
    } else if (legacy && legacy !== explicit) {
        process.stderr.write(
            `[WARN] Config: organization.default_name ("${legacy}") is deprecated and ignored ` +
            `because organization.handle ("${explicit}") is set. Remove default_name.\n`
        );
    }

    if (cfg.designMode?.enabled) {
        // Nothing to pin — design mode renders from disk. Leave whatever resolved
        // above in place (unused) rather than validating it.
        return;
    }

    if (!org.handle) {
        process.stderr.write(
            '[FATAL] organization.handle is not configured. This portal serves a single ' +
            'organization and cannot start without knowing which one. Set it in ' +
            "configs/config.toml, e.g. handle = '{{ env \"APIP_AP_ORGANIZATION_HANDLE\" \"default\" }}'.\n"
        );
        process.exit(1);
    }

    // Handles are stored lowercase (orgDao.create lowercases on insert and
    // findOrgByIdentifier lowercases on lookup), so normalize here too — otherwise
    // a config value of "Acme" would never match the stored "acme".
    org.handle = org.handle.toLowerCase();

    if (!ORG_HANDLE_PATTERN.test(org.handle)) {
        process.stderr.write(
            `[FATAL] organization.handle ("${org.handle}") is not a valid handle. ` +
            'Expected a URL-safe slug starting with a letter or digit and containing only ' +
            'letters, digits, dots, underscores, and hyphens.\n'
        );
        process.exit(1);
    }

    if (RESERVED_ORG_HANDLES.has(org.handle)) {
        process.stderr.write(
            `[FATAL] organization.handle ("${org.handle}") is a reserved word. It collides with a ` +
            'root path the portal owns (the mount prefix, the API base, health/metrics, the static ' +
            'asset mounts, the MCP registry, or a dev/well-known endpoint), which would make the ' +
            'organization silently unreachable. Choose a different handle. Reserved: ' +
            `${[...RESERVED_ORG_HANDLES].join(', ')}.\n`
        );
        process.exit(1);
    }
}

resolveOrganizationConfig(config, interpolatedTomlConfig.organization);

// Every artifact type this portal knows how to serve. `artifacts.enabled_types`
// is an allowlist drawn from this set.
const KNOWN_ARTIFACT_TYPES = ['apis', 'mcp-servers', 'api-workflows'];

/**
 * Validates artifacts.enabledTypes against KNOWN_ARTIFACT_TYPES and aborts on
 * anything else.
 *
 * An allowlist fails differently from a set of booleans: a misspelt boolean key
 * is ignored and its default still applies, but a misspelt array entry silently
 * drops that artifact type — the portal starts, serves fewer pages than intended,
 * and 404s look like a routing bug. Fail at startup instead, consistent with how
 * the other required config is checked above.
 */
function validateArtifactConfig(artifacts) {
    const enabled = artifacts?.enabledTypes;
    if (enabled === undefined) return; // section omitted — defaults apply

    if (!Array.isArray(enabled)) {
        process.stderr.write(
            '[FATAL] artifacts.enabled_types must be an array, e.g. ' +
            'enabled_types = ["apis", "mcp-servers"].\n'
        );
        process.exit(1);
    }
    const unknown = enabled.filter((t) => !KNOWN_ARTIFACT_TYPES.includes(t));
    if (unknown.length) {
        process.stderr.write(
            `[FATAL] artifacts.enabled_types contains unknown ${unknown.length === 1 ? 'entry' : 'entries'}: ` +
            `${unknown.map((t) => `"${t}"`).join(', ')}. Valid entries are ` +
            `${KNOWN_ARTIFACT_TYPES.map((t) => `"${t}"`).join(', ')}.\n`
        );
        process.exit(1);
    }
    if (!enabled.length) {
        process.stderr.write(
            '[WARN] artifacts.enabled_types is empty — this portal serves no artifacts.\n'
        );
    }
}

validateArtifactConfig(config.artifacts);

// ---------------------------------------------------------------------------
// Authorization config (auth.authorization)
// ---------------------------------------------------------------------------

const AUTHORIZATION_MODES = ['scope', 'role'];

// The OpenAPI document apiPortalRouter serves /api/v0.9 from — the authority on
// which dp:* scopes exist, so a grant table naming one that isn't declared there
// can be rejected at startup rather than denying requests later.
const PORTAL_SPEC_PATH = path.join(
    __dirname, '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml'
);

/**
 * Rejects config keys retired when authentication and authorization were split into
 * separate sections.
 *
 * A retired key no longer maps to anything, so leaving it in place would silently
 * apply the DEFAULTS value instead of what the file says — `role_validation = true`
 * would read as "page gating on" to the operator and mean "off" to the portal. Both
 * of these keys govern authorization, so failing here is what keeps a half-migrated
 * config from starting and enforcing something other than what it states.
 *
 * Checked against the raw config.toml tree, not the merged one: DEFAULTS no longer
 * carries either key, so a hit here can only be operator-supplied.
 */
function rejectRetiredAuthKeys(tomlAuth) {
    if (!tomlAuth) return;
    const retired = [];
    if (tomlAuth.roleValidation !== undefined) {
        retired.push(
            'auth.role_validation is retired — it is now ' +
            'auth.authorization.page_role_validation (per-page role gating). Note that ' +
            'REST-API scope enforcement is a separate switch, auth.authorization.enabled, ' +
            'which is on by default'
        );
    }
    if (tomlAuth.idp?.roles !== undefined) {
        retired.push(
            'auth.idp.roles is retired — it is now auth.authorization.portal_roles, ' +
            'outside the idp block, because the portal reads these role names in local ' +
            'auth mode as well'
        );
    }
    if (retired.length) {
        process.stderr.write(
            `[FATAL] Retired authorization config key(s):\n  - ${retired.join('\n  - ')}\n` +
            'Refusing to start: a retired key is ignored, so the effective setting would ' +
            'be the default rather than what the config file says.\n'
        );
        process.exit(1);
    }
}

/**
 * Fail-closed startup check for the authorization section.
 *
 * Runs in every auth mode, not inside a per-mode branch: a token carries the same
 * roles claim whether it was verified against a JWKS endpoint (idp) or the Platform
 * API's public key (local), so role authorization is configurable — and must be
 * validated — in both.
 *
 * `mode` is checked even when `enabled = false`, so a typo surfaces when it is
 * written rather than months later when enforcement is switched back on.
 */
function validateAuthorizationConfig(cfg) {
    const authz = cfg.auth?.authorization;
    if (!authz) {
        process.stderr.write('[FATAL] auth.authorization is missing from the resolved config.\n');
        process.exit(1);
    }

    if (!AUTHORIZATION_MODES.includes(authz.mode)) {
        process.stderr.write(
            `[FATAL] auth.authorization.mode must be one of ${AUTHORIZATION_MODES.map(m => `"${m}"`).join(' | ')}, ` +
            `got ${JSON.stringify(authz.mode)}.\n`
        );
        process.exit(1);
    }

    if (authz.mode === 'role') {
        // Without a roles claim mapping there is nothing to expand; without the grant
        // table the role names would be used verbatim as scope values, which matches no
        // operation and denies every request.
        if (!cfg.auth?.claimMappings?.roles) {
            process.stderr.write(
                '[FATAL] auth.authorization.mode = "role" requires auth.claim_mappings.roles — ' +
                'the token claim carrying the roles to expand (e.g. "roles", or ' +
                '"realm_access.roles" for Keycloak).\n'
            );
            process.exit(1);
        }
        if (!authz.roleToScopeMapping) {
            process.stderr.write(
                '[FATAL] auth.authorization.mode = "role" requires ' +
                'auth.authorization.role_to_scope_mapping — the path to the YAML grant table ' +
                'defining what each role may do. Without it, role names would be used as ' +
                'scope values and every request would be denied.\n'
            );
            process.exit(1);
        }
    }

    // Loaded whenever a path is configured, regardless of mode: an operator switching
    // to role mode should find out at the next restart that their grant table is
    // valid, not the first time a request is authorized against it.
    if (authz.roleToScopeMapping) {
        try {
            const map = roleScopeMap.init(authz.roleToScopeMapping, PORTAL_SPEC_PATH);
            process.stderr.write(
                `[INFO] Authorization: loaded ${map.size} role(s) from ` +
                `"${authz.roleToScopeMapping}" (mode = "${authz.mode}").\n`
            );
        } catch (err) {
            process.stderr.write(`[FATAL] ${err.message}\n`);
            process.exit(1);
        }
    }

    if (!authz.enabled) {
        process.stderr.write(
            '[WARN] auth.authorization.enabled = false — REST API operations accept any ' +
            'authenticated caller regardless of the scopes they declare. Development only.\n'
        );
    }
}

rejectRetiredAuthKeys(interpolatedTomlConfig.auth);
validateAuthorizationConfig(config);

module.exports = { config, KNOWN_ARTIFACT_TYPES, AUTHORIZATION_MODES };
