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
/* eslint-disable no-undef */
const path = require('path');
const fs = require('fs');
const Handlebars = require('handlebars');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const { renderTemplate, renderTemplateFromAPI } = require('../utils/util');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');


const loadOrganizationContent = async (req, res, next) => {

    let html = "";
    if (config.designMode?.enabled) {
        html = await loadOrgContentFromFile(req, res);
    } else {
        html = await loadOrgContentFromAPI(req, res, next);
    }
    res.send(html);
}
const loadDefaultLandingPage = async (req, res) => {

    let html = "";
    const completeTemplatePath = path.join(require.main.filename, '../pages/default-home/page.hbs');
    const templateResponse = await fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
    const template = await Handlebars.compile(templateResponse);
    html = template();

    res.send(html);
}
const loadOrgContentFromFile = async (req, res) => {

    const layoutPath = config.designMode.pathToLayout;
    const templateContent = {
        userProfiles: [],
        baseUrl: config.server.baseUrl + constants.ROUTE.VIEWS_PATH + req.params.viewName,
        devMode: true,
    };

    return renderTemplate(layoutPath + 'pages/home/page.hbs', layoutPath + 'layout/main.hbs', templateContent, false)
}

const loadOrgContentFromAPI = async (req, res, next) => {
    let html;
    const orgName = req.params.orgName;
    const orgDetails = await orgDao.get(orgName);
    const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    try {
        const orgId = await orgDao.getId(orgName);
        let profile = null;
        if (req.user) {
            profile = {
                imageURL: req.user.imageURL,
                firstName: req.user.firstName,
                lastName: req.user.lastName,
                email: req.user.email,
                isAdmin: req.user.isAdmin,
            }
        }
        templateContent = {
            devportalMode: devportalMode,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
            profile: req.isAuthenticated() ? profile : null,
        };
        html = await renderTemplateFromAPI(templateContent, orgId, orgName, 'pages/home', req.params.viewName);
    } catch (error) {
        logger.error(`Failed to load organization`, {
            orgName: req.params?.orgName,
            error: error.message,
            stack: error.stack
        });
        error.status = 500;
        return next(error);
    }
    return html;
}

module.exports = {
    loadOrgContentFromFile,
    loadOrgContentFromAPI,
    loadOrganizationContent,
    loadDefaultLandingPage
};
