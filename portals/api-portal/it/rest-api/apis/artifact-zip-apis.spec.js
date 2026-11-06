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

// POST/PUT /apis via the `artifact` full-ZIP upload path (as opposed to the JSON
// `metadata` field or the standalone `api`/`definition` YAML pair) — see
// apiMetadataService.js's extractFullApiBundleFromUploadedZip. A ZIP must contain,
// at its top level, one of metadata.yaml/metadata.yml/api.yaml/mcp.yaml plus a
// definition file (definition.(yaml|yml|json)); its optional `web`/`docs` content
// follows the same entry rules as the /assets content ZIP.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

const SAMPLE_DEFINITION = JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} });

function buildApiYaml({ handle, displayName, version, description, labels }) {
    const labelsLine = labels !== undefined ? `  labels: [${labels.map((l) => `"${l}"`).join(', ')}]\n` : '';
    return `metadata:\n  name: ${handle}\nspec:\n  displayName: "${displayName}"\n  version: "${version}"\n  description: "${description}"\n  type: REST\n  status: PUBLISHED\n${labelsLine}  endpoints:\n    sandboxUrl: https://sandbox.example.invalid/${handle}\n    productionUrl: https://backend.example.invalid/${handle}\n`;
}

function buildZip(overrides = {}) {
    const handle = overrides.handle || uniqueHandle('zip-api');
    const entries = [
        {
            name: 'api.yaml',
            content: buildApiYaml({
                handle,
                displayName: overrides.displayName || `Zip API ${handle}`,
                version: overrides.version || 'v1.0',
                description: overrides.description || 'created from a zip artifact',
                labels: overrides.labels,
            }),
        },
        { name: 'definition.json', content: overrides.definition || SAMPLE_DEFINITION },
        ...(overrides.extra || []),
    ];
    return { handle, zip: createZip(entries) };
}

async function createApiFromZip(overrides = {}) {
    const { handle, zip } = buildZip(overrides);
    const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
    if (res.status !== 201) {
        throw new Error(`Failed to seed API from zip: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return { handle, body: res.body };
}

describe('APIs via artifact ZIP upload', () => {
    beforeAll(async () => {
        await client.login('publisher');
        await client.login('admin');
    });

    it('creates a REST API from an artifact ZIP', async () => {
        const { handle } = await createApiFromZip({ displayName: 'Created From Zip', description: 'first version' });

        const get = await client.as('publisher').get(`/apis/${handle}`);
        expect(get.status).toBe(200);
        expect(get.body.name).toBe('Created From Zip');
        expect(get.body.description).toBe('first version');
        expect(get.body.type).toBe('RestApi');
        expect(get.body.endPoints.productionURL).toBe(`https://backend.example.invalid/${handle}`);
    });

    it('rejects an artifact ZIP missing a metadata file (metadata.yaml/api.yaml/mcp.yaml)', async () => {
        const zip = createZip([{ name: 'definition.json', content: SAMPLE_DEFINITION }]);
        const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        expect(res.status).toBe(400);
    });

    it('rejects an artifact ZIP missing a definition file', async () => {
        const handle = uniqueHandle('zip-api');
        const zip = createZip([{ name: 'api.yaml', content: buildApiYaml({ handle, displayName: 'No Definition', version: 'v1.0', description: 'd' }) }]);
        const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        expect(res.status).toBe(400);
    });

    it('updates a REST API via an artifact ZIP upload', async () => {
        const { handle } = await createApiFromZip({ displayName: 'Original Via Zip', description: 'original' });

        const { zip: updateZip } = buildZip({ handle, displayName: 'Updated Via Zip', description: 'updated' });
        const put = await client.as('publisher').putMultipart(`/apis/${handle}`).attach('artifact', updateZip, 'artifact.zip');
        expect(put.status).toBe(200);

        const get = await client.as('publisher').get(`/apis/${handle}`);
        expect(get.body.name).toBe('Updated Via Zip');
        expect(get.body.description).toBe('updated');
    });

    // The specific scenario worth checking on its own: create via zip, then re-upload
    // essentially the same zip with one small field changed. Both the metadata handle
    // (metadata.name in the YAML, not spec.displayName) and the API's own `id` must
    // stay stable across the round-trip — apiDao.update() never touches the handle
    // (it is immutable after creation), so no update path can
    // drift the id/handle as a side effect.
    it('creates via zip then updates the same API with the same zip plus a small change', async () => {
        const { handle, zip: firstZip } = buildZip({ displayName: 'Stable Zip API', description: 'version one' });
        const create = await client.as('publisher').postMultipart('/apis').attach('artifact', firstZip, 'artifact.zip');
        expect(create.status).toBe(201);
        expect(create.body.id).toBe(handle);

        // Same handle/displayName/version — only `description` differs.
        const { zip: secondZip } = buildZip({ handle, displayName: 'Stable Zip API', description: 'version two (small change)' });
        const put = await client.as('publisher').putMultipart(`/apis/${handle}`).attach('artifact', secondZip, 'artifact.zip');
        expect(put.status).toBe(200);
        expect(put.body.id).toBe(handle);

        const get = await client.as('publisher').get(`/apis/${handle}`);
        expect(get.status).toBe(200);
        expect(get.body.id).toBe(handle);
        expect(get.body.name).toBe('Stable Zip API');
        expect(get.body.description).toBe('version two (small change)');
        expect(get.body.type).toBe('RestApi');
        expect(get.body.endPoints.productionURL).toBe(`https://backend.example.invalid/${handle}`);
    });

    describe('label handling via ZIP (spec.labels)', () => {
        // mapArtifactYamlToApiMetadata used to run every `spec.labels` through
        // util.normalizeStringArray, which returns `[]` whether the YAML omitted
        // `labels` entirely or set it to an empty list — collapsing "not mentioned"
        // and "explicitly none" into the same value. That broke both directions:
        // create defaulted to no labels instead of ['default'] when omitted, and
        // update wiped every existing label on any change that didn't re-list them.
        it('defaults to the "default" label when a create ZIP omits spec.labels', async () => {
            const { handle } = await createApiFromZip({ displayName: 'No Labels Field' });
            const get = await client.as('publisher').get(`/apis/${handle}`);
            expect(get.body.labels).toEqual(['default']);
        });

        it('attaches no labels when a create ZIP explicitly sets spec.labels to an empty list', async () => {
            const { handle } = await createApiFromZip({ displayName: 'Explicit Empty Labels', labels: [] });
            const get = await client.as('publisher').get(`/apis/${handle}`);
            expect(get.body.labels).toEqual([]);
        });

        it('leaves existing labels untouched when an update ZIP omits spec.labels', async () => {
            const labelId = uniqueHandle('label');
            await client.as('admin').post('/labels', { id: labelId, displayName: 'Zip Label' });
            const { handle } = await createApiFromZip({ displayName: 'Keeps Labels', labels: [labelId] });

            const { zip: updateZip } = buildZip({ handle, displayName: 'Keeps Labels', description: 'unrelated change' });
            const put = await client.as('publisher').putMultipart(`/apis/${handle}`).attach('artifact', updateZip, 'artifact.zip');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${handle}`);
            expect(get.body.labels).toEqual([labelId]);
        });

        it('clears existing labels when an update ZIP explicitly sets spec.labels to an empty list', async () => {
            const labelId = uniqueHandle('label');
            await client.as('admin').post('/labels', { id: labelId, displayName: 'Zip Label' });
            const { handle } = await createApiFromZip({ displayName: 'Clears Labels', labels: [labelId] });

            const { zip: updateZip } = buildZip({ handle, displayName: 'Clears Labels', description: 'cleared', labels: [] });
            const put = await client.as('publisher').putMultipart(`/apis/${handle}`).attach('artifact', updateZip, 'artifact.zip');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${handle}`);
            expect(get.body.labels).toEqual([]);
        });
    });

    describe('content in the ZIP follows the content ZIP entry rules', () => {
        const post = (zip) => client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        const rejected = (res) => {
            expect(res.status).toBe(400);
            expect(res.body.code).toBe('COMMON_VALIDATION_ERROR');
            return res.body.errors.map((e) => e.message);
        };

        it('stores a document from docs/ as category Other', async () => {
            const { handle, zip } = buildZip({ extra: [{ name: 'docs/A.md', content: '# A' }] });
            expect((await post(zip)).status).toBe(201);
            const doc = await client.as('publisher').get(`/apis/${handle}/assets?type=DOC_Other&fileName=A.md`);
            expect(doc.status).toBe(200);
        });

        it('rejects a document that is not Markdown and creates nothing', async () => {
            const { handle, zip } = buildZip({ extra: [{ name: 'docs/guide.pdf', content: 'pdf' }] });
            expect(rejected(await post(zip))).toEqual(['docs/guide.pdf: documents must be Markdown (.md)']);
            expect((await client.as('publisher').get(`/apis/${handle}`)).status).toBe(404);
        });

        it('reads the metadata file only from the top level', async () => {
            const handle = uniqueHandle('zip-api');
            const zip = createZip([
                { name: 'docs/api.yaml', content: buildApiYaml({ handle, displayName: 'Nested', version: 'v1.0', description: 'd' }) },
                { name: 'definition.json', content: SAMPLE_DEFINITION },
            ]);
            expect(rejected(await post(zip))).toEqual(['docs/api.yaml: documents must be Markdown (.md)']);
        });

        it('rejects an unknown top-level file', async () => {
            const { zip } = buildZip({ extra: [{ name: 'README.md', content: '# Read me' }] });
            expect(rejected(await post(zip))).toEqual(['README.md: unknown top-level entry']);
        });

        it('updates an image already stored in another slot and keeps that slot', async () => {
            const PNG = Buffer.from('89504e470d0a1a0a', 'hex');
            const content = createZip([{ name: 'web/pets.png', content: PNG }]);
            const { handle } = await createApiFromZip();
            const stored = await client.as('publisher').putMultipart(`/apis/${handle}/assets`)
                .field('imageMetadata', JSON.stringify({ 'api-icon': 'pets.png' }))
                .attach('content', content, 'content.zip');
            expect(stored.status).toBeLessThan(300);

            const { zip } = buildZip({ handle, extra: [{ name: 'web/pets.png', content: PNG }] });
            const res = await client.as('publisher').putMultipart(`/apis/${handle}`).attach('artifact', zip, 'artifact.zip');
            expect(res.status).toBe(200);
            const image = await client.as('publisher').get(`/apis/${handle}/assets?type=IMAGE&fileName=pets.png`).responseType('blob');
            expect(image.status).toBe(200);
        });

        it('rejects two images for one slot', async () => {
            const { zip } = buildZip({ extra: [
                { name: 'web/logo.png', content: 'png' },
                { name: 'web/logo.jpg', content: 'jpg' },
            ] });
            expect(rejected(await post(zip))).toEqual(
                ['Images logo.jpg, logo.png use the same image slot logo; name one in imageMetadata']);
        });
    });
});
