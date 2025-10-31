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
const { trackHomePageVisit } = require('../utils/telemetry');
const config = require(process.cwd() + '/config.json');
const constants = require('../utils/constants');
const adminDao = require('../dao/admin');

const filePrefix = config.pathToContent;
const baseURLDev = config.baseUrl + constants.ROUTE.VIEWS_PATH;

const loadOrganizationContent = async (req, res) => {

    let html = "";
    if (config.mode === constants.DEV_MODE) {
        html = await loadOrgContentFromFile(req, res);
    } else {
        html = await loadOrgContentFromAPI(req, res);
    }
    res.send(html);
}
const loadDefaultLandingPage = async (req, res) => {

    let html = "";
    const completeTemplatePath = path.join(require.main.filename, '../pages/default-home/page.hbs');
    const templateResponse = await fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
    const template = await Handlebars.compile(templateResponse);
    html = template();
    
    // Track home page visit telemetry for default landing page
    trackHomePageVisit({
        idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
    });
    
    res.send(html);
}
const loadOrgContentFromFile = async (req, res) => {

    //TODO fetch from DB
    const mockProfileDataPath = path.join(process.cwd(), filePrefix + '../mock', '/userProfiles.json');
    const mockProfileData = JSON.parse(fs.readFileSync(mockProfileDataPath, constants.CHARSET_UTF8));
    const templateContent = {
        userProfiles: mockProfileData,
        baseUrl: baseURLDev + req.params.viewName
    };
    
    // Track home page visit telemetry for dev mode
    trackHomePageVisit({
        idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
    });
    
    return renderTemplate(filePrefix + 'pages/home/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false)
}

const loadOrgContentFromAPI = async (req, res) => {
    let html;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
    try {
        const orgId = await adminDao.getOrgId(orgName);
        let profile = null;
        if (req.user) {
            profile = {
                imageURL: req.user.imageURL,
                firstName: req.user.firstName,
                lastName: req.user.lastName,
                email: req.user.email,
            }
        }
        templateContent = {
            devportalMode: devportalMode,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
            profile: req.isAuthenticated() ? profile : null
        };
        html = await renderTemplateFromAPI(templateContent, orgId, orgName, 'pages/home', req.params.viewName);
        
        // Track home page visit telemetry
        trackHomePageVisit({ 
            orgId: orgId, 
            idpId: req.isAuthenticated() ? (req[constants.USER_ID] || req.user.sub) : undefined
        });
    } catch (error) {
        logger.error(`Failed to load organization`, {
            orgName: req.params?.orgName,
            error: error.message,
            stack: error.stack
        });
        const templateContent = {
            devportalMode: devportalMode,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
        }
        html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        return res.send(html);
    }
    return html;
}

module.exports = {
    loadOrgContentFromFile,
    loadOrgContentFromAPI,
    loadOrganizationContent,
    loadDefaultLandingPage
};
