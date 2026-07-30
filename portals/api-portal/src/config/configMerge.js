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

// Side-effect-free helpers for the layered config loader (see configLoader.js).
// Kept in their own module so they can be unit-tested (configMerge.test.js)
// without triggering configLoader's module-load bootstrap, which resolves the
// real config from `--config` flags and exits the process when none are given.

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
    if (value instanceof Date) {
        // TOML date/date-time values parse to Date objects (smol-toml's TomlDate);
        // recursing into one would flatten it to {}.
        return value;
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
 * Prototype-pollution guard, applied to every mergeOver() below.
 */
const BLOCKED_KEYS = new Set(['__proto__', 'prototype', 'constructor']);

/**
 * Deep-merge src into dst (src wins on conflicts), returning dst.
 *
 * Merge semantics — used both to layer config files over each other (last-wins)
 * and to layer the merged result over DEFAULTS:
 *   - Nested tables (plain objects) deep-merge, key by key.
 *   - List/array values are REPLACED wholesale, never appended or merged
 *     index-wise. An overlay that sets a list key replaces the base list — so a
 *     later --config file listing e.g. auth.scopes = ["a"] overrides the base
 *     list entirely rather than concatenating.
 *   - A scalar overrides whatever occupied the key before it.
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

/**
 * Parse repeatable `--config <path>` / `--config=<path>` flags out of an argv
 * slice (pass process.argv.slice(2)), preserving order. Mirrors the Go
 * components' repeatable `-config` flag; order matters because files are merged
 * last-wins downstream. Every other argument is ignored (nodemon and the Node
 * runtime pass their own flags through here). Returns the list of paths, which
 * may be empty — the caller enforces the "at least one required" rule.
 */
function parseConfigPaths(argv) {
    const paths = [];
    for (let i = 0; i < argv.length; i++) {
        const arg = argv[i];
        if (arg === '--config') {
            const value = argv[i + 1];
            if (value === undefined || value.startsWith('--')) {
                throw new Error('--config flag requires a file path argument');
            }
            paths.push(value);
            i++; // skip the value we just consumed
        } else if (arg.startsWith('--config=')) {
            const value = arg.slice('--config='.length);
            if (!value) {
                throw new Error('--config= flag requires a non-empty file path');
            }
            paths.push(value);
        }
    }
    return paths;
}

module.exports = { snakeToCamel, snakeToCamelDeep, mergeOver, parseConfigPaths, BLOCKED_KEYS };
