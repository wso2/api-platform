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

// ---------------------------------------------------------------------------
// Seed helpers — create demo REST APIs / MCP servers through the real devportal
// management API (POST /api/v0.9/apis, /mcp-servers) so UI browse tests have
// something to render. They go through cy.apiRequest, which injects the
// service API-key header; the `organization` header selects the target org
// (authMiddleware.resolveOrgFromHeader), and `labels: ['default']` maps the
// resource into the default view so it appears on the /apis and /mcps listings
// (apiDao.list requires a label mapped to the view).
//
// These endpoints take multipart/form-data. cy.request runs in Node, not the
// browser, so a browser FormData won't serialize — instead we build the
// multipart body by hand. Every part here is UTF-8 text (a JSON metadata field
// and a JSON definition "file"), so a plain string body is sufficient.
// ---------------------------------------------------------------------------

function buildMultipart(parts) {
    const boundary = `----dpItBoundary${Date.now()}${Math.floor(Math.random() * 1e6)}`;
    let body = '';
    for (const part of parts) {
        body += `--${boundary}\r\n`;
        if (part.filename) {
            body += `Content-Disposition: form-data; name="${part.name}"; filename="${part.filename}"\r\n`;
            body += `Content-Type: ${part.contentType || 'application/octet-stream'}\r\n\r\n`;
        } else {
            body += `Content-Disposition: form-data; name="${part.name}"\r\n\r\n`;
        }
        body += `${part.value}\r\n`;
    }
    body += `--${boundary}--\r\n`;
    return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

function seedHeaders(contentType) {
    return {
        // authMiddleware resolves the target org from this header for API-key requests.
        organization: Cypress.env('ORG_HANDLE'),
        'content-type': contentType,
    };
}

// ---------------------------------------------------------------------------
// cy.seedApi(overrides)
//   Create a PUBLISHED REST API in the default view and return its handle.
//   Defaults produce a minimal API whose OpenAPI definition declares an apiKey
//   security scheme (so the per-API "API Keys" action renders — showApiKeysNav →
//   apiUsesApiKeySecurity). Overrides let a richer fixture be built:
//     - name, version, id, endPoints
//     - subscriptionPlans: [{ id }]  — links org-level plans so the plans
//         section renders (the plan must already exist, e.g. the default
//         "Bronze"/"Silver"/"Gold"; this only links, it does not create).
//     - definition: an OpenAPI object (or JSON string) to replace the default
//         (e.g. more paths → more Resources rows, oauth2 scopes for the spec view).
//     - docs: [{ name, content }] — markdown files stored as "Other" documents,
//         which surface in the docs sidebar at /docs/Other/<name>.
// ---------------------------------------------------------------------------
Cypress.Commands.add('seedApi', (overrides = {}) => {
    const handle = overrides.id || `it-portal-access-api-${Date.now()}`;
    const metadata = {
        id: handle,
        name: overrides.name || 'IT Portal Access API',
        version: overrides.version || 'v1.0',
        type: 'REST',
        status: 'PUBLISHED',
        // Maps the API into the default view so it appears on the /apis listing.
        labels: ['default'],
        endPoints: overrides.endPoints || {
            productionURL: `https://backend.example.invalid/${handle}`,
            sandboxURL: `https://sandbox.example.invalid/${handle}`,
        },
    };
    if (overrides.subscriptionPlans) {
        metadata.subscriptionPlans = overrides.subscriptionPlans;
    }

    const definition = overrides.definition
        ? (typeof overrides.definition === 'string' ? overrides.definition : JSON.stringify(overrides.definition))
        : JSON.stringify({
            openapi: '3.0.3',
            info: { title: metadata.name, version: '1.0.0' },
            // apiKey scheme → the API Keys button shows on the API detail page.
            components: { securitySchemes: { ApiKeyAuth: { type: 'apiKey', in: 'header', name: 'apikey' } } },
            security: [{ ApiKeyAuth: [] }],
            paths: { '/ping': { get: { responses: { 200: { description: 'ok' } } } } },
        });

    const parts = [
        { name: 'metadata', value: JSON.stringify(metadata) },
        { name: 'definition', value: definition, filename: 'definition.json', contentType: 'application/json' },
    ];
    // Each `docs` file is stored as an "Other" document (req.files.docs in
    // apiMetadataService.createAPIMetadata).
    (overrides.docs || []).forEach((doc) => {
        parts.push({ name: 'docs', value: doc.content, filename: doc.name, contentType: 'text/markdown' });
    });

    const { body, contentType } = buildMultipart(parts);

    return cy
        .apiRequest('POST', '/api/v0.9/apis', { headers: seedHeaders(contentType), body })
        .then((resp) => {
            expect(resp.status, 'seed REST API').to.eq(201);
            return handle;
        });
});

// ---------------------------------------------------------------------------
// cy.seedMcp(overrides)
//   Create a PUBLISHED MCP server in the default view. Its definition (the tools
//   schema) is required on create. Returns the MCP handle.
// ---------------------------------------------------------------------------
Cypress.Commands.add('seedMcp', (overrides = {}) => {
    const handle = overrides.id || `it-portal-access-mcp-${Date.now()}`;
    const metadata = {
        id: handle,
        name: overrides.name || 'IT Portal Access MCP',
        version: overrides.version || 'v1.0',
        type: 'MCP',
        status: 'PUBLISHED',
        labels: ['default'],
        endPoints: {
            productionURL: `https://mcp.example.invalid/${handle}`,
            sandboxURL: `https://mcp.example.invalid/${handle}`,
        },
    };
    // MCP definition is a list of tools/resources/prompts (JSON is valid YAML).
    const definition = JSON.stringify([
        {
            type: 'TOOL',
            name: 'ping',
            description: 'Health-check tool.',
            inputSchema: { type: 'object', properties: {} },
        },
    ]);

    const { body, contentType } = buildMultipart([
        { name: 'metadata', value: JSON.stringify(metadata) },
        { name: 'definition', value: definition, filename: 'definition.json', contentType: 'application/json' },
    ]);

    return cy
        .apiRequest('POST', '/api/v0.9/mcp-servers', { headers: seedHeaders(contentType), body })
        .then((resp) => {
            expect(resp.status, 'seed MCP server').to.eq(201);
            return handle;
        });
});

// ---------------------------------------------------------------------------
// cy.deleteApi(handle) / cy.deleteMcp(handle)
//   Best-effort teardown — never fails the suite if the resource is already gone.
// ---------------------------------------------------------------------------
Cypress.Commands.add('deleteApi', (handle) => {
    if (!handle) return;
    cy.apiRequest('DELETE', `/api/v0.9/apis/${handle}`, {
        headers: { organization: Cypress.env('ORG_HANDLE') },
        failOnStatusCode: false,
    });
});

Cypress.Commands.add('deleteMcp', (handle) => {
    if (!handle) return;
    cy.apiRequest('DELETE', `/api/v0.9/mcp-servers/${handle}`, {
        headers: { organization: Cypress.env('ORG_HANDLE') },
        failOnStatusCode: false,
    });
});
