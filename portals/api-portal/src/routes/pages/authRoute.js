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
const authController = require('../../controllers/authController');
const registerPartials = require('../../middlewares/registerPartials');
const { attachOrgGuard } = require('../../middlewares/orgGuard');
const constants = require('../../utils/constants');
const orgContext = require('../../utils/orgContext');

// Pin every ':orgName' in this router to the organization this instance serves;
// anything else is a 404 before the route's own handlers run.
attachOrgGuard(router);

// router.get('/portal/login', registerPartials, authController.login);
// router.get('/portal/callback', authController.handleCallback);
// router.get('/portal/logout', authController.handleLogOut);
// router.get('/portal/signup', authController.handleSignUp);

router.get('/:orgName/views/:viewName/login', registerPartials, authController.login);
router.post('/:orgName/views/:viewName/login', authController.handleLocalLogin);
router.get('/:orgName/callback', authController.handleCallback);
router.get('/signin', authController.handleCallback);
router.get('/:orgName/views/:viewName/logout', authController.handleLogOut);
router.get('/:orgName/views/:viewName/signup', authController.handleSignUp);
router.get('/logout', authController.handleLogOutLanding);

// Generic portal login entry point. Three failure paths have no view segment to build a
// view-scoped login URL from — passport's failureRedirect (authController.handleCallback),
// the no-orgName branch of ensureAuthenticated, and handleLocalLogin when the org/view
// segments fail their safe-handle check — and all three target this path. Without it they
// dead-end in a 404 instead of the login page.
//
// The view is resolved rather than hardcoded to 'default', for the same reason the org
// front door resolves it: that view can be renamed or deleted. Only the `error` parameter
// is carried through, re-encoded — the callers that set it generate the message
// themselves, and forwarding the raw query would reflect arbitrary input into a redirect.
router.get('/login', async (req, res) => {
    const error = typeof req.query.error === 'string' ? req.query.error : '';
    const query = error ? `?error=${encodeURIComponent(error)}` : '';
    const viewHandle = await orgContext.getFallbackViewHandle();
    res.redirect(
        `${constants.ROUTE.BASE_PATH}/${orgContext.getHandle()}${constants.ROUTE.VIEWS_PATH}${viewHandle}/login${query}`
    );
});




module.exports = router;

