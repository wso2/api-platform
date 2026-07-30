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
const registerPartials = require('../../middlewares/registerPartials');
const authController = require('../../controllers/authController');
const constants = require('../../utils/constants');
const orgContext = require('../../utils/orgContext');
const { attachOrgGuard } = require('../../middlewares/orgGuard');

// Pin every ':orgName' in this router to the organization this instance serves;
// anything else is a 404 before the route's own handlers run.
attachOrgGuard(router);

router.get('/:orgName/views/:viewName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico' || req.params.orgName === 'images' || req.params.orgName === 'portal' || req.params.orgName === '__dev_reload') {
        return res.status(404).send('Not Found');
    }
    next();
}, authController.handleSilentSSO, registerPartials, orgController.loadOrganizationContent);

router.get('/:orgName', (req, res, next) => {
    if (req.params.orgName === 'favicon.ico' || req.params.orgName === 'images' || req.params.orgName === 'portal' || req.params.orgName === '__dev_reload') {
        return res.status(404).send('Not Found');
    }
    return res.redirect(`${req.params.orgName}/views/default`);
}, authController.handleSilentSSO, registerPartials, orgController.loadOrganizationContent);

// The portal serves one organization, so the root is simply its front door —
// there is nothing to choose between. The handle is validated at config load, so
// this cannot redirect anywhere but into this instance's own portal.
router.get('/', (req, res) => {
    return res.redirect(`/${orgContext.getHandle()}${constants.ROUTE.VIEWS_PATH}default`);
});

module.exports = router;
