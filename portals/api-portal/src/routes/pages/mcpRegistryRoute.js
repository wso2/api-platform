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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
const express = require('express');
const router = express.Router({ mergeParams: true });
const mcpRegistryService = require('../../services/mcpRegistryService');
const { enforceSecurity } = require('../../middlewares/ensureAuthenticated');
const { orgGuardMiddleware } = require('../../middlewares/orgGuard');
const constants = require('../../utils/constants');

router.use((req, res, next) => {
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET');
    if (req.method === 'OPTIONS') {
        return res.sendStatus(204);
    }
    next();
});

// ':orgHandle' is captured by this router's MOUNT paths in app.js
// ('/registry/:orgHandle' and '/:orgHandle/registry'), not by its own route paths,
// so a router.param() registered here would never fire for it — the middleware form
// reads the same mergeParams value the service handlers below use. Runs after CORS
// so a rejected cross-organization request still answers preflight consistently.
// The rejection is answered here rather than via next(): every other response from
// this router is JSON (mcpRegistryService.sendError's { error } shape), and its
// callers are MCP clients, so falling through to app.js's HTML error page would hand
// a program a page. A 500 stays generic — the reason is logged inside the guard.
router.use(orgGuardMiddleware('orgHandle', (res, err) => res.status(err.status).json({
    error: err.status === 404 ? 'Not Found' : 'Internal Server Error',
})));

// Discovery endpoints (public)
router.get('/v0.1/servers', mcpRegistryService.listServers);
router.get('/v0.1/servers/:serverName/versions', mcpRegistryService.listVersions);
router.get('/v0.1/servers/:serverName/versions/:version', mcpRegistryService.getVersion);

// Publishing endpoints — gated by the same dp:mcp_* scopes the admin /api/v0.9/mcp-servers*
// CRUD operations require (see constants.SCOPES.MCP_*), via bearer JWT or local-auth session
// (enforceSecurity), same as every other API Portal write route. Status changes have no
// dedicated admin-API scope of their own — per the OpenAPI spec, they go through the same
// PUT /mcp-servers/{id} operation as a plain update, so they're gated identically here.
//
// publishServer is an upsert (create-or-update), and MCP_PUBLISH_SCOPES here is only a
// coarse pre-filter — proof the caller holds SOME MCP-publishing scope — not the final
// authorization decision: whether this specific request creates or updates isn't knowable
// until publishServer looks up the target name/version/proxyId, so mcpRegistryService.js's
// publishServer re-checks req.tokenScopes against the precise create-vs-update scope once
// it has determined which operation this request actually performs. A caller with only
// dp:mcp_server:create must not be able to update an existing server via this coarse gate alone,
// and vice versa for dp:mcp_server:update against creating a new one.
const MCP_PUBLISH_SCOPES = [...constants.SCOPES.MCP_CREATE, ...constants.SCOPES.MCP_UPDATE];
router.post('/v0.1/publish', enforceSecurity(MCP_PUBLISH_SCOPES), mcpRegistryService.publishServer);
router.put('/v0.1/servers/:serverName/versions/:version', enforceSecurity(constants.SCOPES.MCP_UPDATE), mcpRegistryService.updateVersion);
router.delete('/v0.1/servers/:serverName/versions/:version', enforceSecurity(constants.SCOPES.MCP_DELETE), mcpRegistryService.deleteVersion);
router.patch('/v0.1/servers/:serverName/versions/:version/status', enforceSecurity(constants.SCOPES.MCP_UPDATE), mcpRegistryService.updateVersionStatus);
router.patch('/v0.1/servers/:serverName/status', enforceSecurity(constants.SCOPES.MCP_UPDATE), mcpRegistryService.updateAllVersionsStatus);

module.exports = router;
