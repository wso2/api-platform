/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const apiController = require('../../controllers/apiContentController');
const apiKeysContentController = require('../../controllers/apiKeysPageController');
const registerPartials = require('../../middlewares/registerPartials');
const { ensureAuthenticated } = require('../../middlewares/ensureAuthenticated');
const authController = require('../../controllers/authController');
const util = require('../../utils/util');
const { attachOrgGuard } = require('../../middlewares/orgGuard');

// Pin every ':orgName' in this router to the organization this instance serves;
// anything else is a 404 before the route's own handlers run.
attachOrgGuard(router);

router.get('/:orgName/views/:viewName/llms.txt', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, apiController.loadLlmsTxt);

router.get('/:orgName/views/:viewName/apis.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.requireArtifactType('apis'), apiController.loadAPIsMd);

router.get('/:orgName/views/:viewName/mcps.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.requireArtifactType('mcp-servers'), apiController.loadMCPsMd);

router.get('/:orgName/views/:viewName/apis', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('apis'), ensureAuthenticated, apiController.loadAPIs);

router.get('/:orgName/views/:viewName/mcps', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, util.requireArtifactType('mcp-servers'), apiController.loadAPIs);

router.get('/:orgName/views/:viewName/api/:apiHandle.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.requireArtifactType('apis'), apiController.loadAPIContentMd);

router.get('/:orgName/views/:viewName/mcp/:apiHandle.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.requireArtifactType('mcp-servers'), apiController.loadAPIContentMd);

router.get('/:orgName/views/:viewName/api/:apiHandle', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('apis'), ensureAuthenticated, apiController.loadAPIContent);

router.get('/:orgName/views/:viewName/mcp/:apiHandle', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, util.requireArtifactType('mcp-servers'), apiController.loadAPIContent);


router.get('/:orgName/views/:viewName/api/:apiHandle/api-keys', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('apis'), ensureAuthenticated, apiKeysContentController.loadAPIApiKeys);

// Express 5 / path-to-regexp v8 dropped inline regex params (`:apiType(api|mcp)`,
// `:format(json|graphql|xml)`), so the params are unconstrained here and validated in
// the guard below — an invalid value calls next('route') to fall through as before.
router.get('/:orgName/views/:viewName/:apiType/:apiHandle/docs/specification.:format', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    if (!['api', 'mcp'].includes(req.params.apiType) || !['json', 'graphql', 'xml'].includes(req.params.format)) {
        return next('route');
    }
    next();
  }, registerPartials, util.requireArtifactTypeFromPath, ensureAuthenticated, apiController.loadSpecificationRaw);

router.get('/:orgName/views/:viewName/api/:apiHandle/docs/specification', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('apis'), ensureAuthenticated, apiController.loadDocument);

router.get('/:orgName/views/:viewName/mcp/:apiHandle/docs/specification', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('mcp-servers'), ensureAuthenticated, apiController.loadDocument);

// Express 5: `:apiType(api|mcp)` inline regex dropped — validated in the guard below.
router.get('/:orgName/views/:viewName/:apiType/:apiHandle/docs/:docType/:docName.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    if (!['api', 'mcp'].includes(req.params.apiType)) {
        return next('route');
    }
    next();
}, util.requireArtifactTypeFromPath, apiController.loadDocumentMd);

router.get('/:orgName/views/:viewName/api/:apiHandle/docs/:docType/:docName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('apis'), ensureAuthenticated, apiController.loadDocument);

router.get('/:orgName/views/:viewName/mcp/:apiHandle/docs/:docType/:docName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.requireArtifactType('mcp-servers'), ensureAuthenticated, apiController.loadDocument);

module.exports = router;