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
 * Pins the {orgHandle} segment of a page URL to the single organization this
 * instance serves (src/utils/orgContext.js).
 *
 * The database is shared across organizations, so without this the URL segment is
 * an unauthenticated selector for *any* organization's portal content: a visitor
 * could read another tenant's published APIs, docs, and theming by editing the
 * first path segment. Rejecting anything but the configured handle closes that.
 *
 * Registered through `router.param()` rather than a path-matching middleware on
 * purpose: the param callback receives the exact value the route handler will go
 * on to use, so there is no second, independent parse of the URL that could
 * disagree with the router's own (the failure mode behind encoded-traversal auth
 * bypasses). Express's router runs param callbacks for every matched layer —
 * `.use()` paths included — and only once per request per param name, so
 * registering the same guard on several routers is safe and doesn't re-run.
 */

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const orgContext = require('../utils/orgContext');
const { NotFoundError } = require('../utils/errors/customErrors');

/**
 * Express param callback: 404s unless `value` is this instance's organization
 * handle, and populates req.orgId with the resolved uuid when it is.
 *
 * Only the canonical handle is accepted. orgDao.get() would also resolve a display
 * name or idp_ref_id, but a single-organization portal has exactly one correct URL
 * for itself, and matching the handle alone keeps this off the database entirely
 * for the rejection case.
 *
 * @param {(res, err) => void} [onReject] how to answer a rejection. Omitted, the
 *   error goes to `next()` and app.js renders an HTML error page — right for the
 *   page routers, wrong for the JSON endpoints (MCP registry, try-it proxy) whose
 *   callers are programs. Those pass a responder that emits their own error shape.
 */
async function pinOrgParam(req, res, next, value, onReject) {
    // Design mode renders from disk and has no organization to pin — config
    // validation skips the handle check there, so there is nothing to compare to.
    if (config.designMode?.enabled) return next();

    if (String(value || '').toLowerCase() !== orgContext.getHandle()) {
        logger.warn('Rejected request for a non-local organization', {
            requested: value,
            operation: 'pinOrgParam',
        });
        const err = new Error('Not Found');
        err.status = 404;
        return onReject ? onReject(res, err) : next(err);
    }

    try {
        req.orgId = await orgContext.getOrgUuid();
    } catch (error) {
        // The handle matched config but no such row exists — the portal itself is
        // misconfigured (or not yet seeded), which is a server-side fault. Do not
        // report it as a 404: that would tell a visitor their URL was wrong when it
        // was correct, and redirecting to '/' (which points back here) would loop.
        const status = error instanceof NotFoundError ? 500 : (error.status || 500);
        logger.error('Configured organization could not be resolved', {
            handle: orgContext.getHandle(),
            error: error.message,
            operation: 'pinOrgParam',
        });
        const err = new Error('Internal Server Error');
        err.status = status;
        return onReject ? onReject(res, err) : next(err);
    }

    return next();
}

/**
 * Registers the guard for a router's org URL parameter.
 *
 * @param {import('express').Router} router
 * @param {string} [paramName] the router's own org param — 'orgName' for the page
 *   routers, which all spell it that way.
 * @param {(res, err) => void} [onReject] see pinOrgParam.
 */
function attachOrgGuard(router, paramName = 'orgName', onReject) {
    router.param(paramName, (req, res, next, value) => pinOrgParam(req, res, next, value, onReject));
    return router;
}

/**
 * Middleware form, for the one router whose org segment comes from its *mount*
 * path (app.js mounts mcpRegistryRoute under '/registry/:orgHandle' and
 * '/:orgHandle/registry'). Params captured by a mount path belong to the parent
 * router's layer, so a `router.param()` registered on the child never fires for
 * them; this reads the same mergeParams-supplied value the service handlers use.
 *
 * @param {string} [paramName]
 * @param {(res, err) => void} [onReject] see pinOrgParam.
 */
function orgGuardMiddleware(paramName = 'orgHandle', onReject) {
    return (req, res, next) => pinOrgParam(req, res, next, req.params[paramName], onReject);
}

module.exports = { attachOrgGuard, orgGuardMiddleware, pinOrgParam };
