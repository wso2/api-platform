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
const orgController = require('../controllers/orgContentController');
const apiController = require('../controllers/apiContentController');
const contentController = require('../controllers/customContentController');
const applicationController = require('../controllers/applicationsContentController');
const registerPartials = require('../middlewares/registerPartials');
const authController = require('../controllers/authController');
const settingsController = require('../controllers//settingsController');


router.get('/views/:viewName', registerPartials, orgController.loadOrganizationContent);

router.get('/views/:viewName/apis', registerPartials, apiController.loadAPIs);

router.get('/views/:viewName/api/:apiHandle', registerPartials, apiController.loadAPIContent);

router.get('/views/:viewName/api/:apiHandle/docs/specification', registerPartials, apiController.loadDocument);

//router.get('/views/:viewName/api/:apiHandle/docs', registerPartials, apiController.loadDocsPage);

router.get('/views/:viewName/api/:apiHandle/docs/:docType/:docName', registerPartials, apiController.loadDocument);


router.get('/views/:viewName/applications', registerPartials, applicationController.loadApplications);
router.get('/views/:viewName/applications/:applicationid', registerPartials, applicationController.loadApplication);

router.get('/configure', registerPartials, settingsController.loadSettingPage);

router.get('/login', registerPartials, authController.login);
router.get('/callback', registerPartials, authController.handleCallback);
router.get('/logout', registerPartials, authController.handleLogOut);
router.get('/signup', registerPartials, authController.handleSignUp);

// eslint-disable-next-line no-useless-escape
// Exclude specific paths
router.get(['/favicon.ico', '/images/*', '/styles/*', '/login*', '/devportal/*', '/views/*'], (req, res) => {
    res.status(404).send('Not found');
});
  
router.get('/:path/*', registerPartials, contentController.loadCustomContent);

module.exports = router;
