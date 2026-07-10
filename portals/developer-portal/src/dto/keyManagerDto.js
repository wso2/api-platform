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
const { applyAudit } = require('./dtoUtils');

/**
 * DTO for key manager responses.
 * Never exposes admin credentials in API responses.
 */
class KeyManagerDTO {
    constructor(km, audit) {
        this.id = km.handle;
        this.displayName = km.display_name;
        this.orgId = km.org_uuid;
        this.type = km.type;
        this.enabled = !!km.enabled;
        this.tokenEndpoint = km.token_endpoint;
        applyAudit(this, audit);
    }
}

/**
 * Minimal DTO for developer-facing key manager listing.
 * Only includes information developers need when selecting a KM.
 */
class KeyManagerPublicDTO {
    constructor(km) {
        this.id = km.handle;
        this.displayName = km.display_name;
        this.type = km.type;
        this.tokenEndpoint = km.token_endpoint;
    }
}

module.exports = { KeyManagerDTO, KeyManagerPublicDTO };
