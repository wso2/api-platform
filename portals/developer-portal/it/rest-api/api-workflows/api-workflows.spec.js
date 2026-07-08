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

// POST/GET/PUT/DELETE /views/{viewId}/api-workflows (Arazzo-based agent workflows).
// APIWorkflowCreateRequest requires { displayName, description }; content goes in
// `apiWorkflowDefinition` (JSON/YAML) when contentType is the default ARAZZO.
// `admin` manages the view/label setup; `publisher` manages workflow content.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const SAMPLE_ARAZZO = {
    arazzo: '1.0.0',
    info: { title: 'Sample workflow', version: '1.0.0' },
    sourceDescriptions: [],
    workflows: [],
};

async function createView(overrides = {}) {
    // ViewCreateRequest requires labels to have at least one entry.
    const labelId = uniqueHandle('label');
    await client.as('admin').post('/labels', { id: labelId, displayName: labelId });

    const id = overrides.id || uniqueHandle('view');
    const res = await client.as('admin').post('/views', { id, displayName: overrides.displayName || id, labels: [labelId] });
    if (res.status !== 201) {
        throw new Error(`Failed to seed view: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return { id };
}

describe('API workflows', () => {
    let view;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('publisher');
    });

    beforeEach(async () => {
        view = await createView();
    });

    it('creates an API workflow (Arazzo file_content + agent_prompt)', async () => {
        const id = uniqueHandle('workflow');
        const res = await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id,
            displayName: 'Weather Onboarding',
            description: 'Guides users through the Weather API onboarding workflow.',
            apiWorkflowDefinition: SAMPLE_ARAZZO,
        });
        expect(res.status).toBe(201);
    });

    it('retrieves an API workflow', async () => {
        const id = uniqueHandle('workflow');
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id, displayName: 'Retrievable Workflow', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO,
        });

        const res = await client.as('publisher').get(`/views/${view.id}/api-workflows/${id}`);
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Retrievable Workflow');
    });

    it('updates an API workflow', async () => {
        const id = uniqueHandle('workflow');
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id, displayName: 'Original Workflow', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO,
        });

        const res = await client.as('publisher').put(`/views/${view.id}/api-workflows/${id}`, {
            displayName: 'Updated Workflow',
        });
        expect(res.status).toBe(200);

        const get = await client.as('publisher').get(`/views/${view.id}/api-workflows/${id}`);
        expect(get.body.displayName).toBe('Updated Workflow');
    });

    it('deletes an API workflow', async () => {
        const id = uniqueHandle('workflow');
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id, displayName: 'To Delete', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO,
        });

        const del = await client.as('publisher').del(`/views/${view.id}/api-workflows/${id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/views/${view.id}/api-workflows/${id}`);
        expect(get.status).toBe(404);
    });

    it('lists API workflows for a view', async () => {
        const id = uniqueHandle('workflow');
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id, displayName: 'Listed Workflow', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO,
        });

        const res = await client.as('publisher').get(`/views/${view.id}/api-workflows`);
        expect(res.status).toBe(200);
        expect(res.body.list.some((w) => w.id === id)).toBe(true);
    });

    it('includes HIDDEN workflows in the publisher listing with their agentVisibility flag', async () => {
        const visibleId = uniqueHandle('workflow-visible');
        const hiddenId = uniqueHandle('workflow-hidden');
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id: visibleId, displayName: 'Visible', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO, agentVisibility: 'VISIBLE',
        });
        await client.as('publisher').post(`/views/${view.id}/api-workflows`, {
            id: hiddenId, displayName: 'Hidden', description: 'd', apiWorkflowDefinition: SAMPLE_ARAZZO, agentVisibility: 'HIDDEN',
        });

        const res = await client.as('publisher').get(`/views/${view.id}/api-workflows`);
        expect(res.body.list.some((w) => w.id === visibleId)).toBe(true);
        const hidden = res.body.list.find((w) => w.id === hiddenId);
        expect(hidden).toBeDefined();
        expect(hidden.agentVisibility).toBe('HIDDEN');
    });
});
