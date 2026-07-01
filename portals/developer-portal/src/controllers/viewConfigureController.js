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
const apiWorkflowService = require('../services/apiWorkflowService');
const { renderGivenTemplate, loadLayoutFromAPI } = require('../utils/util');
const { getSessionCsrfToken } = require('../middlewares/csrfProtection');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');

const loadViewSettingsPage = async (req, res) => {

    let orgId;
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
        orgId = await orgDao.getId(orgName);
        const orgDetails = await orgDao.get(orgName);
        templateContent.devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
        templateContent.orgId = orgId;

        const viewId = await viewDao.getId(orgId, viewName);
        const apiWorkflows = await apiWorkflowService.getAllAPIWorkflowsFromDB(orgId, viewId);
        templateContent.apiWorkflows = apiWorkflows;

        const allAPIs = await apiDao.getByCondition({ org_uuid: orgId });
        templateContent.orgAPIs = allAPIs.map(api => ({
            apiId: api.uuid,
            apiName: api.name,
            apiHandle: api.handle,
            apiDescription: api.description,
            apiType: api.type,
            apiVersion: api.version,
            apiStatus: api.status,
            productionUrl: api.production_url,
            sandboxUrl: api.sandbox_url,
            tags: (api.dp_tags || []).map(tag => tag.name),
            agentVisibility: api.agent_visibility,
            subscriptionPlans: (api.dp_subscription_plans || []).map(p => p.name),
        }));

        let orgLabels = [];
        try {
            const labelsRaw = await labelDao.list(orgId);
            orgLabels = labelsRaw.map(l => ({ labelId: l.uuid, name: l.name, displayName: l.display_name }));
        } catch (err) {
            logger.warn('Failed to load labels for settings page', { error: err.message });
        }
        templateContent.orgLabels = orgLabels;

        let orgPlans = [];
        try {
            const plansRaw = await subscriptionPlanDao.list(orgId);
            orgPlans = plansRaw.map(p => ({
                planId: p.uuid,
                planName: p.name,
                displayName: p.display_name,
                description: p.description || '',
                requestCount: p.request_count,
                refId: p.ref_id || '',
            }));
        } catch (err) {
            logger.warn('Failed to load subscription plans for settings page', { error: err.message });
        }
        templateContent.orgPlans = orgPlans;

        let webhookSubscribers = [];
        try {
            const webhookSubscriberRecords = await whDao.list(orgId);
            webhookSubscribers = webhookSubscriberRecords.map(r => new WebhookSubscriberDTO(r));
        } catch (err) {
            logger.warn('Failed to load webhook subscribers for settings page', { error: err.message });
        }
        templateContent.webhookSubscribers = webhookSubscribers;
        templateContent.webhookEventTypes = [...VALID_EVENT_TYPES];

        const configAsset = await orgDao.getContent({
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        let llmsConfig = { aiEnabled: true, portalName: '', portalDescription: '' };
        if (configAsset) {
            try { llmsConfig = { ...llmsConfig, ...JSON.parse(configAsset.file_content.toString('utf8')) }; } catch (e) { /* ignore */ }
        }
        templateContent.llmsConfig = llmsConfig;
        templateContent.llmsConfigContext = { orgId, viewName, csrfToken, baseUrl };

        templateContent.profile = req.user;
        const templateResponse = fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
        const dbLayout = orgId ? await loadLayoutFromAPI(orgId, viewName) : '';
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
        const orgId = await orgDao.getId(orgName);
        const asset = await orgDao.getContent({
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        if (!asset) {
            return res.json({ aiEnabled: true, portalName: '', portalDescription: '' });
        }
        res.json(JSON.parse(asset.file_content.toString('utf8')));
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
        const orgId = await orgDao.getId(orgName);
        const userId = util.resolveActor(req);
        const content = Buffer.from(JSON.stringify({ aiEnabled, portalName, portalDescription }));
        const orgData = {
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName,
            fileName: constants.FILE_NAME.LLMS_CONFIG, fileContent: content, filePath: constants.FILE_TYPE.LLMS_CONFIG,
        };
        const existing = await orgDao.getContent({
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        if (existing) {
            await orgDao.updateContent({ ...orgData, updatedBy: userId });
        } else {
            await orgDao.createContent({ ...orgData, createdBy: userId });
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
