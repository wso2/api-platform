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
// The `docs` file parts on POST/PUT /apis: each is stored as a DOC_Other document,
// and only Markdown (.md) is accepted, because the docs page opens <name>.md.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const DEFINITION = JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} });
const MESSAGE = 'diagram.txt: documents must be Markdown (.md)';

const metadata = (id) => JSON.stringify({
    id,
    name: `Docs API ${id}`,
    version: 'v1.0',
    type: 'REST',
    status: 'PUBLISHED',
    endPoints: { productionURL: `https://backend.example.invalid/${id}` },
});

const create = (id) => client.as('publisher').postMultipart('/apis')
    .field('metadata', metadata(id))
    .attach('definition', Buffer.from(DEFINITION), 'definition.json');

const update = (id) => client.as('publisher').putMultipart(`/apis/${id}`)
    .field('metadata', metadata(id));

const getDoc = (id, fileName) => client.as('publisher')
    .get(`/apis/${id}/assets?type=DOC_Other&fileName=${fileName}`);

describe('API documents in the save request', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('stores Markdown documents on create and update', async () => {
        const id = uniqueHandle('docs');
        expect((await create(id).attach('docs', Buffer.from('# A'), 'A.md')).status).toBe(201);
        expect((await update(id).attach('docs', Buffer.from('# B'), 'B.md')).status).toBe(200);
        expect((await getDoc(id, 'A.md')).status).toBe(200);
        expect((await getDoc(id, 'B.md')).status).toBe(200);
    });

    it('rejects a document that is not .md on create, and creates nothing', async () => {
        const id = uniqueHandle('docs');
        const res = await create(id)
            .attach('docs', Buffer.from('# A'), 'A.md')
            .attach('docs', Buffer.from('x'), 'diagram.txt');
        expect(res.status).toBe(400);
        expect(res.body.errors).toEqual([{ message: MESSAGE }]);
        expect((await client.as('publisher').get(`/apis/${id}`)).status).toBe(404);
    });

    it('rejects a document that is not .md on update, and changes nothing', async () => {
        const id = uniqueHandle('docs');
        expect((await create(id).attach('docs', Buffer.from('# A'), 'A.md')).status).toBe(201);
        const res = await update(id)
            .attach('docs', Buffer.from('# B'), 'B.md')
            .attach('docs', Buffer.from('x'), 'diagram.txt');
        expect(res.status).toBe(400);
        expect(res.body.errors).toEqual([{ message: MESSAGE }]);
        expect((await getDoc(id, 'A.md')).status).toBe(200);
        expect((await getDoc(id, 'B.md')).status).toBe(404);
    });
});
