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
const express = require('express');
const router = express.Router();
const os = require('os');
const path = require('path');
const fs = require('fs').promises;
const devportalService = require('../services/devportalService');
const apiMetadataService = require('../services/apiMetadataService');
const adminService = require('../services/adminService');
const devportalController = require('../controllers/devportalController');
const multer = require('multer');
const storage = multer.memoryStorage()
const multipartHandler = multer({storage: storage})
const { enforceSecuirty } = require('../middlewares/ensureAuthenticated');
const { requireCsrfForMutatingApi } = require('../middlewares/csrfProtection');
const constants = require('../utils/constants');
const { config } = require('../config/configLoader');
const subscriptionService = require('../services/subscriptionService');
const apiKeyController = require('../controllers/apiKeyController');
const apiFlowService = require('../services/apiFlowService');
const webhookAdminController = require('../controllers/webhookAdminController');

// Org-scoped route prefix `/o/:orgId/devportal/v1` — base segment + version
// come from constants.DEVPORTAL_API (single source of truth). Routes that the
// spec keeps at root (/organizations, /applications, /apis, /login,
// /temp-arazzo-file) intentionally do NOT use this prefix.
const ORG = constants.DEVPORTAL_API.ORG_PREFIX;

router.post('/organizations', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'organization', maxCount: 1 }]), adminService.createOrganization);
router.get('/organizations', enforceSecuirty(constants.SCOPES.ADMIN), adminService.getOrganizations);
router.put('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'organization', maxCount: 1 }]), adminService.updateOrganization);
router.get('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), devportalService.getOrganization); // S2S Applied
router.delete('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteOrganization);

router.post(`${ORG}/identity-providers`, enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'identityProvider', maxCount: 1 }]), adminService.createIdentityProvider);
router.put(`${ORG}/identity-providers`, enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'identityProvider', maxCount: 1 }]), adminService.updateIdentityProvider);
router.get(`${ORG}/identity-providers`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.getIdentityProvider);
router.delete(`${ORG}/identity-providers`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteIdentityProvider);

// Key Manager routes (admin — JSON body OR YAML file upload on POST/PUT)
const keyManagerService = require('../services/keyManagerService');
router.post(`${ORG}/key-managers`, enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{name: 'keymanager', maxCount: 1}]), keyManagerService.createKeyManager);
router.get(`${ORG}/key-managers`, enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.getKeyManagers);
router.get(`${ORG}/key-managers/:kmId`, enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.getKeyManager);
router.put(`${ORG}/key-managers/:kmId`, enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{name: 'keymanager', maxCount: 1}]), keyManagerService.updateKeyManager);
router.delete(`${ORG}/key-managers/:kmId`, enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.deleteKeyManager);
// Key Manager routes (developer-facing)
router.get(`${ORG}/key-managers/discover`, enforceSecuirty(constants.SCOPES.DEVELOPER), keyManagerService.getAvailableKeyManagers);

const upload = multer({ dest: os.tmpdir() });
router.post(`${ORG}/views/:viewName/layout`, enforceSecuirty(constants.SCOPES.ADMIN), upload.single('file'), adminService.createOrgContent);
router.put(`${ORG}/views/:viewName/layout`, enforceSecuirty(constants.SCOPES.ADMIN), upload.single('file'), adminService.updateOrgContent);
router.get(`${ORG}/views/:viewName/layout`, devportalService.getOrgContent);
router.get(`${ORG}/views/:viewName/layout/:fileType`, devportalService.getOrgContent);
router.delete(`${ORG}/views/:viewName/layout`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteOrgContent);

router.post(`${ORG}/provider`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.createProvider);
router.put(`${ORG}/provider`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.updateProvider);
router.get(`${ORG}/provider`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.getProviders);
router.delete(`${ORG}/provider`, enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteProvider);

router.post(
    `${ORG}/apis`,
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.createAPIMetadata);
router.get(`${ORG}/apis/:apiId`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getAPIMetadata);
router.get(`${ORG}/apis`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getAllAPIMetadata);
router.put(
    `${ORG}/apis/:apiId`,
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.updateAPIMetadata);
router.delete(`${ORG}/apis/:apiId`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIMetadata);

router.post(`${ORG}/subscription-policies`, enforceSecuirty(constants.SCOPES.DEVELOPER), multipartHandler.fields([{ name: 'subscriptionPolicy', maxCount: 1 }]), apiMetadataService.addSubscriptionPolicies);
router.get(`${ORG}/subscription-policies`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.listSubscriptionPolicies);
router.get(`${ORG}/subscription-policies/:policyId`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getSubscriptionPolicy);
router.put(`${ORG}/subscription-policies`, enforceSecuirty(constants.SCOPES.DEVELOPER), multipartHandler.fields([{ name: 'subscriptionPolicy', maxCount: 1 }]), apiMetadataService.putSubscriptionPolicies);
router.delete(`${ORG}/subscription-policies/:policyId`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteSubscriptionPolicy);

const apiZip = multer({ dest: '/tmp' });
// New /content routes (ZIP structure: web/ + docs/)
router.post(`${ORG}/apis/:apiId/content`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.createAPIContent);
router.put(`${ORG}/apis/:apiId/content`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.updateAPIContent);
router.get(`${ORG}/apis/:apiId/content`, apiMetadataService.getAPIFile);
router.delete(`${ORG}/apis/:apiId/content`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIFile);
// Legacy /template routes (ZIP structure: {name}/content/ + images/ + documents/)
router.post(`${ORG}/apis/:apiId/template`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.createAPITemplate);
router.put(`${ORG}/apis/:apiId/template`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.updateAPITemplate);
router.get(`${ORG}/apis/:apiId/template`, apiMetadataService.getAPIFile);
router.delete(`${ORG}/apis/:apiId/template`, enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIFile);

// S2S Applied APIS
router.post(
    '/apis',
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.createAPIMetadata); // s2s applied
router.get('/apis', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getAllAPIMetadata); // s2s applied
router.put(
    '/apis/:apiId',
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.updateAPIMetadata); // s2s applied
router.delete('/apis/:apiId', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIMetadata); // s2s applied

router.post(`${ORG}/labels`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.createLabels);
router.put(`${ORG}/labels`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.updateLabel);
router.get(`${ORG}/labels`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.retrieveLabels);
router.delete(`${ORG}/labels`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.deleteLabels);


// Platform Gateway Subscriptions
router.post(`${ORG}/subscriptions`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.createSubscription);
router.get(`${ORG}/subscriptions`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.listSubscriptions);
router.get(`${ORG}/subscriptions/:subId`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.getSubscription);
router.put(`${ORG}/subscriptions/:subId`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.updateSubscription);
router.delete(`${ORG}/subscriptions/:subId`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.deleteSubscription);

// API keys — devportal is source of truth; gateway notified via webhook event
router.post(`${ORG}/api-keys/generate`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.generateApiKey);
router.get(`${ORG}/api-keys`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), apiKeyController.listApiKeys);
router.post(`${ORG}/api-keys/:apiKeyId/regenerate`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.regenerateApiKey);
router.post(`${ORG}/api-keys/:apiKeyId/revoke`,
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.revokeApiKey);


router.post(`${ORG}/views`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.addView);
router.put(`${ORG}/views/:viewName`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.updateView);
router.get(`${ORG}/views/:viewName`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.getView);
router.get(`${ORG}/views`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.getAllViews);
router.delete(`${ORG}/views/:viewName`, enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.deleteView);

router.post('/applications', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    devportalController.saveApplication);
router.put('/applications/:applicationId', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    devportalController.updateApplication);
router.delete('/applications/:applicationId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.deleteApplication);
router.post('/applications/:applicationId/reset-throttle-policy', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.resetThrottlingPolicy);
router.post('/applications/:applicationId/generate-keys', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateKeys);
router.post('/applications/:applicationId/oauth-keys/:keyMappingId/generate-token', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateOAuthKeys);
router.delete('/applications/:applicationId/oauth-keys/:keyMappingId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.revokeOAuthKeys);
router.put('/applications/:applicationId/oauth-keys/:keyMappingId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.updateOAuthKeys);
router.post('/applications/:applicationId/oauth-keys/:keyMappingId/clean-up', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.cleanUp);


// API Flows (admin)
router.post(`${ORG}/views/:viewName/api-flows`, enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.createAPIFlow);
router.get(`${ORG}/views/:viewName/api-flows`, enforceSecuirty(constants.SCOPES.ADMIN), apiFlowService.getAllAPIFlows);
router.get(`${ORG}/views/:viewName/api-flows/:apiFlowId`, enforceSecuirty(constants.SCOPES.ADMIN), apiFlowService.getAPIFlow);
router.put(`${ORG}/views/:viewName/api-flows/:apiFlowId`, enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.updateAPIFlow);
router.delete(`${ORG}/views/:viewName/api-flows/:apiFlowId`, enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.deleteAPIFlow);
router.post(`${ORG}/views/:viewName/api-flows/generate-prompt`, enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.generatePrompt);

// Writes Arazzo YAML to a unique temp file so the "Open in VS Code" button can launch it via vscode://file/<path>
router.post('/temp-arazzo-file', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, async (req, res, next) => {
    const { content, filename } = req.body;
    if (!content || typeof content !== 'string') {
        return res.status(400).json({ error: 'content is required' });
    }
    const safeName = (filename || 'workflow.arazzo.yaml')
        .replace(/[^a-zA-Z0-9._-]/g, '-')
        .replace(/\.\.+/g, '.')
        .substring(0, 120);
    try {
        const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'arazzo-'));
        const tmpPath = path.join(tmpDir, safeName);
        await fs.writeFile(tmpPath, content, 'utf8');
        res.json({ path: tmpPath });
    } catch (err) {
        next(err);
    }
});

router.post('/login', devportalController.login);

// Import Application with API Subscriptions
if (config.features?.importApplication?.enabled) {
    router.post(`${ORG}/applications/import`, enforceSecuirty(constants.SCOPES.ADMIN),multipartHandler.single('file'), devportalController.importApplications);
}

// Webhook event admin (admin-only)
router.get(`${ORG}/webhook-events`, enforceSecuirty(constants.SCOPES.ADMIN), webhookAdminController.listEvents);
router.get(`${ORG}/webhook-events/:eventId`, enforceSecuirty(constants.SCOPES.ADMIN), webhookAdminController.getEvent);
router.post(`${ORG}/webhook-deliveries/:deliveryId/retry`, enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, webhookAdminController.retryDelivery);

module.exports = router;
