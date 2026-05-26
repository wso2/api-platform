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
const { renderTemplate, renderTemplateFromAPI, loadMarkdown } = require('../utils/util');
const { config } = require('../config/configLoader');
const markdown = require('marked');
const fs = require('fs');
const path = require('path');
const adminDao = require('../dao/admin');
const constants = require('../utils/constants');
const logger = require('../config/logger');

const filePrefix = config.pathToContent;
const baseURLDev = config.baseUrl + constants.ROUTE.VIEWS_PATH;

const loadCustomContent = async (req, res) => {

    let html = "";
    const { orgName, viewName } = req.params;
    let filePath = req.originalUrl.split("/" + orgName + constants.ROUTE.VIEWS_PATH + viewName + "/")[1];
    
    // Skip manage-keys routes - they should be handled by the application controller
    // This is a safety check in case the route doesn't match properly
    if (filePath && filePath.match(/^applications\/[^/]+\/manage-keys/)) {
        logger.warn('Manage-keys route intercepted by catch-all route - this should not happen', {
            orgName,
            viewName,
            filePath,
            originalUrl: req.originalUrl
        });
        res.status(404).send('Not found');
        return;
    }
    if (config.mode === constants.DEV_MODE) {
        let templateContent = {};
        templateContent[constants.BASE_URL_NAME] = baseURLDev + viewName;
        //read all markdown content
        if (fs.existsSync(path.join(process.cwd(), filePrefix + 'pages', filePath, 'content'))) {
            const markdDownFiles = fs.readdirSync(path.join(process.cwd(), filePrefix + 'pages/' + filePath + '/content'));
            markdDownFiles.forEach((filename) => {
                const tempKey = filename.split('.md')[0];
                templateContent[tempKey] = loadMarkdown(filename, filePrefix + 'pages/' + filePath + '/content')
            });
        }
        html = renderTemplate(filePrefix + 'pages/' + filePath + '/page.hbs', filePrefix + 'layout/main.hbs', templateContent, false)

    } else {
        let content = {};
        let devportalMode = constants.DEVPORTAL_MODE.DEFAULT;
        try {
            filePath = 'pages/' + filePath;
            // Normalize application-specific manage-keys paths to the shared manage-keys page
            const manageKeysMatch = filePath.match(/^pages\/applications\/[^/]+\/manage-keys/);
            if (manageKeysMatch) {
                filePath = 'pages/manage-keys';
            }
            // Check if the file exists before attempting to render
            const resolvedPagePath = path.join(process.cwd(), filePrefix + filePath + '/page.hbs');
            if (!fs.existsSync(resolvedPagePath)) {
                // If it's a manage-keys route that doesn't exist, return 404 or redirect
                if (filePath.includes('manage-keys')) {
                    throw new Error(`Manage keys page not found. This route should be handled by the application controller.`);
                }
                throw new Error(`Content page not found at ${resolvedPagePath}`);
            }
            const orgDetails = await adminDao.getOrganization(orgName);
            const orgId = orgDetails.ORG_ID;
            devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
            let markDownFiles = await adminDao.getOrgContent({
                orgId: orgId,
                fileType: 'markDown',
                viewName: viewName
            });
            if (markDownFiles.length > 0) {
                markDownFiles.forEach((item) => {
                    const tempKey = item.FILE_NAME.split('.md')[0];
                    content[tempKey] = markdown.parse(item.FILE_CONTENT.toString(constants.CHARSET_UTF8));
                });
            }
            content[constants.BASE_URL_NAME] = '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName;
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
            html = await renderTemplateFromAPI(content, orgId, orgName, filePath, viewName);
        } catch (error) {
            const templateContent = {
                devportalMode: devportalMode,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
                profile: req.isAuthenticated() ? req.user : null,
            }
            logger.error('Error while loading custom content', { 
                orgName,
                error: error.message, 
                stack: error.stack,
                filePath: req.params.filePath,
            });
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        }
    }
    res.send(html);
}



module.exports = {
    loadCustomContent
};