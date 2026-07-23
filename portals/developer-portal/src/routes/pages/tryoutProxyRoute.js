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

const express = require('express');
const router = express.Router();
const { config } = require('../../config/configLoader');
const logger = require('../../config/logger');
const util = require('../../utils/util');
const tryoutProxyController = require('../../controllers/tryoutProxyController');

// Mounted with `use` rather than a wildcard route: the target URL is appended
// to this path by Stoplight Elements ("…/tryout-proxy/https://host/path?q=1"),
// so the tail is an arbitrary URL rather than a well-formed path, and prefix
// mounting matches it without depending on how path-to-regexp treats the "//"
// in the scheme. Every HTTP method is accepted — try-it can issue any of them.
// The body is relayed byte-for-byte whatever its content type, so it is read as
// a raw Buffer. app.js deliberately skips its JSON/urlencoded parsers for this
// path — once those consume the stream the original bytes are gone.
const rawBody = express.raw({ type: '*/*', limit: config.tryout?.maxRequestBytes || 1048576 });

// Body-parser failures are answered here rather than falling through to app.js's
// central handler, which renders an HTML error page — the try-it panel issues
// this as a fetch and expects the same JSON error shape the controller returns.
function handleRawBodyError(req, res, next) {
    rawBody(req, res, (err) => {
        if (!err) return next();
        if (err.type === 'entity.too.large' || err.status === 413 || err.statusCode === 413) {
            logger.warn('Try-it proxy request body exceeded the configured limit', {
                operation: 'proxyTryoutRequest',
            });
            // The configured ceiling is deliberately not echoed back.
            return res.status(413).json({
                error: 'payload_too_large',
                message: 'Request body exceeds the maximum allowed size.',
            });
        }
        logger.warn('Try-it proxy request body could not be read', {
            reason: err.message,
            operation: 'proxyTryoutRequest',
        });
        return res.status(400).json({
            error: 'invalid_request',
            message: 'Request body could not be read.',
        });
    });
}

// The same portal-mode gate the spec page carries (apiContentRoute.js): an
// APIs-only or MCP-only portal 404s the page for the excluded type, and the
// proxy must not stay reachable for a type the portal doesn't serve. Wrapped
// because enforcePortalMode signals rejection with next(err), which app.js
// renders as an HTML error page — this endpoint answers a fetch, so its
// rejections stay JSON like every other response here.
async function enforcePortalModeJson(req, res, next) {
    try {
        await util.enforcePortalMode(req, res, (err) => {
            if (err) return res.status(404).json({ error: 'not_found' });
            return next();
        });
    } catch (err) {
        logger.warn('Try-it proxy portal-mode check failed', {
            reason: err.message,
            operation: 'proxyTryoutRequest',
        });
        return res.status(404).json({ error: 'not_found' });
    }
}

router.use(
    '/:orgName/views/:viewName/:apiType/:apiHandle/tryout-proxy',
    handleRawBodyError,
    enforcePortalModeJson,
    tryoutProxyController.proxyTryoutRequest
);

module.exports = router;
