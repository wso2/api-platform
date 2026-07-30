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
const { ensureAuthenticated } = require('../../middlewares/ensureAuthenticated');
const tryoutProxyController = require('../../controllers/tryoutProxyController');
const { attachOrgGuard } = require('../../middlewares/orgGuard');

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

// The same artifact-type gate the spec page carries (apiContentRoute.js): a portal that
// doesn't serve this artifact type 404s the page for it, and the proxy must not
// stay reachable for a type the portal doesn't serve. Wrapped because
// requireArtifactTypeFromPath signals rejection with next(err), which app.js renders
// as an HTML error page — this endpoint answers a fetch, so its rejections stay
// JSON like every other response here.
async function requireArtifactTypeFromPathJson(req, res, next) {
    try {
        await util.requireArtifactTypeFromPath(req, res, (err) => {
            if (err) return res.status(404).json({ error: 'not_found' });
            return next();
        });
    } catch (err) {
        logger.warn('Try-it proxy artifact-type check failed', {
            reason: err.message,
            operation: 'proxyTryoutRequest',
        });
        return res.status(404).json({ error: 'not_found' });
    }
}

// The specification page's own access gate (ensureAuthenticated), so the proxy
// is reachable exactly when that page is — including when a deployer has added
// the page to pageAccessRules.authenticated/authorized, which the proxy would
// otherwise ignore.
//
// The gate is evaluated against the PAGE path (…/{apiType}/{apiHandle}), declared
// via req.accessControlPath: the page-access globs are written against page URLs,
// and this request's own path carries an appended target URL whose encoding would
// otherwise trip ensureAuthenticated's encoded-separator rejection.
// ensureAuthenticated answers a browser navigating to a page: it redirects to the
// login screen when the caller isn't authenticated, and hands 403s to app.js's
// central handler, which renders an HTML error page. The try-it panel fetches
// this endpoint, so both outcomes are answered here instead, with the one generic
// credential failure this endpoint returns everywhere else — same status and body
// whichever way the gate refused (js-error-handling.md directive 4).
function unauthorizedJson(res) {
    return res.status(401).json({
        error: 'unauthorized',
        message: 'Invalid or expired credentials.',
    });
}

function authenticateLikeSpecPage(req, res, next) {
    const { orgName, viewName, apiType, apiHandle } = req.params;
    req.accessControlPath = `/${orgName}/views/${viewName}/${apiType}/${apiHandle}`;

    // The login redirect is issued via res.redirect and never reaches next(), so
    // it is intercepted here. Swapped only for the duration of the gate.
    const originalRedirect = res.redirect;
    let redirected = false;
    res.redirect = () => {
        redirected = true;
        res.redirect = originalRedirect;
        logger.warn('Try-it proxy request rejected: caller is not authenticated for this page', {
            operation: 'proxyTryoutRequest',
        });
        return unauthorizedJson(res);
    };

    return ensureAuthenticated(req, res, (err) => {
        if (!redirected) res.redirect = originalRedirect;
        if (err) {
            logger.warn('Try-it proxy request rejected by the page access gate', {
                status: err.status,
                operation: 'proxyTryoutRequest',
            });
            return unauthorizedJson(res);
        }
        return next();
    });
}

// Pin every ':orgName' in this router to the organization this instance serves;
// anything else is a 404 before the route's own handlers run. Answered as JSON for
// the same reason unauthorizedJson exists above: the try-it panel fetches this
// endpoint, so app.js's HTML error page would arrive where JSON is expected. A 500
// stays generic — the reason is logged inside the guard.
attachOrgGuard(router, 'orgName', (res, err) => res.status(err.status).json({
    error: err.status === 404 ? 'not_found' : 'internal_error',
    message: err.status === 404
        ? 'The requested resource was not found.'
        : 'An unexpected error occurred.',
}));

router.use(
    '/:orgName/views/:viewName/:apiType/:apiHandle/tryout-proxy',
    handleRawBodyError,
    requireArtifactTypeFromPathJson,
    authenticateLikeSpecPage,
    tryoutProxyController.proxyTryoutRequest
);

// Requests this router handles relay their body verbatim, so app.js must not let
// a JSON/urlencoded parser consume the stream first. The pattern lives here, next
// to the mount path it mirrors, so the two cannot drift apart.
const BODY_PARSER_SKIP_PATTERN = /^\/[^/]+\/views\/[^/]+\/[^/]+\/[^/]+\/tryout-proxy(\/|$)/;

module.exports = { router, BODY_PARSER_SKIP_PATTERN };
