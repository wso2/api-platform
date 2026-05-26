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
 * "AS IS" BASIS, WITHOUT WARRANTIES OR ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
const util = require('../utils/util');
const logger = require('../config/logger');
const yaml = require('js-yaml');

const PLATFORM_GATEWAY = 'wso2/api-platform';

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
 * x-header-type: subscription-token, otherwise null.
 */
const findSubscriptionTokenHeader = (apiDefinition) => {
    const parsed = parseApiDefinition(apiDefinition);
    if (!parsed || typeof parsed !== 'object') return null;
    const components = parsed.components || {};
    const params = components.parameters || {};
    for (const param of Object.values(params)) {
        if (param && param['x-header-type'] === 'subscription-token' && param.in === 'header') {
            return param.name || null;
        }
    }
    return null;
};

const apiDefinitionHasApiKeySecurity = (apiDefinition) => {
    if (!apiDefinition || typeof apiDefinition !== 'object') {
        return false;
    }

    const securitySchemes = apiDefinition.components?.securitySchemes || apiDefinition.securityDefinitions;
    if (securitySchemes && typeof securitySchemes === 'object') {
        return Object.values(securitySchemes).some((scheme) => {
            if (!scheme || typeof scheme !== 'object') {
                return false;
            }
            const type = String(scheme.type || '').toLowerCase();
            return type === 'apikey' || type === 'httpapikey' || type === 'http_api_key';
        });
    }

    return false;
};

/**
 * API-level Platform API Keys nav: platform gateway and API Key security enabled on the API.
 * If an API definition is available, the decision is driven by its security schemes.
 * Otherwise, it falls back to the control plane securityScheme field.
 *
 * @param {object} req - Express request (for invokeApiRequest auth)
 * @param {object} metaData - Metadata with apiInfo.gatewayType and apiReferenceID (APIM DTO shape)
 * @param {object|null} existingApiDetail - Optional CP GET /apis/:ref response to avoid a duplicate call
 * @param {string|object|null} apiDefinition - Optional raw API definition to inspect for apiKey security
 * @returns {Promise<boolean>}
 */
async function shouldShowPlatformApiKeysNav(req, metaData, existingApiDetail = null, apiDefinition = null) {
    if (!metaData?.apiInfo || metaData.apiInfo.gatewayType !== PLATFORM_GATEWAY) {
        return false;
    }

    if (apiDefinition) {
        const parsedDefinition = parseApiDefinition(apiDefinition);
        return apiDefinitionHasApiKeySecurity(parsedDefinition);
    }

    const refId = metaData.apiReferenceID;
    let detail = existingApiDetail;

    if (detail && detail.securityScheme !== undefined) {
        return securitySchemeHasApiKey(detail.securityScheme);
    }

    // If no API definition or existing API detail is available, don't attempt
    // to call the control plane; fall back to not showing the nav.
    return false;
}

module.exports = {
    shouldShowPlatformApiKeysNav,
    findSubscriptionTokenHeader,
    securitySchemeHasApiKey,
    PLATFORM_GATEWAY
};
