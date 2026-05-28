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
const billingController = require("../controllers/billingController");
const usageController = require("../controllers/usageController");
const invoiceController = require("../controllers/invoiceController");
const { ensureBillingAuth, verifyRequestOrigin } = require("../middlewares/billingAuth");
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

router.post('/organizations', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'organization', maxCount: 1 }]), adminService.createOrganization);
router.get('/organizations', enforceSecuirty(constants.SCOPES.ADMIN), adminService.getOrganizations);
router.put('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'organization', maxCount: 1 }]), adminService.updateOrganization);
router.get('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), devportalService.getOrganization); // S2S Applied 
router.delete('/organizations/:orgId', enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteOrganization);

router.post('/organizations/:orgId/identityProvider', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'identityProvider', maxCount: 1 }]), adminService.createIdentityProvider);
router.put('/organizations/:orgId/identityProvider', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{ name: 'identityProvider', maxCount: 1 }]), adminService.updateIdentityProvider);
router.get('/organizations/:orgId/identityProvider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.getIdentityProvider);
router.delete('/organizations/:orgId/identityProvider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteIdentityProvider);

// Key Manager routes (admin — JSON body OR YAML file upload on POST/PUT)
const keyManagerService = require('../services/keyManagerService');
router.post('/organizations/:orgId/key-managers', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{name: 'keymanager', maxCount: 1}]), keyManagerService.createKeyManager);
router.get('/organizations/:orgId/key-managers', enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.getKeyManagers);
router.get('/organizations/:orgId/key-managers/:kmId', enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.getKeyManager);
router.put('/organizations/:orgId/key-managers/:kmId', enforceSecuirty(constants.SCOPES.ADMIN), multipartHandler.fields([{name: 'keymanager', maxCount: 1}]), keyManagerService.updateKeyManager);
router.delete('/organizations/:orgId/key-managers/:kmId', enforceSecuirty(constants.SCOPES.ADMIN), keyManagerService.deleteKeyManager);
// Key Manager routes (developer-facing)
router.get('/organizations/:orgId/key-managers/discover', enforceSecuirty(constants.SCOPES.DEVELOPER), keyManagerService.getAvailableKeyManagers);

const upload = multer({ dest: os.tmpdir() });
router.post('/organizations/:orgId/views/:name/layout', enforceSecuirty(constants.SCOPES.ADMIN), upload.single('file'), adminService.createOrgContent);
router.put('/organizations/:orgId/views/:name/layout', enforceSecuirty(constants.SCOPES.ADMIN), upload.single('file'), adminService.updateOrgContent);
router.get('/organizations/:orgId/views/:name/layout', devportalService.getOrgContent);
router.get('/organizations/:orgId/views/:name/layout/:fileType', devportalService.getOrgContent);
router.delete('/organizations/:orgId/views/:name/layout', enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteOrgContent);

router.post('/organizations/:orgId/provider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.createProvider);
router.put('/organizations/:orgId/provider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.updateProvider);
router.get('/organizations/:orgId/provider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.getProviders);
router.delete('/organizations/:orgId/provider', enforceSecuirty(constants.SCOPES.ADMIN), adminService.deleteProvider);

router.post(
    '/organizations/:orgId/apis',
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.createAPIMetadata);
router.get('/organizations/:orgId/apis/:apiId', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getAPIMetadata);
router.get('/organizations/:orgId/apis', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getAllAPIMetadata);
router.put(
    '/organizations/:orgId/apis/:apiId',
    enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([
        {name: 'api', maxCount: 1},
        {name: 'apiDefinition', maxCount: 1},
        {name: 'artifact', maxCount: 1},
        {name: 'schemaDefinition', maxCount: 1},
    ]),
    apiMetadataService.updateAPIMetadata);
router.delete('/organizations/:orgId/apis/:apiId', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIMetadata);

router.post('/organizations/:orgId/subscription-policies', enforceSecuirty(constants.SCOPES.DEVELOPER), multipartHandler.fields([{ name: 'subscriptionPolicy', maxCount: 1 }]), apiMetadataService.addSubscriptionPolicies);
router.get('/organizations/:orgId/subscription-policies/:policyID', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.getSubscriptionPolicy);
router.put('/organizations/:orgId/subscription-policies', enforceSecuirty(constants.SCOPES.DEVELOPER), multipartHandler.fields([{ name: 'subscriptionPolicy', maxCount: 1 }]), apiMetadataService.putSubscriptionPolicies);
router.delete('/organizations/:orgId/subscription-policies/:policyName', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteSubscriptionPolicy);

const apiZip = multer({ dest: '/tmp' });
// New /content routes (ZIP structure: web/ + docs/)
router.post('/organizations/:orgId/apis/:apiId/content', enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.createAPIContent);
router.put('/organizations/:orgId/apis/:apiId/content', enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.updateAPIContent);
router.get('/organizations/:orgId/apis/:apiId/content', apiMetadataService.getAPIFile);
router.delete('/organizations/:orgId/apis/:apiId/content', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIFile);
// Legacy /template routes (ZIP structure: {name}/content/ + images/ + documents/)
router.post('/organizations/:orgId/apis/:apiId/template', enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.createAPITemplate);
router.put('/organizations/:orgId/apis/:apiId/template', enforceSecuirty(constants.SCOPES.DEVELOPER), apiZip.single('apiContent'), apiMetadataService.updateAPITemplate);
router.get('/organizations/:orgId/apis/:apiId/template', apiMetadataService.getAPIFile);
router.delete('/organizations/:orgId/apis/:apiId/template', enforceSecuirty(constants.SCOPES.DEVELOPER), apiMetadataService.deleteAPIFile);

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

router.post('/organizations/:orgId/labels', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.createLabels);
router.put('/organizations/:orgId/labels', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.updateLabel);
router.get('/organizations/:orgId/labels', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.retrieveLabels);
router.delete('/organizations/:orgId/labels', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.deleteLabels);

router.post('/organizations/:orgId/applications', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    adminService.createDevPortalApplication);
router.put('/organizations/:orgId/applications/:appId', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    adminService.updateDevPortalApplication);
router.get('/organizations/:orgId/applications/:appId', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.getDevPortalApplicationDetails);
router.get('/organizations/:orgId/applications', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.getDevPortalApplications);
router.delete('/organizations/:orgId/applications/:appId', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.deleteDevPortalApplication);

// Platform Gateway Subscriptions
router.post('/organizations/:orgId/subscriptions',
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.createSubscription);
router.get('/organizations/:orgId/subscriptions',
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.listSubscriptions);
router.get('/organizations/:orgId/subscriptions/:subscriptionId',
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.getSubscription);
router.put('/organizations/:orgId/subscriptions/:subscriptionId',
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.updateSubscription);
router.delete('/organizations/:orgId/subscriptions/:subscriptionId',
    enforceSecuirty(constants.SCOPES.DEVELOPER), subscriptionService.deleteSubscription);

// API keys — devportal is source of truth; gateway notified via webhook event
router.post('/organizations/:orgId/platform-api-keys/generate',
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.generateApiKey);
router.get('/organizations/:orgId/platform-api-keys',
    enforceSecuirty(constants.SCOPES.DEVELOPER), apiKeyController.listApiKeys);
router.post('/organizations/:orgId/platform-api-keys/:apiKeyId/regenerate',
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.regenerateApiKey);
router.post('/organizations/:orgId/platform-api-keys/:apiKeyId/revoke',
    enforceSecuirty(constants.SCOPES.DEVELOPER), requireCsrfForMutatingApi, apiKeyController.revokeApiKey);

//store key mapping for devportal app and Control plane apps
router.post('/organizations/:orgId/app-key-mapping', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.createAppKeyMapping);
router.get('/organizations/:orgId/app-key-mapping/:appId', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.retriveAppKeyMappings);
//router.delete('/organizations/:orgId/app-key-mapping/:appId', enforceSecuirty(constants.SCOPES.DEVELOPER), adminService.updateDevPortalApplication);

router.post('/organizations/:orgId/views', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.addView);
router.put('/organizations/:orgId/views/:name', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.updateView);
router.get('/organizations/:orgId/views/:name', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.getView);
router.get('/organizations/:orgId/views', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.getAllViews);
router.delete('/organizations/:orgId/views/:name', enforceSecuirty(constants.SCOPES.ADMIN), apiMetadataService.deleteView);

router.post('/applications', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    devportalController.saveApplication);
router.put('/applications/:applicationId', enforceSecuirty(constants.SCOPES.DEVELOPER),
    multipartHandler.fields([{name: 'application', maxCount: 1}]),
    devportalController.updateApplication);
router.delete('/applications/:applicationId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.deleteApplication);
router.post('/applications/:applicationId/reset-throttle-policy', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.resetThrottlingPolicy);
router.post('/applications/:applicationId/api-keys/generate', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateAPIKeys);
router.post('/applications/:applicationId/generate-keys', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateApplicationKeys);
router.post('/applications/:applicationId/oauth-keys/:keyMappingId/generate-token', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateOAuthKeys);
router.delete('/applications/:applicationId/oauth-keys/:keyMappingId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.revokeOAuthKeys);
router.put('/applications/:applicationId/oauth-keys/:keyMappingId', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.updateOAuthKeys);
router.post('/applications/:applicationId/oauth-keys/:keyMappingId/clean-up', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.cleanUp);


router.post('/api-keys/generate', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.generateAPIKeys);
router.post('/api-keys/:apiKeyID/revoke', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.revokeAPIKeys);
router.post('/api-keys/:apiKeyID/regenerate', enforceSecuirty(constants.SCOPES.DEVELOPER), devportalController.regenerateAPIKeys);

// Billing / Stripe
router.get("/organizations/:orgId/billing/usage-data", ensureBillingAuth, billingController.getUsageData);
router.get("/organizations/:orgId/billing/payment-methods", ensureBillingAuth, billingController.getPaymentMethods);

// Billing Engine Keys CRUD
router.post("/organizations/:orgId/billing-engine-keys", verifyRequestOrigin, enforceSecuirty(constants.SCOPES.ADMIN), billingController.addBillingEngineKeys);
router.put("/organizations/:orgId/billing-engine-keys", verifyRequestOrigin, enforceSecuirty(constants.SCOPES.ADMIN), billingController.updateBillingEngineKeys);
router.delete("/organizations/:orgId/billing-engine-keys", verifyRequestOrigin, enforceSecuirty(constants.SCOPES.ADMIN), billingController.deleteBillingEngineKeys);
router.get("/organizations/:orgId/billing-engine-keys", enforceSecuirty(constants.SCOPES.ADMIN), billingController.getBillingEngineKeys);
router.get("/organizations/:orgId/billing/info", ensureBillingAuth, billingController.getBillingInfo);
router.get("/organizations/:orgId/billing/subscriptions", ensureBillingAuth, billingController.getActiveSubscriptions);
router.post("/organizations/:orgId/monetization/checkout", verifyRequestOrigin, ensureBillingAuth, billingController.createCheckoutSessionForSubscription);
router.post("/organizations/:orgId/monetization/stripe/register/:checkoutSessionId", verifyRequestOrigin, ensureBillingAuth, billingController.registerStripeCheckoutSession);
router.post("/organizations/:orgId/subscriptions/:subId/cancel", verifyRequestOrigin, ensureBillingAuth, billingController.cancelSubscription);
router.get("/organizations/:orgId/subscriptions/:subId/billing-status", ensureBillingAuth, billingController.getSubscriptionBillingStatus);
router.post("/organizations/:orgId/billing-portal", verifyRequestOrigin, ensureBillingAuth, billingController.createBillingPortalByOrg);
router.post("/organizations/:orgId/subscriptions/:subId/billing-portal", verifyRequestOrigin, ensureBillingAuth, billingController.createBillingPortal);

// Usage
router.get("/organizations/:orgId/subscriptions/:subId/usage", ensureBillingAuth, usageController.getSubscriptionUsage);

// Invoices
router.get("/organizations/:orgId/invoices", ensureBillingAuth, invoiceController.listInvoices);
router.get("/organizations/:orgId/invoices/:invoiceId", ensureBillingAuth, invoiceController.getInvoice);
router.get("/organizations/:orgId/subscriptions/:subId/invoices", ensureBillingAuth, invoiceController.listInvoicesBySubscription);
router.get("/organizations/:orgId/invoices/:invoiceId/pdf", ensureBillingAuth, invoiceController.getInvoicePdfLink);
router.get("/organizations/:orgId/invoices/:invoiceId/hosted", ensureBillingAuth, invoiceController.redirectHostedInvoice);

// API Flows (admin)
router.post('/organizations/:orgId/views/:viewName/api-flows', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.createAPIFlow);
router.get('/organizations/:orgId/views/:viewName/api-flows', enforceSecuirty(constants.SCOPES.ADMIN), apiFlowService.getAllAPIFlows);
router.get('/organizations/:orgId/views/:viewName/api-flows/:apiFlowId', enforceSecuirty(constants.SCOPES.ADMIN), apiFlowService.getAPIFlow);
router.put('/organizations/:orgId/views/:viewName/api-flows/:apiFlowId', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.updateAPIFlow);
router.delete('/organizations/:orgId/views/:viewName/api-flows/:apiFlowId', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.deleteAPIFlow);
router.post('/organizations/:orgId/views/:viewName/api-flows/generate-prompt', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, apiFlowService.generatePrompt);

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
    router.post('/organizations/:orgId/applications/import', enforceSecuirty(constants.SCOPES.ADMIN),multipartHandler.single('file'), devportalController.importApplications);
}

// Webhook event admin (admin-only)
router.get('/organizations/:orgId/events', enforceSecuirty(constants.SCOPES.ADMIN), webhookAdminController.listEvents);
router.get('/organizations/:orgId/events/:eventId', enforceSecuirty(constants.SCOPES.ADMIN), webhookAdminController.getEvent);
router.post('/organizations/:orgId/deliveries/:deliveryId/retry', enforceSecuirty(constants.SCOPES.ADMIN), requireCsrfForMutatingApi, webhookAdminController.retryDelivery);

module.exports = router;
