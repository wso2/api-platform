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
 * Server-side companion to the Stoplight Elements "Try It" panel.
 *
 * Elements issues the try-it call from the browser straight at the API's server
 * URL, which is a different origin from the portal — so the browser preflights
 * it and, unless the gateway happens to return CORS headers naming this portal,
 * discards the response. Rather than requiring every gateway and every backend
 * to trust the portal origin, Elements is pointed at this proxy
 * (`tryItCorsProxy`), which makes the call server-side and returns it
 * same-origin.
 *
 * That makes this module an outbound-request sink driven by a browser-supplied
 * URL, i.e. an SSRF surface. Two independent controls apply:
 *
 *   1. The target must match one of the endpoints registered for *that* API in
 *      the portal's own database (production/sandbox). The caller cannot name a
 *      host — it can only pick among URLs the portal already publishes in the
 *      spec's `servers` block. This is the primary control.
 *   2. Dial-time IP checking via ssrfGuard, as defense in depth against an API
 *      registered (or later re-resolving) to a metadata/link-local address.
 */

const axios = require('axios');
const http = require('node:http');
const https = require('node:https');

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const apiMetadataService = require('./apiMetadataService');
const { createGuardedLookup, assertAllowedScheme } = require('../utils/ssrfGuard');

function tryoutConfig() {
    return config.tryout || {};
}

// Request headers that must never be forwarded upstream. `cookie` is the
// important one: the browser attaches the portal session cookie to this
// same-origin request, and handing it to a third-party API endpoint would leak
// the session. `origin`/`referer` are dropped so the gateway sees a plain
// server-to-server call rather than a browser one (and cannot make its own CORS
// decision based on a portal origin that is no longer relevant).
const STRIPPED_REQUEST_HEADERS = new Set([
    'host', 'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization',
    'te', 'trailer', 'transfer-encoding', 'upgrade', 'content-length',
    'cookie', 'origin', 'referer', 'x-csrf-token', 'csrf-token',
    'sec-fetch-site', 'sec-fetch-mode', 'sec-fetch-dest', 'sec-ch-ua',
    'sec-ch-ua-mobile', 'sec-ch-ua-platform',
]);

// Response headers that must never be relayed back. Beyond hop-by-hop, this
// drops `set-cookie` (an upstream must not be able to set cookies on the portal
// origin) and the infrastructure-identifying headers called out in
// js-error-handling.md. `content-encoding`/`content-length` go too because axios
// has already decompressed the body — relaying the original values would
// describe bytes we are no longer sending.
const STRIPPED_RESPONSE_HEADERS = new Set([
    'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization',
    'te', 'trailer', 'transfer-encoding', 'upgrade',
    'content-encoding', 'content-length', 'set-cookie',
    'x-powered-by', 'server', 'via',
]);

const LEAKY_RESPONSE_HEADER_PREFIXES = ['x-amz-', 'x-amzn-', 'x-vercel-', 'cf-', 'x-azure-', 'x-goog-'];

let cachedClient = null;

/**
 * axios instance used for every proxied call: dial-time destination checking,
 * no automatic redirect following, and hard timeout/size ceilings.
 */
function getClient() {
    if (cachedClient) return cachedClient;

    const cfg = tryoutConfig();
    const lookup = createGuardedLookup({ allowPrivate: cfg.allowPrivateEndpoints !== false });

    cachedClient = axios.create({
        timeout: cfg.timeoutMs || 15000,
        // No silent redirect-following: a 3xx is handed back to the browser as
        // the response, so a redirect cannot walk the request to a destination
        // that never passed the endpoint allowlist.
        maxRedirects: 0,
        maxContentLength: cfg.maxResponseBytes || 5242880,
        maxBodyLength: cfg.maxRequestBytes || 1048576,
        responseType: 'arraybuffer',
        // Upstream 4xx/5xx are a legitimate try-it result, not a proxy failure.
        validateStatus: () => true,
        httpAgent: new http.Agent({ lookup }),
        httpsAgent: new https.Agent({ lookup, rejectUnauthorized: !cfg.tlsSkipVerify }),
    });
    return cachedClient;
}

/**
 * The endpoints registered for this API — exactly the URLs that
 * replaceEndpointParams() publishes as the spec's `servers` entries, so the
 * allowlist cannot drift from what the try-it panel is able to call.
 *
 * @returns {Promise<string[]>}
 */
async function resolveAllowedEndpoints(orgName, apiHandle) {
    const orgId = await orgDao.getId(orgName);
    const apiId = await apiDao.getId(orgId, apiHandle);
    const metaData = await apiMetadataService.getMetadataFromDB(orgId, apiId);
    if (!metaData) {
        return [];
    }
    const endPoints = metaData.endPoints || {};
    return [endPoints.productionURL, endPoints.sandboxURL]
        .filter((url) => typeof url === 'string' && url.trim().length > 0)
        .map((url) => url.trim());
}

function normalizedPort(parsed) {
    if (parsed.port) return parsed.port;
    return parsed.protocol === 'https:' ? '443' : '80';
}

// Path containment: '/pizza/1.0' must cover '/pizza/1.0/menu' but not
// '/pizza/1.0-internal'. Comparing with an explicit separator is what makes the
// difference — a bare startsWith() would accept the second.
function pathContains(basePath, targetPath) {
    const base = basePath.replace(/\/+$/, '');
    if (base === '') return true;
    return targetPath === base || targetPath.startsWith(`${base}/`);
}

/**
 * Resolve `rawTarget` against the API's registered endpoints.
 *
 * @param {string} rawTarget            absolute URL taken from the proxy path
 * @param {string[]} allowedEndpoints
 * @returns {URL} the validated target
 * @throws  {Error} with .statusCode when the target is not permitted
 */
function selectAllowedTarget(rawTarget, allowedEndpoints) {
    const cfg = tryoutConfig();
    assertAllowedScheme(rawTarget, { allowHttp: cfg.allowHttpEndpoints !== false });

    const target = new URL(rawTarget);

    // No legitimate try-it target carries credentials, and userinfo is exactly
    // the shape that makes "https://gateway.example.com@attacker.example" read
    // as an allowlisted host to a naive comparison.
    if (target.username || target.password) {
        throw Object.assign(new Error('Target URL must not contain credentials'), { statusCode: 422 });
    }

    for (const endpoint of allowedEndpoints) {
        let allowed;
        try {
            allowed = new URL(endpoint);
        } catch {
            logger.warn('Skipping unparseable registered endpoint during try-it validation');
            continue;
        }
        if (
            allowed.protocol === target.protocol &&
            allowed.hostname.toLowerCase() === target.hostname.toLowerCase() &&
            normalizedPort(allowed) === normalizedPort(target) &&
            pathContains(allowed.pathname, target.pathname)
        ) {
            return target;
        }
    }

    throw Object.assign(new Error('Target URL is not a registered endpoint for this API'), { statusCode: 422 });
}

function forwardableRequestHeaders(headers) {
    const out = {};
    for (const [name, value] of Object.entries(headers || {})) {
        const lower = name.toLowerCase();
        if (STRIPPED_REQUEST_HEADERS.has(lower)) continue;
        if (lower.startsWith('x-forwarded-')) continue;
        out[name] = value;
    }
    return out;
}

function relayableResponseHeaders(headers) {
    const out = {};
    for (const [name, value] of Object.entries(headers || {})) {
        const lower = name.toLowerCase();
        if (STRIPPED_RESPONSE_HEADERS.has(lower)) continue;
        if (LEAKY_RESPONSE_HEADER_PREFIXES.some((prefix) => lower.startsWith(prefix))) continue;
        out[name] = value;
    }
    return out;
}

/**
 * Validate and perform a proxied try-it call.
 *
 * @param {object} params
 * @param {string} params.orgName
 * @param {string} params.apiHandle
 * @param {string} params.rawTarget   absolute target URL
 * @param {string} params.method
 * @param {object} params.headers     inbound request headers
 * @param {Buffer} [params.body]
 * @returns {Promise<{status:number, headers:object, body:Buffer}>}
 */
async function proxyTryout({ orgName, apiHandle, rawTarget, method, headers, body }) {
    const allowedEndpoints = await resolveAllowedEndpoints(orgName, apiHandle);
    if (allowedEndpoints.length === 0) {
        throw Object.assign(new Error('API has no registered endpoints'), { statusCode: 422 });
    }

    const target = selectAllowedTarget(rawTarget, allowedEndpoints);

    // JS-AUTH-006: normalize before use so a lowercased method from any caller
    // reaches the upstream in the canonical form its routes match on.
    const upstreamMethod = String(method || 'GET').toUpperCase();

    const hasBody = body && body.length > 0 && !['GET', 'HEAD'].includes(upstreamMethod);

    const response = await getClient().request({
        url: target.toString(),
        method: upstreamMethod,
        headers: forwardableRequestHeaders(headers),
        data: hasBody ? body : undefined,
    });

    // axios v1 returns an AxiosHeaders instance; toJSON() gives a plain object.
    const rawHeaders = typeof response.headers?.toJSON === 'function'
        ? response.headers.toJSON()
        : response.headers;

    return {
        status: response.status,
        headers: relayableResponseHeaders(rawHeaders),
        body: Buffer.from(response.data || ''),
    };
}

module.exports = {
    proxyTryout,
    // Exported for tests.
    resolveAllowedEndpoints,
    selectAllowedTarget,
};
