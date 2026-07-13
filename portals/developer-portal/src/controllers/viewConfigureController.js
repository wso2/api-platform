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
const apiFileDao = require('../dao/apiFileDao');
const viewDao = require('../dao/viewDao');
const labelDao = require('../dao/labelDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const whDao = require('../dao/webhookSubscriberDao');
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const kmDao = require('../dao/keyManagerDao');
const { KeyManagerDTO } = require('../dto/keyManagerDto');
const { VALID_EVENT_TYPES } = require('../services/webhooks/eventPublisher');
const apiWorkflowService = require('../services/apiWorkflowService');
const apiMetadataService = require('../services/apiMetadataService');
const util = require('../utils/util');
const { renderGivenTemplate, loadLayoutFromAPI } = require('../utils/util');
const { getSessionCsrfToken } = require('../middlewares/csrfProtection');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');

// Org-scoped settings page. The URL is /:orgName/settings (no view segment) —
// almost all settings data is keyed by org. The two genuinely view-scoped panels
// (LLM Instructions, API Workflows) render for an initial view (default, or the
// first view) and switch client-side via the in-page view selector.
const loadSettingsPage = async (req, res) => {

    let orgId;
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', 'settings', 'page.hbs');
    const layoutPath = path.join(process.cwd(), 'src', 'defaultContent', 'layout', 'main.hbs');

    const orgName = req.params.orgName;
    // Org-scoped self-links (view selector switches the two view-scoped panels client-side).
    const settingsUrl = '/' + orgName + '/settings';
    const csrfToken = getSessionCsrfToken(req);
    let templateContent = {
        settingsUrl,
        csrfToken,
        showApiWorkflowsNav: config.features?.apiWorkflows === true,
        demoMode: config.demo?.enabled === true
    };
    try {
        templateContent.loggedOrg = orgName;
        orgId = await orgDao.getId(orgName);
        const orgDetails = await orgDao.get(orgName);
        templateContent.devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
        templateContent.orgId = orgId;

        // The Organization tab manages only the current org (no listing/add/delete).
        const cur = orgDetails.dataValues || orgDetails;
        templateContent.currentOrg = {
            id: cur.handle,
            displayName: cur.display_name,
            businessOwner: cur.business_owner || '',
            businessOwnerContact: cur.business_owner_contact || '',
            businessOwnerEmail: cur.business_owner_email || '',
            idpRefId: orgDetails.idp_ref_id || '',
            cpRefId: orgDetails.cp_ref_id || '',
        };

        // Views for the selector and the merged Views management tab. The in-page
        // view selector picks which view the LLM + API Workflow panels edit via the
        // ?view= query param (the path stays org-scoped). Default to 'default', then
        // fall back to the first view; ignore an unknown ?view value.
        const views = await apiMetadataService.getViewsFromDB(orgId);
        templateContent.views = views;
        const requestedView = typeof req.query.view === 'string' ? req.query.view : '';
        const viewExists = (name) => views.some(v => v.id === name);
        let viewName = 'default';
        if (requestedView && viewExists(requestedView)) {
            viewName = requestedView;
        } else if (!viewExists('default') && views.length > 0) {
            viewName = views[0].id;
        }
        templateContent.viewName = viewName;
        templateContent.selectedView = viewName;

        // Portal chrome (sidebar/header/home link) is inherently view-scoped.
        const baseUrl = '/' + orgName + '/views/' + viewName;
        templateContent.baseUrl = baseUrl;

        const viewId = await viewDao.getId(orgId, viewName);
        const apiWorkflows = await apiWorkflowService.getAllAPIWorkflowsFromDB(orgId, viewId);
        templateContent.apiWorkflows = apiWorkflows;

        const allAPIs = await apiDao.getByCondition({ org_uuid: orgId });
        const docNamesByApiId = await apiFileDao.listDocNamesForApis(orgId, allAPIs.map(api => api.uuid));
        const mappedAPIs = allAPIs.map(api => ({
            apiId: api.handle,
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
            subscriptionPlans: (api.dp_subscription_plans || []).map(p => p.display_name),
            existingDocs: docNamesByApiId[api.uuid] || [],
        }));
        // MCP servers get their own admin tab; keep REST/WS/GraphQL/SOAP/WebSub in the APIs tab.
        // orgAllAPIs backs the client-side apiMap (edit/drawer lookups) shared by both tables.
        templateContent.orgAllAPIs = mappedAPIs;
        templateContent.orgAPIs = mappedAPIs.filter(api => api.apiType !== constants.API_TYPE.MCP);
        templateContent.orgMCPs = mappedAPIs.filter(api => api.apiType === constants.API_TYPE.MCP);

        let orgLabels = [];
        try {
            const labelsRaw = await labelDao.list(orgId);
            orgLabels = labelsRaw.map(l => ({ labelId: l.uuid, id: l.handle, displayName: l.display_name }));
        } catch (err) {
            logger.warn('Failed to load labels for settings page', { error: err.message });
        }
        templateContent.orgLabels = orgLabels;

        let orgPlans = [];
        try {
            const plansRaw = await subscriptionPlanDao.list(orgId);
            orgPlans = plansRaw.map(p => ({
                planId: p.handle,
                planName: p.handle,
                displayName: p.display_name,
                description: p.description || '',
                refId: p.ref_id || '',
                limits: (p.limits || []).map(l => ({
                    limitType:  l.limit_type,
                    timeUnit:   l.time_unit ?? null,
                    timeAmount: l.time_amount,
                    limitCount: Number(l.limit_count),
                })),
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

        let keyManagers = [];
        try {
            const keyManagerRecords = await kmDao.list(orgId);
            keyManagers = keyManagerRecords.map(r => new KeyManagerDTO(r));
        } catch (err) {
            logger.warn('Failed to load key managers for settings page', { error: err.message });
        }
        templateContent.keyManagers = keyManagers;

        const configAsset = await orgDao.getContent({
            orgId: orgId, fileType: constants.FILE_TYPE.LLMS_CONFIG, viewName, fileName: constants.FILE_NAME.LLMS_CONFIG
        });
        let llmsConfig = { aiEnabled: true, portalName: '', portalDescription: '' };
        if (configAsset) {
            try { llmsConfig = { ...llmsConfig, ...JSON.parse(configAsset.file_content.toString('utf8')) }; } catch (e) { /* ignore */ }
        }
        templateContent.llmsConfig = llmsConfig;
        // orgName + views let the client rebuild view-scoped URLs when the selector changes.
        templateContent.llmsConfigContext = { orgId, orgName, viewName, csrfToken, baseUrl, views };

        const hasCustomTheme = await orgDao.hasThemeContent(orgId, viewName);
        templateContent.themingContext = { orgId, orgName, viewName, csrfToken, baseUrl, views, hasCustomTheme };

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
        logger.error(`Error while loading settings page`, {
            error: error.message,
            stack: error.stack
        });
        res.status(500).send('Error loading settings page');
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
        res.status(500).json({ error: 'Failed to get LLMs configuration' });
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
        res.status(500).json({ error: 'Failed to save LLMs configuration' });
    }
};

module.exports = {
    loadSettingsPage,
    getLlmsConfig,
    saveLlmsConfig,
};
