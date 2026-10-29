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
/*
 * The Usage page: the billable usage of the signed-in subscriber's paid subscriptions. The page is
 * sent without the usage; its script reads it from GET /usage.
 */
const { renderTemplateWithView, pageErrorStatus } = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const { pageAccess } = require('../utils/pageAccess');

const loadUsage = async (req, res, next) => {
    const { orgName, viewName } = req.params;
    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;
        if (!req.user) {
            return res.redirect(`${constants.ROUTE.BASE_PATH}/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        if (!(await pageAccess(req)).readUsage) {
            // The page reads usage through the REST API, which this user may not call.
            const denied = new Error('Access Denied');
            denied.status = 403;
            return next(denied);
        }
        const templateContent = {
            baseUrl: constants.ROUTE.BASE_PATH + '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: {
                firstName: req.user.firstName,
                lastName: req.user.lastName,
                email: req.user.email,
                imageURL: req.user.picture || req.user.imageURL || constants.DEFAULT_PROFILE_IMAGE_URL,
                isAdmin: req.user.isAdmin && !req.foreignOrgSession,
            },
            orgId,
        };
        const html = await renderTemplateWithView('../pages/usage/page.hbs', './src/defaultContent/layout/main.hbs',
            templateContent, true, orgId, viewName);
        return res.send(html);
    } catch (error) {
        logger.error('Error loading usage page', { error: error.message, orgName });
        error.status = pageErrorStatus(error);
        return next(error);
    }
};

module.exports = { loadUsage };
