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

// POST /views/{viewId}/api-workflows/generate-prompt — generates the LLM agent
// execution prompt text from a proposed workflow's name/description/APIs, before
// the workflow itself necessarily exists (APIWorkflowPromptRequest requires only
// { displayName, description }). `admin` manages the view/label setup;
// `publisher` manages workflow content.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

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

describe('API workflow prompt generation', () => {
    let view;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('publisher');
    });

    beforeEach(async () => {
        view = await createView();
    });

    it('generates an agent prompt from a workflow definition', async () => {
        const res = await client.as('publisher').post(`/views/${view.id}/api-workflows/generate-prompt`, {
            displayName: 'Weather Onboarding',
            description: 'Guides users through the Weather API onboarding workflow.',
        });
        expect(res.status).toBe(200);
        expect(typeof res.body.agentPrompt).toBe('string');
        expect(res.body.agentPrompt.length).toBeGreaterThan(0);
    });

    it('rejects prompt generation missing the required fields', async () => {
        const res = await client.as('publisher').post(`/views/${view.id}/api-workflows/generate-prompt`, {
            displayName: 'Missing Description',
        });
        expect(res.status).toBe(400);
    });
});
