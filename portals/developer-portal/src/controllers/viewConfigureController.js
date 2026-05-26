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
const fs = require('fs');
const path = require('path');
const logger = require('../config/logger');
const adminDao = require('../dao/admin');
const apiMetadataDao = require('../dao/apiMetadata');
const apiFlowService = require('../services/apiFlowService');
const { renderGivenTemplate, loadLayoutFromAPI } = require('../utils/util');
const { getSessionCsrfToken } = require('../middlewares/csrfProtection');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');

const loadViewSettingsPage = async (req, res) => {

    let orgID;
    const viewName = req.params.viewName || 'default';
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'api-flows', 'page.hbs');
    const layoutPath = path.join(process.cwd(), 'src', 'defaultContent', 'layout', 'main.hbs');

    const baseUrl = '/' + req.params.orgName + '/views/' + viewName;
    const csrfToken = getSessionCsrfToken(req);
    let templateContent = {
        baseUrl,
        viewName,
        csrfToken,
        showApiWorkflowsNav: config.features?.apiWorkflows?.enabled === true
    };
    try {
        if (config.mode === constants.DEV_MODE) {
            templateContent.apiFlows = [];
            templateContent.orgAPIs = [];
            templateContent.devportalMode = constants.DEVPORTAL_MODE.DEFAULT;
            templateContent.llmsConfig = { aiEnabled: true, portalName: '', portalDescription: '' };
            templateContent.llmsConfigContext = { orgID: '', viewName, csrfToken, baseUrl };
        } else {
            const orgName = req.params.orgName;
            templateContent.loggedOrg = orgName;
            orgID = await adminDao.getOrgId(orgName);
            const orgDetails = await adminDao.getOrganization(orgName);
            templateContent.devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
            templateContent.orgID = orgID;

            const viewId = await apiMetadataDao.getViewID(orgID, viewName);
            const apiFlows = await apiFlowService.getAllAPIFlowsFromDB(orgID, viewId);
            templateContent.apiFlows = apiFlows;

            const allAPIs = await apiMetadataDao.getAPIMetadataByCondition({ ORG_ID: orgID, STATUS: constants.API_STATUS.PUBLISHED });
            templateContent.orgAPIs = allAPIs.map(api => ({
                apiId: api.API_ID,
                apiName: api.API_NAME,
                apiHandle: api.API_HANDLE,
                apiDescription: api.API_DESCRIPTION,
                apiType: api.API_TYPE,
                productionUrl: api.PRODUCTION_URL,
                agentVisibility: api.AGENT_VISIBILITY
            }));

            const configAsset = await adminDao.getOrgContent({
                orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
            });
            let llmsConfig = { aiEnabled: true, portalName: '', portalDescription: '' };
            if (configAsset) {
                try { llmsConfig = { ...llmsConfig, ...JSON.parse(configAsset.FILE_CONTENT.toString('utf8')) }; } catch (e) { /* ignore */ }
            }
            templateContent.llmsConfig = llmsConfig;
            templateContent.llmsConfigContext = { orgID, viewName, csrfToken, baseUrl };

            templateContent.profile = req.user;
        }
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        const dbLayout = orgID ? await loadLayoutFromAPI(orgID, viewName) : '';
        let html;
        if (dbLayout) {
            html = await renderGivenTemplate(templateResponse, dbLayout, templateContent);
        } else {
            const layoutResponse = fs.readFileSync(layoutPath, constants.CHARSET_UTF8);
            html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        }
        res.send(html);
    } catch (error) {
        logger.error(`Error while loading view settings page`, {
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('Error loading view configuration page');
    }
};

const getLlmsConfig = async (req, res) => {
    const { orgName, viewName } = req.params;
    try {
        const orgID = await adminDao.getOrgId(orgName);
        const asset = await adminDao.getOrgContent({
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        if (!asset) {
            return res.json({ aiEnabled: true, portalName: '', portalDescription: '' });
        }
        res.json(JSON.parse(asset.FILE_CONTENT.toString('utf8')));
    } catch (err) {
        logger.error('Error getting llms config', { error: err.message, stack: err.stack });
        res.status(500).json({ error: err.message });
    }
};

const saveLlmsConfig = async (req, res) => {
    const { orgName, viewName } = req.params;
    const { aiEnabled: rawAiEnabled, portalName: rawPortalName, portalDescription: rawPortalDescription } = req.body;

    const aiEnabled = rawAiEnabled === true || rawAiEnabled === 'true' || rawAiEnabled === '1' || rawAiEnabled === 1;
    const portalName = (typeof rawPortalName === 'string' ? rawPortalName : String(rawPortalName ?? ''))
        .trim().replace(/[<>"'&]/g, '').slice(0, 100);
    const portalDescription = (typeof rawPortalDescription === 'string' ? rawPortalDescription : String(rawPortalDescription ?? ''))
        .trim().replace(/[<>"'&]/g, '').slice(0, 1000);

    try {
        const orgID = await adminDao.getOrgId(orgName);
        const content = Buffer.from(JSON.stringify({ aiEnabled, portalName, portalDescription }));
        const orgData = {
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName,
            fileName: constants.FILE_NAME.LLMS_CONFIG, fileContent: content, filePath: constants.FILE_TYPE.LLMS_CONFIG
        };
        const existing = await adminDao.getOrgContent({
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        if (existing) {
            await adminDao.updateOrgContent(orgData);
        } else {
            await adminDao.createOrgContent(orgData);
        }
        res.json({ message: 'Saved successfully' });
    } catch (err) {
        logger.error('Error saving llms config', { error: err.message, stack: err.stack });
        res.status(500).json({ error: err.message });
    }
};

module.exports = {
    loadViewSettingsPage,
    getLlmsConfig,
    saveLlmsConfig,
};
