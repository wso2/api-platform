// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Seed helpers that go through the real REST API (not the DAO layer) so every
// spec exercises the same validation/authorization path production traffic
// does. Every fixture operates within the single shared org (client.ORG_HANDLE)
// that the admin/publisher/developer accounts are seeded into — resource
// uniqueness comes from uniqueHandle(), not a fresh org per test. Callers must
// have already done `await client.login(role)` for whichever role a fixture uses.

const crypto = require('crypto');

const client = require('./client');

function uniqueHandle(prefix) {
    return `${prefix}-${crypto.randomUUID()}`;
}

// Seeds a view (ViewCreateRequest requires at least one label). Seeds its own
// label unless the caller passes `overrides.labels` (specs that manage label
// visibility themselves supply their own label ids).
async function createView(overrides = {}) {
    let labels = overrides.labels;
    if (!labels) {
        const labelId = uniqueHandle('label');
        // client.post auto-tracks the label + view for afterAll cleanup.
        await client.as('admin').post('/labels', { id: labelId, displayName: labelId });
        labels = [labelId];
    }
    const id = overrides.id || uniqueHandle('view');
    const res = await client.as('admin').post('/views', { id, displayName: overrides.displayName || id, labels });
    if (res.status !== 201) {
        throw new Error(`Failed to seed view: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return { id };
}

// Only used by organizations.spec.js's own CRUD tests — org creation isn't
// scoped to the caller's own org, so admin can manage additional orgs beyond
// the fixed one, even though no file-based account can ever log into them.
async function createOrganization(overrides = {}) {
    const id = overrides.id || uniqueHandle('org');
    const res = await client.as('admin').post('/organizations', {
        id,
        displayName: overrides.displayName || `Test Org ${id}`,
        idpRefId: overrides.idpRefId || id,
        ...overrides,
    });
    if (res.status !== 201) {
        throw new Error(`Failed to seed organization: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

async function deleteOrganization(handleOrId) {
    await client.as('admin').del(`/organizations/${handleOrId}`);
}

const MINIMAL_OPENAPI_DEFINITION = JSON.stringify({
    openapi: '3.0.3',
    info: { title: 'Fixture API', version: '1.0.0' },
    paths: { '/ping': { get: { responses: { 200: { description: 'ok' } } } } },
});

// POST /apis takes multipart/form-data: `metadata` (JSON string) + `definition`
// (file) — see docs/devportal-openapi-spec-v0.9.yaml ApiMetadataMultipartBody.
// `publisher` holds the API-management scopes; pass `role` to override.
async function createApi(overrides = {}) {
    const { definition: _definition, definitionFileName: _definitionFileName, role = 'publisher', ...metadataOverrides } = overrides;
    const id = overrides.id || uniqueHandle('api');
    const metadata = {
        id,
        name: overrides.name || `Test API ${id}`,
        version: overrides.version || 'v1.0',
        type: overrides.type || 'REST',
        status: overrides.status || 'PUBLISHED',
        // Required — createAPIMetadata rejects with 400 if endPoints is missing
        // (src/services/apiMetadataService.js).
        endPoints: overrides.endPoints || {
            productionURL: `https://backend.example.invalid/${id}`,
            sandboxURL: `https://sandbox.example.invalid/${id}`,
        },
        ...metadataOverrides,
    };
    // Extension drives validation in apiMetadataService.js's
    // prepareApiDefinitionForStorage (.json parsed as JSON, .wsdl/.xml as text) —
    // must match the actual content, not just be any file.
    const definitionFileName = overrides.definitionFileName || 'definition.json';
    const res = await client
        .as(role)
        .postMultipart('/apis')
        .field('metadata', JSON.stringify(metadata))
        .attach('definition', Buffer.from(overrides.definition || MINIMAL_OPENAPI_DEFINITION), definitionFileName);
    if (res.status !== 201) {
        throw new Error(`Failed to seed API: ${res.status} ${JSON.stringify(res.body)}`);
    }
    // client.postMultipart auto-tracks the created API/MCP for afterAll cleanup.
    return res.body;
}

// `admin` manages org-level integration config; pass `role` to override.
async function createWebhookSubscriber(overrides = {}) {
    const { role = 'admin', ...bodyOverrides } = overrides;
    const id = overrides.id || uniqueHandle('subscriber');
    const res = await client.as(role).post('/webhook-subscribers', {
        id,
        targetUrl: overrides.targetUrl,
        events: overrides.events || [],
        enabled: overrides.enabled !== undefined ? overrides.enabled : true,
        ...bodyOverrides,
    });
    if (res.status !== 201) {
        throw new Error(`Failed to seed webhook subscriber: ${res.status} ${JSON.stringify(res.body)}`);
    }
    // client.post auto-tracks the subscriber for afterAll cleanup.
    return res.body;
}

module.exports = {
    uniqueHandle,
    createView,
    createOrganization,
    deleteOrganization,
    createApi,
    createWebhookSubscriber,
};
