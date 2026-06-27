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
const { decryptCredentials } = require('../../dao/keyManagerDao');
const AsgardeoAdapter = require('./asgardeoAdapter');

const ADAPTER_MAP = {
    ASGARDEO: AsgardeoAdapter,
    // WSO2IS: require('./wso2isAdapter'),
    // KEYCLOAK: require('./keycloakAdapter'),
    // GENERIC_OIDC: require('./genericOIDCAdapter'),
};

/**
 * Factory function that returns the appropriate adapter for a key manager record.
 *
 * @param {object} kmRecord - Raw DP_KEY_MANAGER Sequelize instance.
 * @returns {BaseKeyManagerAdapter} Concrete adapter instance ready to make AS calls.
 */
function getKeyManagerAdapter(kmRecord) {
    const AdapterClass = ADAPTER_MAP[kmRecord.TYPE];
    if (!AdapterClass) {
        throw new Error(`Unsupported key manager type: ${kmRecord.TYPE}`);
    }
    const credentials = decryptCredentials(kmRecord);
    return new AdapterClass(kmRecord, credentials);
}

const SUPPORTED_KM_TYPES = Object.keys(ADAPTER_MAP);

module.exports = { getKeyManagerAdapter, SUPPORTED_KM_TYPES };
