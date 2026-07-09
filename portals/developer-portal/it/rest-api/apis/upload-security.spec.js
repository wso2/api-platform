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

// Upload / archive-extraction limits for the `artifact` full-ZIP path of
// POST /apis (apiMetadataService.js -> util.unzipDirectory): entry-name
// containment, entry-count cap, and the multipart size limit (HTTP 413).
//
// Limits are config-sourced (configDefaults.js `uploads`); this suite assumes
// the defaults that it/test-config.toml does not override: maxBytes = 10 MiB,
// maxZipEntries = 500.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

const MAX_ZIP_ENTRIES = 500;      // configDefaults.js uploads.maxZipEntries
const MAX_UPLOAD_BYTES = 10485760; // configDefaults.js uploads.maxBytes (10 MiB)

const SAMPLE_DEFINITION = JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} });

function buildApiYaml(handle) {
    return `metadata:\n  name: ${handle}\nspec:\n  displayName: "Upload Security ${handle}"\n  version: "v1.0"\n  description: "upload-security fixture"\n  type: REST\n  status: PUBLISHED\n  endpoints:\n    sandboxUrl: https://sandbox.example.invalid/${handle}\n    productionUrl: https://backend.example.invalid/${handle}\n`;
}

// A structurally valid artifact (api.yaml + definition) so that any rejection is
// attributable to the security guard under test, not to a missing bundle file.
function validArtifactEntries(handle) {
    return [
        { name: 'api.yaml', content: buildApiYaml(handle) },
        { name: 'definition.json', content: SAMPLE_DEFINITION },
    ];
}

describe('artifact ZIP upload — file-access hardening', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('rejects an artifact ZIP whose entry name escapes the extraction root (zip slip)', async () => {
        const handle = uniqueHandle('zip-slip');
        const zip = createZip([
            ...validArtifactEntries(handle),
            // Entry that escapes the extraction root — must be refused.
            { name: '../evil.txt', content: 'x' },
        ]);
        const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        expect(res.status).toBe(400);
        // The API must not have been created despite carrying a valid api.yaml.
        const get = await client.as('publisher').get(`/apis/${handle}`);
        expect(get.status).toBe(404);
    });

    it('rejects an artifact ZIP that exceeds the maximum entry count', async () => {
        const handle = uniqueHandle('zip-many');
        const entries = validArtifactEntries(handle);
        for (let i = 0; i < MAX_ZIP_ENTRIES + 20; i += 1) {
            entries.push({ name: `filler/file-${i}.txt`, content: 'x' });
        }
        const zip = createZip(entries);
        const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        expect(res.status).toBe(400);
        // The API must not have been created despite carrying a valid api.yaml.
        const get = await client.as('publisher').get(`/apis/${handle}`);
        expect(get.status).toBe(404);
    });

    it('rejects a multipart upload larger than the configured limit with HTTP 413', async () => {
        // One entry over the ceiling pushes the whole multipart body past maxBytes.
        const oversized = Buffer.alloc(MAX_UPLOAD_BYTES + 1024 * 1024, 0x41); // +1 MiB
        const zip = createZip([{ name: 'big.bin', content: oversized }]);
        const res = await client.as('publisher').postMultipart('/apis').attach('artifact', zip, 'artifact.zip');
        expect(res.status).toBe(413);
        // The configured limit must not be disclosed in any representation — raw byte
        // count nor common human-readable forms.
        const body = JSON.stringify(res.body);
        for (const form of [String(MAX_UPLOAD_BYTES), '10 MB', '10MB', '10 MiB', '10MiB', '10240']) {
            expect(body).not.toContain(form);
        }
    }, 30000);
});
