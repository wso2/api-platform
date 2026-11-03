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

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');
const express = require('express');
const OpenApiValidator = require('express-openapi-validator');

const SPEC_PATH = path.join(__dirname, '..', '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml');
const PLANS_PATH = '/api-portal/api/v0.9/subscription-plans';

let server;

// Same request-validation settings as apiPortalRouter, minus auth and handlers.
test.before(async () => {
    const app = express();
    app.use(OpenApiValidator.middleware({
        apiSpec: SPEC_PATH,
        validateRequests: { allowUnknownQueryParameters: false },
        validateResponses: false,
        validateSecurity: false,
    }));
    app.get(PLANS_PATH, (req, res) => res.json({}));
    await new Promise((resolve) => { server = app.listen(0, resolve); });
});

test.after(() => server.close());

function getStatus(query) {
    return new Promise((resolve, reject) => {
        http.get({ port: server.address().port, path: PLANS_PATH + query }, (res) => {
            res.resume();
            res.on('end', () => resolve(res.statusCode));
        }).on('error', reject);
    });
}

test('listing subscription plans accepts limit and offset within range', async () => {
    for (const query of ['', '?limit=5&offset=0', '?limit=1', '?limit=100', '?offset=1000', '?limit=100&offset=1000']) {
        assert.equal(await getStatus(query), 200, query);
    }
});

test('listing subscription plans rejects limit and offset outside their range', async () => {
    for (const query of ['?limit=0', '?limit=101', '?limit=-1', '?limit=abc', '?limit=1.5', '?offset=-1', '?offset=abc', '?offset=1.5']) {
        assert.equal(await getStatus(query), 400, query);
    }
});
