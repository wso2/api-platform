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
const assert = require('node:assert');
const Module = require('node:module');
const path = require('node:path');

const CONFIG_LOADER = path.join(__dirname, '..', 'config', 'configLoader.js');
const SETTINGS = path.join(__dirname, 'deploymentSettings.js');

/** deploymentSettings.js as it reads a config whose [api_portal.billing.moesif] is `moesif`. */
function withConfig(moesif) {
    const loader = new Module(CONFIG_LOADER);
    loader.filename = CONFIG_LOADER;
    loader.loaded = true;
    loader.exports = { config: { billing: { moesif } } };
    require.cache[CONFIG_LOADER] = loader;
    delete require.cache[SETTINGS];
    return require(SETTINGS);
}

test.afterEach(() => {
    delete require.cache[CONFIG_LOADER];
    delete require.cache[SETTINGS];
});

test('no keys in config: nothing is supplied', () => {
    const settings = withConfig({ collectorApplicationId: '', managementKey: '' });
    assert.equal(settings.deploymentSettings(), null);
    assert.deepEqual(settings.deploymentFields(), []);
    assert.equal(settings.suppliedByDeployment('moesifManagementKey'), false);
});

test('both keys in config: they are supplied under the billing setup names', () => {
    const settings = withConfig({ collectorApplicationId: 'eyJhcHAiOiI2NDQ3OjEyMyJ9', managementKey: 'moesif-key-0000' });
    assert.deepEqual(settings.deploymentSettings(), {
        moesifCollectorApplicationId: 'eyJhcHAiOiI2NDQ3OjEyMyJ9',
        moesifManagementKey: 'moesif-key-0000',
    });
    assert.deepEqual(settings.deploymentFields(), ['moesifCollectorApplicationId', 'moesifManagementKey']);
    assert.equal(settings.suppliedByDeployment('moesifManagementKey'), true);
    assert.equal(settings.suppliedByDeployment('stripeApiKey'), false);
});

test('no billing section at all: nothing is supplied', () => {
    assert.equal(withConfig(undefined).deploymentSettings(), null);
});
