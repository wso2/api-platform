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
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const viewDao = require('../dao/viewDao');
const labelDao = require('../dao/labelDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const whDao = require('../dao/webhookSubscriberDao');
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const { VALID_EVENT_TYPES } = require('../services/webhooks/eventPublisher');
const apiFlowService = require('../services/apiFlowService');
const { renderGivenTemplate, loadLayoutFromAPI } = require('../utils/util');
const { getSessionCsrfToken } = require('../middlewares/csrfProtection');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');

const loadViewSettingsPage = async (req, res) => {

    let orgID;
    const viewName = req.params.viewName || 'default';
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'settings', 'page.hbs');
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
        const orgName = req.params.orgName;
        templateContent.loggedOrg = orgName;
        orgID = await orgDao.getId(orgName);
        const orgDetails = await orgDao.get(orgName);
        templateContent.devportalMode = orgDetails.CONFIGURATION?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
        templateContent.orgID = orgID;

        const viewId = await viewDao.getId(orgID, viewName);
        const apiFlows = await apiFlowService.getAllAPIFlowsFromDB(orgID, viewId);
        templateContent.apiFlows = apiFlows;

        const allAPIs = await apiDao.getByCondition({ ORG_UUID: orgID });
        templateContent.orgAPIs = allAPIs.map(api => ({
            apiId: api.UUID,
            apiName: api.NAME,
            apiHandle: api.HANDLE,
            apiDescription: api.DESCRIPTION,
            apiType: api.TYPE,
            apiVersion: api.VERSION,
            apiStatus: api.STATUS,
            productionUrl: api.PRODUCTION_URL,
            sandboxUrl: api.SANDBOX_URL,
            tags: (api.DP_TAGs || []).map(tag => tag.NAME),
            agentVisibility: api.AGENT_VISIBILITY,
            subscriptionPlans: (api.SubscriptionPlans || []).map(p => p.NAME),
        }));

        let orgLabels = [];
        try {
            const labelsRaw = await labelDao.list(orgID);
            orgLabels = labelsRaw.map(l => ({ labelId: l.UUID, name: l.NAME, displayName: l.DISPLAY_NAME }));
        } catch (err) {
            logger.warn('Failed to load labels for settings page', { error: err.message });
        }
        templateContent.orgLabels = orgLabels;

        let orgPlans = [];
        try {
            const plansRaw = await subscriptionPlanDao.list(orgID);
            orgPlans = plansRaw.map(p => ({
                planId: p.UUID,
                planName: p.NAME,
                displayName: p.DISPLAY_NAME,
                description: p.DESCRIPTION || '',
                requestCount: p.REQUEST_COUNT,
                refId: p.REF_ID || '',
            }));
        } catch (err) {
            logger.warn('Failed to load subscription plans for settings page', { error: err.message });
        }
        templateContent.orgPlans = orgPlans;

        let webhookSubscribers = [];
        try {
            const webhookSubscriberRecords = await whDao.list(orgID);
            webhookSubscribers = webhookSubscriberRecords.map(r => new WebhookSubscriberDTO(r));
        } catch (err) {
            logger.warn('Failed to load webhook subscribers for settings page', { error: err.message });
        }
        templateContent.webhookSubscribers = webhookSubscribers;
        templateContent.webhookEventTypes = [...VALID_EVENT_TYPES];

        const configAsset = await orgDao.getContent({
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        let llmsConfig = { aiEnabled: true, portalName: '', portalDescription: '' };
        if (configAsset) {
            try { llmsConfig = { ...llmsConfig, ...JSON.parse(configAsset.FILE_CONTENT.toString('utf8')) }; } catch (e) { /* ignore */ }
        }
        templateContent.llmsConfig = llmsConfig;
        templateContent.llmsConfigContext = { orgID, viewName, csrfToken, baseUrl };

        templateContent.profile = req.user;
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
        const orgID = await orgDao.getId(orgName);
        const asset = await orgDao.getContent({
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
        const orgID = await orgDao.getId(orgName);
        const userId = util.resolveActor(req);
        const content = Buffer.from(JSON.stringify({ aiEnabled, portalName, portalDescription }));
        const orgData = {
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName,
            fileName: constants.FILE_NAME.LLMS_CONFIG, fileContent: content, filePath: constants.FILE_TYPE.LLMS_CONFIG,
            createdBy: userId
        };
        const existing = await orgDao.getContent({
            orgId: orgID, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        if (existing) {
            await orgDao.updateContent(orgData);
        } else {
            await orgDao.createContent(orgData);
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
