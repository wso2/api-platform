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
const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

// The util imports the logger, which needs a config file at load time. Replace it with a no-op.
const loggerFile = require.resolve(path.join(__dirname, '../config/logger.js'));
require.cache[loggerFile] = {
    id: loggerFile,
    filename: loggerFile,
    loaded: true,
    exports: { warn() {}, info() {}, debug() {}, error() {} },
};

const constants = require('./constants');
const { apiUsesApiKeySecurity } = require('./apiDefinitionUtil');

test('API keys are offered for a REST API regardless of the spec security schemes', () => {
    assert.strictEqual(apiUsesApiKeySecurity({ type: constants.API_TYPE.REST }), true);
});

test('API keys are offered for an API with no type set', () => {
    assert.strictEqual(apiUsesApiKeySecurity({}), true);
});

test('API keys are not offered for GraphQL, MCP, SOAP and WebSub APIs', () => {
    for (const type of [constants.API_TYPE.GRAPHQL, constants.API_TYPE.MCP, constants.API_TYPE.SOAP,
        constants.API_TYPE.WEBSUB]) {
        assert.strictEqual(apiUsesApiKeySecurity({ type }), false, type);
    }
});

test('API keys are not offered when there is no API metadata', () => {
    assert.strictEqual(apiUsesApiKeySecurity(null), false);
});
