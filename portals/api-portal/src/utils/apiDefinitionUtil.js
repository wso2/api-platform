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
const logger = require('../config/logger');
const yaml = require('./yaml');
const constants = require('./constants');

const securitySchemeHasApiKey = (securityScheme) =>
    Array.isArray(securityScheme) && securityScheme.includes('api_key');

const parseApiDefinition = (definition) => {
    if (!definition) {
        return null;
    }
    if (typeof definition !== 'string') {
        return definition;
    }
    try {
        return JSON.parse(definition);
    } catch (jsonError) {
        try {
            return yaml.load(definition);
        } catch (yamlError) {
            logger.warn('parseApiDefinition: failed to parse API definition', {
                error: yamlError.message || jsonError.message
            });
            return null;
        }
    }
};

/**
 * Returns the subscription token header name if the definition declares a parameter with
 * x-header-type: subscription-key, otherwise null.
 */
const findSubscriptionTokenHeader = (apiDefinition) => {
    const parsed = parseApiDefinition(apiDefinition);
    if (!parsed || typeof parsed !== 'object') return null;
    const components = parsed.components || {};
    const params = components.parameters || {};
    for (const param of Object.values(params)) {
        if (param && param['x-header-type'] === 'subscription-key' && param.in === 'header') {
            return param.name || null;
        }
    }
    return null;
};

// API types for which API keys are not offered. WebSub is among them because the
// Platform API's API key webhook does not accept WebSub APIs, so a key generated here
// would never reach the gateway.
const API_KEY_UNSUPPORTED_TYPES = [
    constants.API_TYPE.GRAPHQL,
    constants.API_TYPE.MCP,
    constants.API_TYPE.SOAP,
    constants.API_TYPE.WEBSUB,
];

/**
 * Whether API Key options (sidebar item, API Keys page) are offered for an API.
 *
 * Deliberately independent of the API definition: an OpenAPI spec is not required
 * to declare an apiKey security scheme for users to be able to generate API keys.
 *
 * @param {object} metaData - Metadata in APIM DTO shape
 * @returns {boolean}
 */
function apiUsesApiKeySecurity(metaData) {
    if (!metaData) {
        return false;
    }
    return !API_KEY_UNSUPPORTED_TYPES.includes(metaData.type);
}

module.exports = {
    apiUsesApiKeySecurity,
    findSubscriptionTokenHeader,
    securitySchemeHasApiKey,
};
