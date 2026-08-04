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
const orgController = require('../../controllers/orgContentController');
const apiController = require('../../controllers/apiContentController');
const applicationController = require('../../controllers/applicationsContentController');
const contentController = require('../../controllers/customContentController');
const registerPartials = require('../../middlewares/registerPartials');

router.get('/views/:viewName', registerPartials, orgController.loadOrganizationContent);

router.get('/views/:viewName/apis', registerPartials, apiController.loadAPIs);
router.get('/views/:viewName/mcps', registerPartials, apiController.loadAPIs);

router.get('/views/:viewName/api/:apiHandle', registerPartials, apiController.loadAPIContent);
router.get('/views/:viewName/mcp/:apiHandle', registerPartials, apiController.loadAPIContent);

router.get('/views/:viewName/api/:apiHandle/docs', registerPartials, apiController.loadDocsPage);
router.get('/views/:viewName/mcp/:apiHandle/docs', registerPartials, apiController.loadDocsPage);

router.get('/views/:viewName/api/:apiHandle/docs/specification', registerPartials, apiController.loadDocument);
router.get('/views/:viewName/mcp/:apiHandle/docs/specification', registerPartials, apiController.loadDocument);

router.get('/views/:viewName/api/:apiHandle/docs/:docType/:docName', registerPartials, apiController.loadDocument);
router.get('/views/:viewName/mcp/:apiHandle/docs/:docType/:docName', registerPartials, apiController.loadDocument);

router.get('/views/:viewName/applications', registerPartials, applicationController.loadApplications);

// No reserved-path denylist: a root path that doesn't match one of the specific
// /views/:viewName routes above falls through to the catch-all below, where
// loadCustomContent resolves no orgName/viewName param and answers a clean 404 — so
// login/images/styles/settings paths (all disabled in design mode) 404 without an
// explicit exclusion list.

// Trailing `*` -> named wildcard `*splat`; handler reads req.originalUrl, param unused.
router.get('/:path/*splat', registerPartials, contentController.loadCustomContent);

module.exports = router;
