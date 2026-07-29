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

const crypto = require('crypto');

// Default cap on a generated handle. Handle columns are VARCHAR(255), so this leaves
// room for the disambiguating suffix appended on collision.
const DEFAULT_MAX_HANDLE_LENGTH = 200;

/**
 * Derive a URL-safe handle from a human display name.
 *
 * Returns an empty string when the name contains nothing slugifiable (all punctuation,
 * or a script with no a-z0-9 characters) — callers are expected to substitute their own
 * fallback base rather than persisting an empty handle.
 *
 * @param {string} displayName
 * @param {number} [maxLength]
 * @returns {string} lowercase [a-z0-9-] slug, never leading/trailing '-'
 */
function slugifyHandle(displayName, maxLength = DEFAULT_MAX_HANDLE_LENGTH) {
    return String(displayName == null ? '' : displayName)
        .toLowerCase()
        .trim()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .slice(0, maxLength)
        // slice() can cut mid-separator and leave a trailing dash.
        .replace(/-+$/, '');
}

/**
 * The nth candidate handle for a base: the base itself first, then "-2", "-3", ...
 * so the first subscriber of a given name gets the clean handle.
 *
 * @param {string} base
 * @param {number} attempt zero-based
 * @returns {string}
 */
function handleCandidate(base, attempt) {
    return attempt === 0 ? base : `${base}-${attempt + 1}`;
}

/**
 * A plain UUID handle, for when there is no usable slug base (the name doesn't
 * slugify to anything) or the readable numeric ladder has been exhausted.
 *
 * A bare UUID rather than a base-prefixed random tail: once we're past the numeric
 * suffixes the handle is no longer meaningfully readable anyway, so there's nothing
 * to gain from keeping the base — and a UUID is unambiguous and collision-proof on
 * its own. crypto.randomUUID (not Math.random) so the uniqueness is a real property,
 * not a hope.
 *
 * @returns {string}
 */
function uuidHandle() {
    return crypto.randomUUID();
}

module.exports = {
    slugifyHandle,
    handleCandidate,
    uuidHandle,
    DEFAULT_MAX_HANDLE_LENGTH,
};
