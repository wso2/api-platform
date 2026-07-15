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

// Organization theming REST API — the /views/{viewId}/(apply-theme|reset-theme|
// export-theme|asset) endpoints (adminService.applyTheme/resetTheme/exportTheme,
// devportalService.getOrgAsset). A theme is a ZIP of view assets (styles/, images/,
// layout/, partials/, …). applyTheme unzips it and atomically replaces the view's
// stored theme rows; getOrgAsset serves one asset back and — like the pre-auth
// login page — is PUBLIC (security: []), resolving the org from the `orgId` query
// param when there is no session. Binary integrity of image assets matters: they are
// stored as BLOBs and served raw, so the tests compare bytes round-tripped through
// the API, not just status codes.
//
// Roles (real scopes via file-based login, not the preauthorized session bypass):
//   admin      → dp:org_content_manage / dp:org_manage  (can theme)
//   publisher  → API scopes only, no org content manage  (403 on theme writes)
//   developer  → read-only                               (403 on theme writes)

const client = require('../support/client');
const db = require('../support/db');
const { createView } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

// A real 1x1 PNG — arbitrary binary whose bytes must survive the BLOB round-trip.
const PNG_BYTES = Buffer.from(
    '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489' +
    '0000000d49444154789c6360000002000100ffff03000006000557bfabd4000000' +
    '0049454e44ae426082',
    'hex',
);

const CSS_BODY = 'body { color: #123456; }';

// A valid theme ZIP wrapped in a top-level folder (readFilesInDirectory strips the
// first path segment on import, mirroring what export-theme produces). Only file
// types the importer recognises are included: .css → style, .png → image.
function buildThemeZip({ imageName = 'brand-mark.png', css = CSS_BODY, image = PNG_BYTES } = {}) {
    return createZip([
        { name: `theme/styles/main.css`, content: css },
        { name: `theme/images/${imageName}`, content: image },
    ]);
}

describe('Organization theming (view theme assets)', () => {
    let orgUuid;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('publisher');
        await client.login('developer');
        // The public getOrgAsset path resolves the view via the orgId query param
        // when no session is present — that value is the internal org uuid.
        orgUuid = await db.findOrgUuidByHandle(client.ORG_HANDLE);
    });

    async function applyThemeAs(role, viewId, zip) {
        return client.as(role).postMultipart(`/views/${viewId}/apply-theme`).attach('file', zip, 'theme.zip');
    }

    describe('apply-theme + asset retrieval', () => {
        it('applies a theme ZIP and serves back the image asset with matching bytes', async () => {
            const { id: viewId } = await createView();
            const apply = await applyThemeAs('admin', viewId, buildThemeZip());
            expect(apply.status).toBe(200);

            const img = await client.as('admin')
                .get(`/views/${viewId}/asset?fileType=image&fileName=brand-mark.png`)
                .responseType('blob');
            expect(img.status).toBe(200);
            expect(img.headers['content-type']).toContain('image/png');
            expect(Buffer.compare(img.body, PNG_BYTES)).toBe(0);
        });

        it('serves back the style asset as CSS', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);

            const css = await client.as('admin').get(`/views/${viewId}/asset?fileType=style&fileName=main.css`);
            expect(css.status).toBe(200);
            expect(css.headers['content-type']).toContain('text/css');
            expect(String(css.text || css.body)).toContain('#123456');
        });

        it('replaces the previous theme wholesale on re-apply', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip({ imageName: 'old.png' }))).status).toBe(200);
            // Re-apply with a different image name — the old asset must be gone.
            expect((await applyThemeAs('admin', viewId, buildThemeZip({ imageName: 'new.png' }))).status).toBe(200);

            const oldAsset = await client.as('admin').get(`/views/${viewId}/asset?fileType=image&fileName=old.png`);
            expect(oldAsset.status).toBe(404);
            const newAsset = await client.as('admin').get(`/views/${viewId}/asset?fileType=image&fileName=new.png`).responseType('blob');
            expect(newAsset.status).toBe(200);
        });

        it('rejects a ZIP containing an unsupported file type', async () => {
            const { id: viewId } = await createView();
            const badZip = createZip([{ name: 'theme/evil.exe', content: Buffer.from([0x4d, 0x5a]) }]);
            const res = await applyThemeAs('admin', viewId, badZip);
            expect(res.status).toBe(400);
        });
    });

    describe('public asset access (no session)', () => {
        it('serves the image asset to an anonymous caller that supplies orgId', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);

            const res = await client.raw()
                .get(`${client.API_PREFIX}/views/${viewId}/asset?fileType=image&fileName=brand-mark.png&orgId=${orgUuid}`)
                .responseType('blob');
            expect(res.status).toBe(200);
            expect(res.headers['content-type']).toContain('image/png');
            expect(Buffer.compare(res.body, PNG_BYTES)).toBe(0);
        });

        it('falls back to default content (not the custom asset) for an anonymous caller without orgId', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);

            // No orgId + no session → the view-specific asset cannot be resolved, so it
            // falls through to packaged default content, which has no brand-mark.png → 404.
            const res = await client.raw()
                .get(`${client.API_PREFIX}/views/${viewId}/asset?fileType=image&fileName=brand-mark.png`);
            expect(res.status).toBe(404);
        });
    });

    describe('export-theme', () => {
        it('exports the applied theme as a ZIP archive', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);

            const res = await client.as('admin').get(`/views/${viewId}/export-theme`).responseType('blob');
            expect(res.status).toBe(200);
            expect(res.headers['content-type']).toContain('application/zip');
            expect(res.body.length).toBeGreaterThan(0);
            // ZIP local-file-header magic — proves a real archive, not an error page.
            expect(res.body.slice(0, 2).toString('latin1')).toBe('PK');
        });
    });

    describe('reset-theme', () => {
        it('reverts to defaults so the custom asset is no longer served', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);
            // Sanity: custom asset present before reset.
            expect((await client.as('admin').get(`/views/${viewId}/asset?fileType=image&fileName=brand-mark.png`).responseType('blob')).status).toBe(200);

            const reset = await client.as('admin').post(`/views/${viewId}/reset-theme`);
            expect(reset.status).toBe(204);

            const after = await client.as('admin').get(`/views/${viewId}/asset?fileType=image&fileName=brand-mark.png`);
            expect(after.status).toBe(404);
        });
    });

    describe('authorization', () => {
        it('forbids a publisher (no org content scope) from applying a theme', async () => {
            const { id: viewId } = await createView();
            const res = await applyThemeAs('publisher', viewId, buildThemeZip());
            expect(res.status).toBe(403);
        });

        it('forbids a read-only developer from applying a theme', async () => {
            const { id: viewId } = await createView();
            const res = await applyThemeAs('developer', viewId, buildThemeZip());
            expect(res.status).toBe(403);
        });

        it('forbids a developer from resetting a theme', async () => {
            const { id: viewId } = await createView();
            expect((await applyThemeAs('admin', viewId, buildThemeZip())).status).toBe(200);
            const res = await client.as('developer').post(`/views/${viewId}/reset-theme`);
            expect(res.status).toBe(403);
        });
    });
});
