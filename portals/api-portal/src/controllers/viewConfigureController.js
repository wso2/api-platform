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
const orgContext = require('../utils/orgContext');
const labelDao = require('../dao/labelDao');
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const whDao = require('../dao/webhookSubscriberDao');
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const kmDao = require('../dao/keyManagerDao');
const kmRegistry = require('../services/keyManagerRegistry');
const kmConfigDao = require('../dao/keyManagerConfigurationDao');
require('../keymanagers/drivers'); // built-in drivers self-register on require
const { registeredTypeOptions } = require('../keymanagers/core/registry');
const { KeyManagerDTO } = require('../dto/keyManagerDto');
const { VALID_EVENT_TYPES } = require('../services/webhooks/eventPublisher');
const { groupWebhookEventTypes } = require('../utils/webhookEventGroups');
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
    const settingsUrl = constants.ROUTE.BASE_PATH + '/' + orgName + '/settings';
    const csrfToken = getSessionCsrfToken(req);
    let templateContent = {
        settingsUrl,
        csrfToken,
        showApiWorkflowsNav: (config.artifacts?.enabledTypes || []).includes('api-workflows')
    };
    try {
        templateContent.loggedOrg = orgName;
        orgId = await orgDao.getId(orgName);
        const orgDetails = await orgDao.get(orgName);
        templateContent.orgId = orgId;

        // The Organization tab manages only the current org (no listing/add/delete).
        const cur = orgDetails;
        templateContent.currentOrg = {
            id: cur.handle,
            displayName: cur.display_name,
            businessOwner: cur.business_owner || '',
            businessOwnerContact: cur.business_owner_contact || '',
            businessOwnerEmail: cur.business_owner_email || '',
            idpRefId: orgDetails.idp_ref_id || '',
            cpRefId: orgDetails.cp_ref_id || '',
            // Whole configuration object so the save can merge rather than
            // replace — orgDao.update overwrites the column wholesale, and the
            // API contract allows additional free-form keys.
            configuration: cur.configuration || {},
        };
        // Views for the selector and the merged Views management tab. The in-page
        // view selector picks which view the LLM + API Workflow panels edit via the
        // ?view= query param (the path stays org-scoped); an unknown ?view value is
        // ignored. With none given, the view comes from the single portal-wide resolver
        // (orgContext.getFallbackViewHandle → viewDao.getFallbackHandle): prefer the view
        // whose handle is 'default', else the earliest-created one. This page used to
        // hardcode 'default' with its own views[0] fallback, so it could land on a
        // different view than the bare-org redirect, the error page's home link and the
        // chrome partials — all of which resolve through that resolver.
        const views = await apiMetadataService.getViewsFromDB(orgId);
        templateContent.views = views;
        const requestedView = typeof req.query.view === 'string' ? req.query.view : '';
        const viewExists = (name) => views.some(v => v.id === name);
        let viewName = await orgContext.getFallbackViewHandle();
        if (requestedView && viewExists(requestedView)) {
            viewName = requestedView;
        }
        templateContent.viewName = viewName;
        templateContent.selectedView = viewName;

        // Portal chrome (sidebar/header/home link) is inherently view-scoped.
        const baseUrl = constants.ROUTE.BASE_PATH + '/' + orgName + '/views/' + viewName;
        templateContent.baseUrl = baseUrl;

        const viewId = await viewDao.getId(orgId, viewName);
        const apiWorkflows = await apiWorkflowService.getAllAPIWorkflowsFromDB(orgId, viewId);
        templateContent.apiWorkflows = apiWorkflows;

        const allAPIs = await apiDao.getByCondition({ orgId });
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
            tags: (api.tags || []).map(tag => tag.name),
            labels: (api.labels || []).map(label => label.handle),
            agentVisibility: api.agent_visibility,
            owners: {
                technicalOwner:      api.technical_owner,
                technicalOwnerEmail: api.technical_owner_email,
                businessOwner:       api.business_owner,
                businessOwnerEmail:  api.business_owner_email,
            },
            subscriptionPlans: (api.subscription_plans || []).map(p => p.display_name),
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

        const labelNameByHandle = new Map(orgLabels.map(l => [l.id, l.displayName]));
        templateContent.views = views.map(view => ({
            ...view,
            labelNames: (view.labels || []).map(handle => labelNameByHandle.get(handle) || handle),
        }));

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
        // Grouped by event-type prefix so the picker renders as categories rather than a
        // flat list of 12 checkboxes. Derived from VALID_EVENT_TYPES, so a new event type
        // shows up without editing the template.
        templateContent.webhookEventGroups = groupWebhookEventTypes(VALID_EVENT_TYPES);

        // Both sources, through the one resolver — the same list GET /key-managers
        // returns. Reading kmDao directly here would show only the stored rows, so a
        // config-declared key manager would be missing from the very screen an admin
        // goes to in order to see what exists, while appearing everywhere else.
        let keyManagers = [];
        try {
            const keyManagerRecords = await kmRegistry.list(orgId, { includeDisabled: true });
            // The same projection the REST API applies, so the panel and
            // GET /key-managers agree on which entries can generate keys and what
            // their provisioning holds. One query for the page, not one per row.
            let configs = new Map();
            try {
                configs = await kmConfigDao.listByOrg(orgId);
            } catch (cfgErr) {
                logger.warn('Could not load key manager provisioning for the settings page', {
                    error: cfgErr.message, orgId,
                });
            }
            keyManagers = keyManagerRecords.map((r) => new KeyManagerDTO(
                r.source === kmRegistry.SOURCE_CONFIG
                    // A config-declared key manager carries its driver config in the
                    // deployed TOML, so it can always generate keys and has no row to show.
                    ? { ...r, canGenerateKeys: true }
                    : {
                        ...r,
                        canGenerateKeys: configs.has(r.uuid),
                        provisioning: configs.get(r.uuid) || undefined,
                    }
            ));
        } catch (err) {
            // The registry also builds the configured key managers, so a bad entry (an
            // unreadable mTLS certificate, say) throws here. Fall back to the stored rows
            // rather than showing nothing: an incomplete list an admin can still work with
            // beats an empty one that looks like the data is gone.
            logger.warn('Failed to merge configured key managers for the settings page; falling back to stored rows', {
                error: err.message,
            });
            try {
                const stored = await kmDao.list(orgId);
                keyManagers = stored.map(r => new KeyManagerDTO({ ...r, source: kmRegistry.SOURCE_API }));
            } catch (daoErr) {
                logger.warn('Failed to load key managers for settings page', { error: daoErr.message });
            }
        }
        templateContent.keyManagers = keyManagers;
        // The driver types this build ships, for the key manager form's selector.
        // Read from the registry rather than hardcoded in the template: adding a
        // driver is a code change, and the list must follow it without a second
        // edit that could be forgotten.
        templateContent.keyManagerTypes = registeredTypeOptions();

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
        util.sendError(res, 500, 'Failed to get LLMs configuration');
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
        util.sendError(res, 500, 'Failed to save LLMs configuration');
    }
};

module.exports = {
    loadSettingsPage,
    getLlmsConfig,
    saveLlmsConfig,
};
