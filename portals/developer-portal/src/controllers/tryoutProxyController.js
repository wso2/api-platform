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

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const tryoutProxyService = require('../services/tryoutProxyService');

const PROXY_SEGMENT = '/tryout-proxy/';

/**
 * Extract the target URL from the raw request line.
 *
 * Elements' `tryItCorsProxy` builds its request URL by concatenating the proxy
 * base and the full target URL, so everything after the marker segment is the
 * target — including its query string. It is read off req.originalUrl rather
 * than req.params because Express decodes params, and a target whose path or
 * query carries percent-encoded characters must reach the upstream exactly as
 * the browser wrote it.
 */
function extractRawTarget(originalUrl) {
    const marker = originalUrl.indexOf(PROXY_SEGMENT);
    if (marker === -1) return '';
    return originalUrl.slice(marker + PROXY_SEGMENT.length);
}

/**
 * Reject cross-site callers. The proxy is only ever driven by the try-it panel
 * on a portal page, so a request carrying another site's Origin has no business
 * here — this keeps the endpoint from being usable as a general-purpose proxy
 * by a third-party page riding the visitor's session.
 */
function isSameSiteRequest(req) {
    const site = req.headers['sec-fetch-site'];
    if (site) {
        return site === 'same-origin' || site === 'same-site' || site === 'none';
    }
    const origin = req.headers.origin;
    // Fail closed when neither signal is present. Every browser that can run the
    // try-it panel sends Sec-Fetch-Site, so an absent pair means the caller is
    // not the panel — and "no headers" must not be the one way to bypass the
    // check. A non-browser caller (curl, a test) is expected to set an explicit
    // same-origin Origin header.
    if (!origin) return false;
    try {
        return new URL(origin).host === req.get('host');
    } catch {
        return false;
    }
}

const proxyTryoutRequest = async (req, res) => {
    // Design mode serves APIs from sample files with no registered endpoints to
    // validate a target against, so the proxy has nothing to allowlist and stays off.
    if (config.tryout?.enabled === false || config.designMode?.enabled) {
        return res.status(404).json({ error: 'not_found' });
    }
    if (!isSameSiteRequest(req)) {
        logger.warn('Rejected cross-site try-it proxy request', { operation: 'proxyTryoutRequest' });
        return res.status(403).json({ error: 'forbidden', message: 'Request origin is not allowed.' });
    }

    const { orgName, apiHandle, apiType } = req.params;
    if (!['api', 'mcp'].includes(apiType)) {
        return res.status(404).json({ error: 'not_found' });
    }

    const rawTarget = extractRawTarget(req.originalUrl);
    if (!rawTarget) {
        return res.status(400).json({ error: 'invalid_request', message: 'No target URL was supplied.' });
    }

    try {
        const upstream = await tryoutProxyService.proxyTryout({
            orgName,
            apiHandle,
            rawTarget,
            method: req.method,
            headers: req.headers,
            body: Buffer.isBuffer(req.body) ? req.body : undefined,
        });

        for (const [name, value] of Object.entries(upstream.headers)) {
            res.setHeader(name, value);
        }
        res.setHeader('X-Content-Type-Options', 'nosniff');
        return res.status(upstream.status).send(upstream.body);
    } catch (err) {
        // The concrete reason (which endpoint failed to match, what a hostname
        // resolved to) stays in the log — echoing it back would help map the
        // internal network from a page anyone can open.
        logger.warn('Try-it proxy request rejected', {
            reason: err.message,
            code: err.code,
            orgName,
            apiHandle,
            operation: 'proxyTryoutRequest',
        });

        if (err.statusCode) {
            return res.status(err.statusCode).json({
                error: 'invalid_request',
                message: 'The requested target could not be called.',
            });
        }
        if (err.code === 'ECONNABORTED' || err.code === 'ETIMEDOUT') {
            return res.status(504).json({ error: 'gateway_timeout', message: 'The API endpoint did not respond in time.' });
        }
        if (err.code === 'ERR_FR_MAX_CONTENT_LENGTH_EXCEEDED') {
            return res.status(502).json({ error: 'bad_gateway', message: 'The API response exceeded the maximum allowed size.' });
        }
        return res.status(502).json({ error: 'bad_gateway', message: 'The API endpoint could not be reached.' });
    }
};

module.exports = {
    proxyTryoutRequest,
    // Exported for tests.
    extractRawTarget,
};
