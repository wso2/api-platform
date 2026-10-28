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
 * The Invoices page: the signed-in subscriber's invoices. The page is sent without the invoices; its
 * script reads GET /billing/invoices and opens the payment provider's billing page for payment
 * methods. The Billing menu item (/billing) opens this page.
 */
const { renderTemplateWithView, pageErrorStatus } = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const { pageAccess } = require('../utils/pageAccess');

const renderBillingPage = (template) => async (req, res, next) => {
    const { orgName, viewName } = req.params;
    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;
        if (!req.user) {
            return res.redirect(`${constants.ROUTE.BASE_PATH}/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        if (!(await pageAccess(req)).readInvoices) {
            // The page reads invoices through the REST API, which this user may not call.
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
        const html = await renderTemplateWithView(template, './src/defaultContent/layout/main.hbs',
            templateContent, true, orgId, viewName);
        return res.send(html);
    } catch (error) {
        logger.error('Error loading billing page', { error: error.message, orgName });
        error.status = pageErrorStatus(error);
        return next(error);
    }
};

const loadInvoices = renderBillingPage('../pages/billing-invoices/page.hbs');

/** GET /billing: opens Invoices, or Usage for a user who may read only usage. */
const redirectToInvoices = async (req, res) => {
    const { orgName, viewName } = req.params;
    const page = (await pageAccess(req)).readInvoices ? '/billing/invoices' : '/usage';
    return res.redirect(`${constants.ROUTE.BASE_PATH}/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}${page}`);
};

module.exports = { loadInvoices, redirectToInvoices };
