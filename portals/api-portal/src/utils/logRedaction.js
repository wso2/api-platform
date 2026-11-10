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
 * Keeps secrets out of logs. A field is sensitive when its name contains one of SENSITIVE_FIELDS
 * (case-insensitive), or, in a query string, when it is one of OAUTH_QUERY_FIELDS: an OAuth sign-in
 * callback carries its authorization code and state there.
 */

const crypto = require('crypto');

const SENSITIVE_FIELDS = ['password', 'token', 'secret', 'key', 'authorization', 'idToken', 'accessToken',
    'refreshToken', 'collectorApplicationId'];
const OAUTH_QUERY_FIELDS = ['code', 'state', 'session_state'];
const REDACTED = '[REDACTED]';

function isSensitiveField(name, fields = SENSITIVE_FIELDS) {
    const lower = String(name).toLowerCase();
    return fields.some((field) => lower.includes(field.toLowerCase()));
}

function isSensitiveQueryField(name, fields) {
    return isSensitiveField(name, fields) || OAUTH_QUERY_FIELDS.includes(String(name).toLowerCase());
}

/** An object (a request body) with the values of sensitive fields replaced, at any depth. */
function redactObject(obj, fields = SENSITIVE_FIELDS) {
    if (!obj || typeof obj !== 'object') return obj;
    if (Array.isArray(obj)) return obj.map((item) => redactObject(item, fields));
    const redacted = {};
    for (const [key, value] of Object.entries(obj)) {
        if (isSensitiveField(key, fields)) redacted[key] = REDACTED;
        else redacted[key] = typeof value === 'object' && value !== null ? redactObject(value, fields) : value;
    }
    return redacted;
}

/** A parsed query (req.query) with the values of sensitive fields replaced. */
function redactQuery(query, fields = SENSITIVE_FIELDS) {
    if (!query || typeof query !== 'object') return query;
    const redacted = {};
    for (const [key, value] of Object.entries(query)) {
        if (isSensitiveQueryField(key, fields)) redacted[key] = REDACTED;
        else redacted[key] = typeof value === 'object' && value !== null ? redactObject(value, fields) : value;
    }
    return redacted;
}

/** A URL or path with the values of its sensitive query fields replaced; everything else stays as written. */
function redactUrl(url, fields = SENSITIVE_FIELDS) {
    if (typeof url !== 'string') return url;
    const start = url.indexOf('?');
    if (start < 0) return url;
    const hash = url.indexOf('#', start);
    const end = hash < 0 ? url.length : hash;
    const params = url.slice(start + 1, end).split('&').map((pair) => {
        const eq = pair.indexOf('=');
        const rawName = eq < 0 ? pair : pair.slice(0, eq);
        let name = rawName;
        try { name = decodeURIComponent(rawName.replace(/\+/g, ' ')); } catch { /* keep it as written */ }
        return eq >= 0 && isSensitiveQueryField(name, fields) ? `${rawName}=${REDACTED}` : pair;
    });
    return url.slice(0, start + 1) + params.join('&') + url.slice(end);
}

/** A short, stable stand-in for a session id, so log lines can be matched up without recording the id. */
function sessionRef(sessionId) {
    if (!sessionId) return 'no-session';
    return crypto.createHash('sha256').update(String(sessionId)).digest('hex').slice(0, 16);
}

module.exports = {
    REDACTED,
    SENSITIVE_FIELDS,
    redactObject,
    redactQuery,
    redactUrl,
    sessionRef,
};
