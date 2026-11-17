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
 */

'use strict';

/*
 * API keys are refused for WebSub APIs: Platform API's API key webhook does not accept
 * them, so a generated or regenerated key would be returned but never reach the gateway.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

let apiType;          // type of the API the handle resolves to
let serviceCalls;     // apiKeyService calls made

stub(local('../config/logger.js'), { warn: () => {}, error: () => {}, info: () => {}, debug: () => {} });
stub(local('../middlewares/auditLogger.js'), { logUserAction: () => {} });
stub(local('../utils/util.js'), {
    resolveActor: () => 'alice',
    sendError: (res, status, message, body) => res.status(status).json({ message, ...body }),
});
stub(local('../dao/apiDao.js'), {
    getIdExcludingType: async () => 'api-1',
    getIdByType: async (_org, _handle, type) => (type === apiType ? 'api-1' : undefined),
});
stub(local('../dao/apiKeyDao.js'), { getIdByHandle: async () => 'key-1' });
stub(local('../dao/applicationDao.js'), { getId: async () => undefined });
stub(local('../dao/userIdpReferenceDao.js'), {});
stub(local('../services/apiKeyService.js'), {
    generate: async () => { serviceCalls.push('generate'); return { keyId: 'k', id: 'key' }; },
    regenerate: async () => { serviceCalls.push('regenerate'); return { keyId: 'k', id: 'key' }; },
});

const constants = require('../utils/constants');
const { generateApiKey, regenerateApiKey } = require('./apiKeyController');

function call(handler, body) {
    const res = {
        statusCode: 0, body: undefined,
        status(code) { this.statusCode = code; return this; },
        json(b) { this.body = b; return this; },
    };
    return handler({ orgId: 'org-1', params: { apiId: 'weather' }, body, user: {} }, res).then(() => res);
}

test('generating a key for a WebSub API is refused before the service is called', async () => {
    apiType = constants.API_TYPE.WEBSUB; serviceCalls = [];
    const res = await call(generateApiKey, { id: 'key' });
    assert.equal(res.statusCode, 400);
    assert.deepEqual(serviceCalls, []);
});

test('regenerating a key for a WebSub API is refused before the service is called', async () => {
    apiType = constants.API_TYPE.WEBSUB; serviceCalls = [];
    const res = await call(regenerateApiKey, { keyId: 'key' });
    assert.equal(res.statusCode, 400);
    assert.deepEqual(serviceCalls, []);
});

test('a REST API still gets its key', async () => {
    apiType = constants.API_TYPE.REST; serviceCalls = [];
    const res = await call(generateApiKey, { id: 'key' });
    assert.equal(res.statusCode, 201);
    assert.deepEqual(serviceCalls, ['generate']);
});
