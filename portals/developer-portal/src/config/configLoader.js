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
const toml = require('smol-toml');
const Handlebars = require('handlebars');
const { DEFAULTS } = require('./configDefaults');

// Load api-platform.env if present (silently ignored if absent)
try {
    require('dotenv').config({ path: path.join(process.cwd(), 'api-platform.env') });
} catch (_) {}

/**
 * Recursively convert snake_case keys to camelCase (e.g. "base_url" -> "baseUrl").
 * Applied to the parsed TOML tree so config.toml can use snake_case while the
 * in-code struct and every consumer use camelCase.
 */
function snakeToCamel(key) {
    return key.replace(/_([a-z0-9])/g, (_, c) => c.toUpperCase());
}

function snakeToCamelDeep(value) {
    if (Array.isArray(value)) {
        return value.map(snakeToCamelDeep);
    }
    if (value !== null && typeof value === 'object') {
        const out = {};
        for (const [k, v] of Object.entries(value)) {
            out[snakeToCamel(k)] = snakeToCamelDeep(v);
        }
        return out;
    }
    return value;
}

/**
 * Load configs/config.toml (snake_case), converted to camelCase.
 * Returns an empty object if the file does not exist, so DEFAULTS alone can
 * drive the app.
 *
 * Every key lives under the single [developer_portal] table (mirrors the
 * Platform API's [platform_api] and the AI Workspace's [ai_workspace] tables),
 * so one shared config.toml could hold all three services' sections side by
 * side without their tables colliding. That wrapper is unwrapped here so the
 * in-code config tree stays flat (config.server, config.security, …); anything
 * outside the [developer_portal] table is ignored.
 */
function loadTomlConfig() {
    const tomlPath = path.join(process.cwd(), 'configs', 'config.toml');

    if (fs.existsSync(tomlPath)) {
        const raw = fs.readFileSync(tomlPath, 'utf8');
        return snakeToCamelDeep(toml.parse(raw)).developerPortal || {};
    }
    return {};
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
// There is no automatic APIP_DP_* prefix mapping anymore (removed the same way
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
const DEFAULT_FILE_ALLOWLIST = ['/etc/devportal', '/secrets/devportal'];
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
        throw new Error('{{ env }} requires a variable name, e.g. {{ env "APIP_DP_X" }}');
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

/**
 * Prototype-pollution guard, applied to the DEFAULTS<-config.toml merge below.
 */
const BLOCKED_KEYS = new Set(['__proto__', 'prototype', 'constructor']);

/**
 * Deep-merge src into dst (src wins on conflicts) — used to layer the
 * interpolated config.toml tree over a clone of DEFAULTS.
 */
function mergeOver(dst, src) {
    for (const [k, v] of Object.entries(src)) {
        if (BLOCKED_KEYS.has(k)) continue;
        if (v !== null && typeof v === 'object' && !Array.isArray(v) &&
            dst[k] !== null && typeof dst[k] === 'object' && !Array.isArray(dst[k])) {
            mergeOver(dst[k], v);
        } else {
            dst[k] = v;
        }
    }
    return dst;
}

const rawTomlConfig = loadTomlConfig();

let interpolatedTomlConfig;
try {
    interpolatedTomlConfig = interpolateTree(rawTomlConfig, '');
} catch (err) {
    // Use process.stderr directly — logger is not yet initialised at this point
    process.stderr.write(`[FATAL] ${err.message}\n`);
    process.exit(1);
}

// Precedence: DEFAULTS (source of truth) → configs/config.toml, with {{ env }}/
// {{ file }} references resolved before the merge.
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
            `e.g. ${fieldName === 'encryptionKey' ? 'encryption_key' : 'session_secret'} = '{{ env "APIP_DP_SECURITY_${fieldName === 'encryptionKey' ? 'ENCRYPTIONKEY' : 'SESSIONSECRET'}" }}'.\n`
        );
        process.exit(1);
    }
}

requireHexSecret(config.security.encryptionKey, 'encryptionKey');
requireHexSecret(config.security.sessionSecret, 'sessionSecret');

module.exports = { config };
