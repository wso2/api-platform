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
 *
 */
/*
 * Spec ↔ service response drift checker.
 *
 * Validates representative response payloads (drawn directly from service code)
 * against the spec response schemas using AJV — the same engine
 * express-openapi-validator uses internally.
 *
 *   node scripts/drift_check.js
 *
 * Exit code 0 means every sample matches the spec. Exit code 1 means at least
 * one sample failed validation; the failing operationId, status, and AJV
 * error path are printed.
 *
 * --- Authoring samples ---
 *
 * Each sample MUST mirror what the legacy service actually emits today
 * (look at the corresponding `res.status(...).send(...)` / `res.json(...)`
 * call in the service file, NOT what the spec says it should emit). The
 * point is to catch drift between the two; using a spec-shaped sample
 * masks drift.
 *
 * As you find drift in production via `validateResponses: 'log-only'`, add a
 * canned sample here so the regression is caught at boot.
 */

const path = require('path');
const fs = require('fs');
const yaml = require('js-yaml');
const Ajv = require('ajv');

const SPEC = yaml.load(
    fs.readFileSync(path.join(__dirname, '..', 'docs/devportal-openapi-spec-v1.yaml'), 'utf8')
);

function deref(ref) {
    const parts = ref.replace(/^#\//, '').split('/');
    let cur = SPEC;
    for (const p of parts) cur = cur[p];
    return cur;
}

function validationSchema(schema) {
    // Keep $refs intact so AJV can validate recursive schemas instead of
    // replacing cycle edges with permissive `{}` schemas.
    return {
        components: SPEC.components,
        allOf: [schema],
    };
}

// Match express-openapi-validator's AJV config (see node_modules/express-openapi-validator/dist/framework/ajv/options.js).
// `nullable: true` makes OpenAPI 3.0 `nullable: true` declarations honored — without this, AJV rejects `null` for any typed field.
const ajv = new Ajv({
    allErrors: true,
    jsonPointers: true,
    nullable: true,
    useDefaults: true,
    logger: false,
});

function validate(schema, value) {
    try {
        const v = ajv.compile(validationSchema(schema));
        const ok = v(value);
        return { ok, errors: v.errors || [] };
    } catch (e) {
        return { ok: false, errors: [{ message: 'compile error: ' + e.message }] };
    }
}

function responseSchema(operationId, status) {
    for (const methods of Object.values(SPEC.paths)) {
        for (const [m, op] of Object.entries(methods)) {
            if (m === 'parameters') continue;
            if (op.operationId !== operationId) continue;
            const resp = (op.responses || {})[String(status)];
            if (!resp) return null;
            const r = resp.$ref ? deref(resp.$ref) : resp;
            const json = r.content && r.content['application/json'];
            if (!json) return { plainText: true };
            return json.schema;
        }
    }
    return null;
}

// ---------------------------------------------------------------------------
// Samples — taken verbatim from the legacy service / controller code.
// Add to this list as new drift cases surface from runtime log-only validation.
// ---------------------------------------------------------------------------

const SAMPLES = [
    // Organizations — adminService.createOrganization L105-120 emits no superAdminRole;
    // spec marks it nullable so this passes.
    ['createOrganization', 201, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        roleClaimName: 'roles', groupsClaimName: 'groups',
        organizationClaimName: 'organizationIdentifier', organizationIdentifier: 'ACME',
        adminRole: 'admin', subscriberRole: 'sub', groupClaimName: 'group',
        orgConfiguration: {},
    }],
    ['getOrganizations', 200, [{
        orgName: 'Acme', orgID: 'org-1', businessOwner: 'Jane',
        businessOwnerContact: '+1', businessOwnerEmail: 'jane@acme.example',
        orgHandle: 'acme', roleClaimName: 'roles', groupsClaimName: 'groups',
        organizationClaimName: 'organizationIdentifier', organizationIdentifier: 'ACME',
        adminRole: 'admin', subscriberRole: 'sub', superAdminRole: 'super',
        orgConfiguration: {},
    }]],
    ['getOrganization', 200, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        roleClaimName: 'roles', groupsClaimName: 'groups',
        organizationClaimName: 'organizationIdentifier', organizationIdentifier: 'ACME',
        adminRole: 'admin', superAdminRole: 'super', subscriberRole: 'sub',
        groupClaimName: 'group', orgConfiguration: {},
    }],
    ['updateOrganization', 200, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        roleClaimName: 'roles', groupsClaimName: 'groups',
        organizationClaimName: 'organizationIdentifier', organizationIdentifier: 'ACME',
        adminRole: 'admin', subscriberRole: 'sub', superAdminRole: 'super',
        orgConfiguration: {},
    }],

    // Identity Providers — IdentityProviderDTO output (src/dto/identityProvider.js)
    ['createIdentityProvider', 201, {
        name: 'IS', issuer: 'https://idp/iss', authorizationURL: 'https://idp/auth',
        tokenURL: 'https://idp/token', clientId: 'cid', callbackURL: 'http://cb',
        scope: 'openid', logoutURL: 'http://lo', logoutRedirectURI: 'http://lr',
        userInfoURL: 'http://ui',
    }],
    ['updateIdentityProvider', 200, {
        name: 'IS', issuer: 'https://idp/iss', authorizationURL: 'https://idp/auth',
        tokenURL: 'https://idp/token', clientId: 'cid', callbackURL: 'http://cb',
        scope: 'openid', logoutURL: 'http://lo', logoutRedirectURI: 'http://lr',
    }],
    ['getIdentityProvider', 200, {
        name: 'IS', issuer: 'https://idp/iss', authorizationURL: 'https://idp/auth',
        tokenURL: 'https://idp/token', clientId: 'cid', callbackURL: 'http://cb',
        scope: 'openid', logoutURL: 'http://lo', logoutRedirectURI: 'http://lr',
    }],

    // Org content — adminService.createOrgContent L402, updateOrgContent L488 (both 201)
    ['createOrgContent', 201, { orgId: 'org-1', fileName: 'theme.zip' }],
    ['updateOrgContent', 201, { orgId: 'org-1', fileName: 'theme.zip' }],
    ['getOrgLayoutContentByFileType', 200, [
        { orgId: 'org-1', fileName: 'main.css', fileContent: 'body{}' },
    ]],

    // Providers — adminService.createProvider L599 emits providerData object
    ['createProvider', 201, { orgId: 'org-1', name: 'WSO2', providerURL: 'https://wso2.com' }],
    ['updateProvider', 200, { orgId: 'org-1', name: 'WSO2', providerURL: 'https://wso2.com' }],
    ['getProviders', 200, [{ name: 'WSO2', providerURL: 'https://wso2.com' }]],

    // Subscriptions — subscriptionService.formatSubscriptionResponse shape
    ['createSubscription', 201, {
        subscriptionId: 'sub-12345', subscriptionToken: 'tok-abc123',
        status: 'ACTIVE', gatewayType: 'wso2/api-platform',
        apiId: 'api-7f4c2a6b', subscriptionPlanName: 'Gold',
        createdAt: '2026-05-07T08:30:00.000Z',
    }],
    ['updateSubscription', 200, {
        subscriptionId: 'sub-12345', subscriptionToken: 'tok-abc123',
        status: 'INACTIVE', gatewayType: 'wso2/api-platform',
        apiId: 'api-7f4c2a6b', subscriptionPlanName: 'Gold',
        createdAt: '2026-05-07T08:30:00.000Z',
    }],

    // Labels — service emits LabelDTO[] = [{name, displayName}]
    ['retrieveLabels', 200, [
        { name: 'premium', displayName: 'Premium APIs' },
        { name: 'internal', displayName: 'Internal APIs' },
    ]],
    // createLabels echoes req.body — clients send LabelRequest[]
    ['createLabels', 201, [{ name: 'premium', displayName: 'Premium APIs' }]],

    // Subscription Policies — single-create returns DTO; bulk-disabled returns {message}
    ['addSubscriptionPolicies', 201, {
        policyID: 'p1', policyName: 'Bronze', displayName: 'Bronze',
        billingPlan: 'FREE', description: 'desc', requestCount: 100,
        orgID: 'org-1', pricingModel: 'flat', currency: 'usd',
        billingPeriod: 'month', flatAmount: 0, unitAmount: 0, pricingMetadata: {},
    }],
    ['addSubscriptionPolicies', 200, { message: 'Bulk creation disabled' }],

    // API Flows — apiFlowService.createAPIFlow L207 emits {apiFlowId, name, status}
    ['createAPIFlow', 201, { apiFlowId: 'f1', name: 'flow1', status: 'PUBLISHED' }],
    ['getAllAPIFlows', 200, [{
        apiFlowId: 'f1', name: 'flow1', handle: 'flow-1', description: 'desc',
        agentPrompt: 'prompt', status: 'PUBLISHED', visibility: 'PUBLIC',
        agentVisibility: 'VISIBLE', contentType: 'ARAZZO',
        apiFlowDefinition: '{}', markdownContent: null,
        createdAt: 'May 7, 2026', updatedAt: '2026-05-07T08:30:00Z',
    }]],

    // Billing engine keys — billingController L557 emits 4-field shape
    ['getBillingEngineKeys', 200, {
        billingEngine: 'STRIPE', secretKey: '****',
        publishableKey: '****', webhookSecret: '****',
    }],
    ['addBillingEngineKeys', 201, { message: 'Billing engine keys saved' }],
    ['updateBillingEngineKeys', 200, { message: 'Billing engine keys updated' }],
    ['deleteBillingEngineKeys', 200, { message: 'Billing engine keys deleted' }],

    // API Keys — apiKeyController (devportal source of truth, no CP lookup)
    // generateApiKey res.status(201).json({ keyId, name, key, expiresAt, status })
    ['generateApiKey', 201, {
        keyId: 'key-12345', name: 'weather_prod_key',
        key: 'ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ',
        expiresAt: '2026-12-31T23:59:59.000Z', status: 'ACTIVE',
    }],
    // listApiKeys res.status(200).json(keys.map(k => ({ keyId, name, status, expiresAt, createdAt, revokedAt?, apiId })))
    ['listApiKeys', 200, [{
        keyId: 'key-12345', name: 'weather_prod_key', status: 'ACTIVE',
        expiresAt: null, createdAt: '2026-05-07T08:30:00.000Z', apiId: 'api-7f4c2a6b',
    }]],
    // regenerateApiKey res.status(200).json({ keyId, name, key, expiresAt, status })
    ['regenerateApiKey', 200, {
        keyId: 'key-12345', name: 'weather_prod_key',
        key: 'ak_bmV3a2V5Zm9yZGVtb25zdHJhdGlvbg',
        expiresAt: null, status: 'ACTIVE',
    }],

    // Webhook Events — webhookAdminController maps Sequelize rows to camelCase DTOs
    // listEvents res.json({ total, events: rows.map(formatEvent) })
    // listEvents and getEvent both use formatEvent() — same shape with full delivery fields
    ['listWebhookEvents', 200, {
        total: 1,
        events: [{
            eventId: 'evt-abc123', eventType: 'apikey.generated',
            orgId: 'org-default', gatewayType: 'default',
            aggregateType: 'apikey', aggregateId: 'key-12345',
            status: 'ALL_DELIVERED', occurredAt: '2026-05-07T08:30:00.000Z',
            deliveries: [{
                deliveryId: 'del-abc123', subscriberId: 'sub-1',
                targetUrl: 'https://example.com/webhook',
                status: 'DELIVERED', attemptCount: 1,
                lastHttpStatus: 200, lastError: null,
                lastAttemptAt: '2026-05-07T08:30:01.000Z', deliveredAt: '2026-05-07T08:30:01.000Z',
            }],
        }],
    }],
    ['getWebhookEvent', 200, {
        eventId: 'evt-abc123', eventType: 'apikey.generated',
        orgId: 'org-default', gatewayType: 'default',
        aggregateType: 'apikey', aggregateId: 'key-12345',
        status: 'ALL_DELIVERED', occurredAt: '2026-05-07T08:30:00.000Z',
        deliveries: [{
            deliveryId: 'del-abc123', subscriberId: 'sub-1',
            targetUrl: 'https://example.com/webhook',
            status: 'DELIVERED', attemptCount: 1,
            lastHttpStatus: 200, lastError: null,
            lastAttemptAt: '2026-05-07T08:30:01.000Z', deliveredAt: '2026-05-07T08:30:01.000Z',
        }],
    }],
    // retryDelivery res.json({ message })
    ['retryWebhookDelivery', 200, { message: 'Delivery queued for retry' }],
];

const drifts = [];
for (const [opId, status, sample] of SAMPLES) {
    const schema = responseSchema(opId, status);
    if (!schema) {
        console.log('SKIP   ' + opId + ' ' + status + '  (no spec response found at this status)');
        continue;
    }
    if (schema.plainText) {
        console.log('skip   ' + opId + ' ' + status + '  (plain-text response in spec)');
        continue;
    }
    const r = validate(schema, sample);
    if (r.ok) {
        console.log('ok     ' + opId + ' ' + status);
    } else {
        console.log('DRIFT  ' + opId + ' ' + status);
        for (const e of r.errors) {
            const where = e.dataPath || e.instancePath || '';
            const params = e.params ? '  ' + JSON.stringify(e.params) : '';
            console.log('         ' + where + ' :: ' + e.message + params);
        }
        drifts.push({ opId, status, errors: r.errors });
    }
}

console.log('---');
console.log('drift items:', drifts.length, 'of', SAMPLES.length, 'samples');
process.exit(drifts.length > 0 ? 1 : 0);
