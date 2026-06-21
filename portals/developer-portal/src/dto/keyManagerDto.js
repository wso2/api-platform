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

/**
 * DTO for key manager responses.
 * Never exposes admin credentials in API responses.
 */
class KeyManagerDTO {
    constructor(km) {
        this.id = km.KM_ID;
        this.orgId = km.ORG_ID;
        this.name = km.NAME;
        this.type = km.TYPE;
        this.enabled = km.ENABLED;
        this.tokenEndpoint = km.TOKEN_ENDPOINT;
        this.clientRegistrationEndpoint = km.CLIENT_REG_ENDPOINT;
        if (km.ISSUER) {
            this.issuer = km.ISSUER;
        }
        if (km.JWKS_URL) {
            this.jwksURL = km.JWKS_URL;
        }
        this.supportedGrantTypes = km.SUPPORTED_GRANT_TYPES || ['client_credentials'];
        this.supportedScopes = km.SUPPORTED_SCOPES || ['openid'];
        this.additionalProperties = km.ADDITIONAL_PROPERTIES || {};
    }
}

/**
 * Minimal DTO for developer-facing key manager listing.
 * Only includes information developers need when selecting a KM.
 */
class KeyManagerPublicDTO {
    constructor(km) {
        this.id = km.KM_ID;
        this.name = km.NAME;
        this.type = km.TYPE;
        this.tokenEndpoint = km.TOKEN_ENDPOINT;
        this.supportedGrantTypes = km.SUPPORTED_GRANT_TYPES || ['client_credentials'];
        this.supportedScopes = km.SUPPORTED_SCOPES || ['openid'];
    }
}

module.exports = { KeyManagerDTO, KeyManagerPublicDTO };
