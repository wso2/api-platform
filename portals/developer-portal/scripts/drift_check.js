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
    // Organizations — adminService.createOrganization/updateOrganization and
    // devportalService.getOrganizationDetails all emit {orgId, orgName, businessOwner,
    // businessOwnerContact, businessOwnerEmail, orgHandle, idpRefId,
    // cpRefId, orgConfiguration}. No IDP claim-mapping fields (roleClaimName etc.) —
    // those were removed from the response shape long ago.
    ['createOrganization', 201, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', orgConfiguration: {},
    }],
    // adminService.getAllOrganizations builds the same shape per item (orgId, not
    // orgID); adminService.getOrganizations wraps it via util.toPaginatedList.
    ['getOrganizations', 200, {
        list: [{
            orgName: 'Acme', orgId: 'org-1', businessOwner: 'Jane',
            businessOwnerContact: '+1', businessOwnerEmail: 'jane@acme.example',
            orgHandle: 'acme', idpRefId: 'ACME', cpRefId: 'cp-ref-1',
            orgConfiguration: {},
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],
    ['getOrganization', 200, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', orgConfiguration: {},
    }],
    ['updateOrganization', 200, {
        orgId: 'org-1', orgName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example', orgHandle: 'acme',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', orgConfiguration: {},
    }],

    // Org content — adminService.createOrgContent/updateOrgContent both
    // res.status(201).send({ orgId, fileName }).
    ['createOrgContent', 201, { orgId: 'org-1', fileName: 'theme.zip' }],
    ['updateOrgContent', 201, { orgId: 'org-1', fileName: 'theme.zip' }],
    // devportalService.getOrgContent (fileType-only branch) res.status(200).send(results) —
    // a bare array, not paginated.
    ['getOrgLayoutContentByFileType', 200, [
        { orgId: 'org-1', fileName: 'main.css', fileContent: 'body{}' },
    ]],

    // Subscriptions — subscriptionService.formatSubscriptionResponse shape
    ['createSubscription', 201, {
        subscriptionId: 'sub-12345', subscriptionToken: 'tok-abc123',
        status: 'ACTIVE',
        apiId: 'api-7f4c2a6b', subscriptionPlanName: 'Gold',
        createdBy: 'alice@example.com',
        createdAt: '2026-05-07T08:30:00.000Z',
    }],
    ['updateSubscription', 200, {
        subscriptionId: 'sub-12345', subscriptionToken: 'tok-abc123',
        status: 'INACTIVE',
        apiId: 'api-7f4c2a6b', subscriptionPlanName: 'Gold',
        createdBy: 'alice@example.com',
        createdAt: '2026-05-07T08:30:00.000Z',
    }],

    // Labels — apiMetadataService.retrieveLabels wraps via util.toPaginatedList;
    // createLabels/updateLabel echo req.body verbatim (clients send LabelRequest[]).
    ['retrieveLabels', 200, {
        list: [
            { name: 'premium', displayName: 'Premium APIs' },
            { name: 'internal', displayName: 'Internal APIs' },
        ],
        pagination: { total: 2, limit: 20, offset: 0 },
    }],
    ['createLabels', 201, [{ name: 'premium', displayName: 'Premium APIs' }]],
    ['updateLabel', 201, [{ name: 'premium', displayName: 'Premium APIs' }]],

    // Subscription Plans — subscriptionPlanDto.js shape: {planId, planName, displayName,
    // description, requestCount, refId, orgId}. requestCount is always a string or null
    // (computed in subscriptionPlanDao.js), never a raw number. Single-create
    // (createSubscriptionPlan) returns one object; bulk-create (createSubscriptionPlans)
    // returns an array, or a {message} when generateDefaultSubPlans disables bulk create.
    ['addSubscriptionPlans', 201, {
        planId: 'p1', planName: 'bronze', displayName: 'Bronze',
        description: 'desc', requestCount: '1000', refId: null, orgId: 'org-1',
    }],
    ['addSubscriptionPlans', 201, [{
        planId: 'p1', planName: 'bronze', displayName: 'Bronze',
        description: 'desc', requestCount: '1000', refId: null, orgId: 'org-1',
    }]],
    ['addSubscriptionPlans', 200, {
        message: "Bulk creation of subscription plans is not allowed because 'generateDefaultSubPlans' is enabled in the Developer Portal.",
    }],

    // API Workflows — apiWorkflowService.createAPIWorkflow res.status(201).json({apiWorkflowId, name, status});
    // getAllAPIWorkflows wraps toAPIWorkflowDTO(...) items via util.toPaginatedList.
    ['createApiWorkflow', 201, { apiWorkflowId: 'w1', name: 'workflow1', status: 'PUBLISHED' }],
    ['getAllApiWorkflows', 200, {
        list: [{
            apiWorkflowId: 'w1', name: 'workflow1', handle: 'workflow-1', description: 'desc',
            agentPrompt: 'prompt', status: 'PUBLISHED',
            agentVisibility: 'VISIBLE', contentType: 'ARAZZO',
            apiWorkflowDefinition: '{}', markdownContent: null,
            createdAt: 'May 7, 2026', updatedAt: '2026-05-07T08:30:00Z',
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],

    // API Keys — apiKeyController (devportal source of truth, no CP lookup)
    // generateApiKey res.status(201).json({ keyId, name, key, expiresAt, status })
    ['generateApiKey', 201, {
        keyId: 'key-12345', name: 'weather_prod_key',
        key: 'ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ',
        expiresAt: '2026-12-31T23:59:59.000Z', status: 'ACTIVE',
    }],
    // listApiKeys res.status(200).json(util.toPaginatedList(keys.map(mapKey), req)) —
    // mapKey emits { keyId, name, status, expiresAt, createdAt, revokedAt?, apiId, appId, appName }.
    ['listApiKeys', 200, {
        list: [{
            keyId: 'key-12345', name: 'weather_prod_key', status: 'ACTIVE',
            expiresAt: null, createdAt: '2026-05-07T08:30:00.000Z', apiId: 'api-7f4c2a6b',
            appId: null, appName: null,
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],
    // regenerateApiKey res.status(200).json({ keyId, name, key, expiresAt, status })
    ['regenerateApiKey', 200, {
        keyId: 'key-12345', name: 'weather_prod_key',
        key: 'ak_bmV3a2V5Zm9yZGVtb25zdHJhdGlvbg',
        expiresAt: null, status: 'ACTIVE',
    }],

    // Webhook Events — webhookAdminController.formatEvent() shape.
    // listEvents res.json({ list: rows.map(formatEvent), pagination }) — NOT
    // { total, events } as a previous version of this comment claimed (that stale
    // shape happened to validate anyway because the spec schema has no required
    // fields, masking the drift instead of catching it).
    ['listWebhookEvents', 200, {
        list: [{
            eventId: 'evt-abc123', eventType: 'apikey.generated',
            orgId: 'org-default',
            aggregateType: 'apikey', aggregateId: 'key-12345',
            status: 'ALL_DELIVERED', occurredAt: '2026-05-07T08:30:00.000Z',
            deliveries: [{
                deliveryId: 'del-abc123', subscriberId: 'sub-1',
                targetUrl: 'https://example.com/webhook',
                status: 'DELIVERED',
                lastHttpStatus: 200, lastError: null,
                lastAttemptAt: '2026-05-07T08:30:01.000Z', deliveredAt: '2026-05-07T08:30:01.000Z',
            }],
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],
    // getEvent res.json(formatEvent(event)) — a single object, not wrapped.
    ['getWebhookEvent', 200, {
        eventId: 'evt-abc123', eventType: 'apikey.generated',
        orgId: 'org-default',
        aggregateType: 'apikey', aggregateId: 'key-12345',
        status: 'ALL_DELIVERED', occurredAt: '2026-05-07T08:30:00.000Z',
        deliveries: [{
            deliveryId: 'del-abc123', subscriberId: 'sub-1',
            targetUrl: 'https://example.com/webhook',
            status: 'DELIVERED',
            lastHttpStatus: 200, lastError: null,
            lastAttemptAt: '2026-05-07T08:30:01.000Z', deliveredAt: '2026-05-07T08:30:01.000Z',
        }],
    }],
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
