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
 *   node tools/drift_check.js
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
    fs.readFileSync(path.join(__dirname, '..', 'docs/devportal-openapi-spec-v0.9.yaml'), 'utf8')
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
    // Organizations — adminService.createOrganization/updateOrganization/
    // getAllOrganizations and devportalService.getOrganizationDetails all emit
    // {id, displayName, businessOwner, businessOwnerContact, businessOwnerEmail,
    // idpRefId, cpRefId, configuration, createdAt, updatedAt} (adminService.js
    // ~194-204). The previous org-prefixed field names (orgId/orgName/orgHandle/
    // orgConfiguration) haven't matched the real response in a long time.
    ['createOrganization', 201, {
        id: 'acme', displayName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', configuration: { devportalMode: 'DEFAULT' },
        createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
    }],
    ['getOrganizations', 200, {
        list: [{
            id: 'acme', displayName: 'Acme', businessOwner: 'Jane',
            businessOwnerContact: '+1', businessOwnerEmail: 'jane@acme.example',
            idpRefId: 'ACME', cpRefId: 'cp-ref-1', configuration: { devportalMode: 'DEFAULT' },
            createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],
    ['getOrganization', 200, {
        id: 'acme', displayName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', configuration: { devportalMode: 'DEFAULT' },
        createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
    }],
    ['updateOrganization', 200, {
        id: 'acme', displayName: 'Acme', businessOwner: 'Jane', businessOwnerContact: '+1',
        businessOwnerEmail: 'jane@acme.example',
        idpRefId: 'ACME', cpRefId: 'cp-ref-1', configuration: { devportalMode: 'DEFAULT' },
        createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
    }],

    // Org content — createOrgContent/updateOrgContent/getOrgLayoutContentByFileType
    // never existed as operationIds; that whole naming scheme was stale. The real,
    // documented operation is `applyTheme` (POST /views/{viewId}/apply-theme),
    // adminService.js:440-471 — a single upsert (atomic delete+recreate) covering
    // what the old create/update split assumed were separate operations:
    // `res.status(200).json({ id: organization.handle, fileName: zipFile.originalname })`.
    // `resetTheme` (204, no body) and `getOrgAsset` (200, raw file content — not JSON)
    // have nothing here to validate via AJV, so they're not worth a sample.
    ['applyTheme', 200, { id: 'acme', fileName: 'theme.zip' }],

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

    // Labels — apiMetadataService.js. The real operationIds are `listLabels`
    // (singular create is `createLabel`, not the invented plural "createLabels")
    // and `updateLabel` responds 200, not 201. Every one returns `new LabelDTO(...)`
    // (labelDto.js: {id, displayName} — not {name, displayName}), a single object
    // for create/get/update, and listLabels wraps via util.toPaginatedList.
    ['listLabels', 200, {
        list: [
            { id: 'premium', displayName: 'Premium APIs' },
            { id: 'internal', displayName: 'Internal APIs' },
        ],
        pagination: { total: 2, limit: 20, offset: 0 },
    }],
    ['createLabel', 201, { id: 'premium', displayName: 'Premium APIs' }],
    ['updateLabel', 200, { id: 'premium', displayName: 'Premium APIs' }],

    // Subscription Plans — subscriptionPlanDto.js's SubscriptionPlan class shape:
    // {id, displayName, description, refId, orgId, limits[]}, where each limit is
    // {limitType, timeUnit, timeAmount, limitCount} (limitCount is a number unless
    // it overflows a safe integer, in which case subscriptionPlanDao.js stringifies
    // it). There is no planId/planName/requestCount field — that shape predates the
    // current DTO. Single-create (createSubscriptionPlan) returns one object;
    // bulk-create (createSubscriptionPlans) returns an array, or a {message} when
    // generateDefaultSubPlans disables bulk create.
    ['addSubscriptionPlans', 201, {
        id: 'bronze', displayName: 'Bronze', description: 'desc',
        limits: [{ limitType: 'REQUEST_COUNT', timeUnit: 'MONTH', timeAmount: 1, limitCount: 1000 }],
        refId: null, orgId: 'org-1',
        createdBy: 'alice@example.com', updatedBy: 'alice@example.com',
        createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
    }],
    ['addSubscriptionPlans', 201, [{
        id: 'bronze', displayName: 'Bronze', description: 'desc',
        limits: [{ limitType: 'REQUEST_COUNT', timeUnit: 'MONTH', timeAmount: 1, limitCount: 1000 }],
        refId: null, orgId: 'org-1',
        createdBy: 'alice@example.com', updatedBy: 'alice@example.com',
        createdAt: '2026-05-07T08:30:00.000Z', updatedAt: '2026-05-07T08:30:00.000Z',
    }]],
    ['addSubscriptionPlans', 200, {
        message: "Bulk creation of subscription plans is not allowed because 'generateDefaultSubPlans' is enabled in the Developer Portal.",
    }],

    // API Workflows — apiWorkflowService.js. createAPIWorkflow:
    // `res.status(201).json({ id: apiWorkflow.handle, displayName: apiWorkflow.display_name,
    // status })` (~line 230) — the handle is returned under `id`, not a separate
    // apiWorkflowId/name pair (those never existed in this response). getAllAPIWorkflows
    // wraps toAPIWorkflowDTO(...) items (~line 403) via util.toPaginatedList; each item
    // also carries createdBy/updatedBy appended from audit data, on top of the DTO's own
    // fields (id, displayName, description, agentPrompt, status, agentVisibility,
    // contentType, apiWorkflowDefinition, markdownContent, createdAt, updatedAt).
    ['createApiWorkflow', 201, { id: 'workflow-1', displayName: 'Workflow 1', status: 'PUBLISHED' }],
    ['getAllApiWorkflows', 200, {
        list: [{
            id: 'workflow-1', displayName: 'Workflow 1', description: 'desc',
            agentPrompt: 'prompt', status: 'PUBLISHED',
            agentVisibility: 'VISIBLE', contentType: 'ARAZZO',
            apiWorkflowDefinition: '{}', markdownContent: null,
            createdAt: 'May 7, 2026', updatedAt: 'May 7, 2026',
            createdBy: 'alice@example.com', updatedBy: 'alice@example.com',
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],

    // API Keys — apiKeyController.js (devportal source of truth, no CP lookup).
    // apiKeyService.generate/regenerate return { keyId, id, displayName, key, expiresAt,
    // status, ...audit } (apiKeyService.js:197,248) — the key's handle is `id`, its
    // display name is `displayName`; there is no `name` field.
    ['generateApiKey', 201, {
        keyId: 'key-12345', id: 'weather_prod_key', displayName: 'Weather Prod Key',
        key: 'ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ',
        expiresAt: '2026-12-31T23:59:59.000Z', status: 'ACTIVE',
    }],
    // listApiKeys res.status(200).json(util.toPaginatedList(keys.map(mapKey), req)) —
    // mapKey (apiKeyController.js:85-99) emits { keyId, id, displayName, status,
    // expiresAt, createdAt, revokedAt?, apiId, appId, appDisplayName }.
    ['listApiKeys', 200, {
        list: [{
            keyId: 'key-12345', id: 'weather_prod_key', displayName: 'Weather Prod Key', status: 'ACTIVE',
            expiresAt: null, createdAt: '2026-05-07T08:30:00.000Z', apiId: 'api-7f4c2a6b',
            appId: null, appDisplayName: null,
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],
    // regenerateApiKey — same shape as generateApiKey, minus displayName being
    // re-derived (existing.display_name is carried through unchanged).
    ['regenerateApiKey', 200, {
        keyId: 'key-12345', id: 'weather_prod_key', displayName: 'Weather Prod Key',
        key: 'ak_bmV3a2V5Zm9yZGVtb25zdHJhdGlvbg',
        expiresAt: null, status: 'ACTIVE',
    }],

    // APIs — apiMetadataService.js. Verified against a live server, not just read
    // from source: create/update/get shapes below are actual captured response
    // bodies from IT probes run this session, not hand-derived.
    //
    // createAPIMetadata: `res.status(201).send({ ...apiMetadata, ...audit })` (line
    // ~247) — spreads the parsed request metadata (name, version, type, status,
    // agentVisibility, tags, labels, owners, endPoints, subscriptionPlans, the
    // resolved `id`) plus audit fields. NOT an APIDTO — no `refId`/`title`/
    // `remotes`/`apiImageMetadata`, unlike get/update below.
    ['createApiMetadata', 201, {
        name: 'Zip API', version: 'v1.0', description: 'first', type: 'RestApi',
        status: 'PUBLISHED', agentVisibility: 'VISIBLE', tags: [], labels: [], owners: {},
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/zip-api-1', productionURL: 'https://backend.example.invalid/zip-api-1' },
        subscriptionPlans: [], id: 'zip-api-1',
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
    }],
    // getApiMetadata: getMetadataFromDB wraps the DAO row in `new APIDTO(...)`
    // (apiDto.js) — a different, larger shape than create's.
    ['getApiMetadata', 200, {
        id: 'zip-api-1', refId: null, name: 'Zip API', title: null, remotes: [],
        version: 'v1.0', description: 'first', type: 'RestApi', status: 'PUBLISHED',
        agentVisibility: 'VISIBLE', apiImageMetadata: {}, tags: [], labels: [],
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/zip-api-1', productionURL: 'https://backend.example.invalid/zip-api-1' },
        subscriptionPlans: [],
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
    }],
    // updateAPIMetadata: `res.status(200).send(new APIDTO(updatedAPI[0].dataValues,
    // audit))` (line ~698) — an APIDTO like get, but observed without `apiImageMetadata`/
    // `tags`/`labels` present when the update request didn't touch those fields
    // (APIDTO only sets what its input row/audit actually carried at that point).
    ['updateApiMetadata', 200, {
        id: 'zip-api-1', refId: null, name: 'Zip API', title: null, remotes: [],
        version: 'v1.0', description: 'second (changed)', type: 'RestApi', status: 'PUBLISHED',
        agentVisibility: 'VISIBLE',
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/zip-api-1', productionURL: 'https://backend.example.invalid/zip-api-1' },
        subscriptionPlans: [],
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:52:41.706Z',
    }],
    // getAllApiMetadataForOrganization: util.toPaginatedList(retrievedAPIs.map(APIDTO)) —
    // same per-item shape as getApiMetadata.
    ['getAllApiMetadataForOrganization', 200, {
        list: [{
            id: 'zip-api-1', refId: null, name: 'Zip API', title: null, remotes: [],
            version: 'v1.0', description: 'first', type: 'RestApi', status: 'PUBLISHED',
            agentVisibility: 'VISIBLE', apiImageMetadata: {}, tags: [], labels: ['default'],
            endPoints: { sandboxURL: 'https://sandbox.example.invalid/zip-api-1', productionURL: 'https://backend.example.invalid/zip-api-1' },
            subscriptionPlans: [],
            createdBy: 'publisher', updatedBy: 'publisher',
            createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
    }],

    // MCP Servers — mcpServerService.js delegates every one of these straight into
    // the same apiMetadataService handlers above via asMcpRequest() (req.__forceApiType
    // = 'MCP'), so the response shape is identical apart from `type` — not a separate
    // implementation to independently verify.
    ['createMcpServer', 201, {
        name: 'Booking MCP', version: 'v1.0', description: 'first', type: 'Mcp',
        status: 'PUBLISHED', agentVisibility: 'VISIBLE', tags: [], labels: [], owners: {},
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/mcp-1', productionURL: 'https://backend.example.invalid/mcp-1' },
        subscriptionPlans: [], id: 'mcp-server-1',
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
    }],
    ['getMcpServer', 200, {
        id: 'mcp-server-1', refId: null, name: 'Booking MCP', title: null, remotes: [],
        version: 'v1.0', description: 'first', type: 'Mcp', status: 'PUBLISHED',
        agentVisibility: 'VISIBLE', apiImageMetadata: {}, tags: [], labels: [],
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/mcp-1', productionURL: 'https://backend.example.invalid/mcp-1' },
        subscriptionPlans: [],
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
    }],
    ['updateMcpServer', 200, {
        id: 'mcp-server-1', refId: null, name: 'Booking MCP', title: null, remotes: [],
        version: 'v1.0', description: 'second (changed)', type: 'Mcp', status: 'PUBLISHED',
        agentVisibility: 'VISIBLE',
        endPoints: { sandboxURL: 'https://sandbox.example.invalid/mcp-1', productionURL: 'https://backend.example.invalid/mcp-1' },
        subscriptionPlans: [],
        createdBy: 'publisher', updatedBy: 'publisher',
        createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:52:41.706Z',
    }],
    ['getAllMcpServersForOrganization', 200, {
        list: [{
            id: 'mcp-server-1', refId: null, name: 'Booking MCP', title: null, remotes: [],
            version: 'v1.0', description: 'first', type: 'Mcp', status: 'PUBLISHED',
            agentVisibility: 'VISIBLE', apiImageMetadata: {}, tags: [], labels: ['default'],
            endPoints: { sandboxURL: 'https://sandbox.example.invalid/mcp-1', productionURL: 'https://backend.example.invalid/mcp-1' },
            subscriptionPlans: [],
            createdBy: 'publisher', updatedBy: 'publisher',
            createdAt: '2026-07-07T12:48:30.875Z', updatedAt: '2026-07-07T12:48:30.875Z',
        }],
        pagination: { total: 1, limit: 20, offset: 0 },
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
