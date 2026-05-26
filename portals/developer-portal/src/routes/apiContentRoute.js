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
const apiController = require('../controllers/apiContentController');
const subscriptionsController = require('../controllers/subscriptionsContentController');
const platformApiKeysContentController = require('../controllers/platformApiKeysContentController');
const registerPartials = require('../middlewares/registerPartials');
const { ensureAuthenticated } = require('../middlewares/ensureAuthenticated');
const authController = require('../controllers/authController');
const util = require('../utils/util');

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
}, util.enforcePortalMode, apiController.loadAPIsMd);

router.get('/:orgName/views/:viewName/mcps.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.enforcePortalMode, apiController.loadMCPsMd);

router.get('/:orgName/views/:viewName/apis', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadAPIs);

router.get('/:orgName/views/:viewName/mcps', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, util.enforcePortalMode, apiController.loadAPIs);

router.get('/:orgName/views/:viewName/api/:apiHandle.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.enforcePortalMode, apiController.loadAPIContentMd);

router.get('/:orgName/views/:viewName/mcp/:apiHandle.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.enforcePortalMode, apiController.loadAPIContentMd);

router.get('/:orgName/views/:viewName/api/:apiHandle', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadAPIContent);

router.get('/:orgName/views/:viewName/mcp/:apiHandle', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, registerPartials, ensureAuthenticated, util.enforcePortalMode, apiController.loadAPIContent);

router.get('/:orgName/views/:viewName/api/:apiHandle/subscriptions', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, subscriptionsController.loadAPISubscriptions);

router.get('/:orgName/views/:viewName/api/:apiHandle/api-keys', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, platformApiKeysContentController.loadAPIPlatformApiKeys);

router.get('/:orgName/views/:viewName/:apiType(api|mcp)/:apiHandle/docs/specification.:format(json|graphql|xml)', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
  }, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadSpecificationRaw);

router.get('/:orgName/views/:viewName/api/:apiHandle/docs/specification', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadDocument);

router.get('/:orgName/views/:viewName/mcp/:apiHandle/docs/specification', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadDocument);

router.get('/:orgName/views/:viewName/:apiType(api|mcp)/:apiHandle/docs/:docType/:docName.md', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, util.enforcePortalMode, apiController.loadDocumentMd);

router.get('/:orgName/views/:viewName/api/:apiHandle/docs/:docType/:docName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadDocument);

router.get('/:orgName/views/:viewName/mcp/:apiHandle/docs/:docType/:docName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, util.enforcePortalMode, ensureAuthenticated, apiController.loadDocument);

module.exports = router;