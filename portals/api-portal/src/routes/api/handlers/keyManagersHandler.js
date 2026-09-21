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
 *
 */

/*
 * Tag: Key Managers
 */
const keyManagerService = require('../../../services/keyManagerService');
// getKeyManagerMetadata is tagged "Key Managers" (so it resolves here) but belongs
// to the OAuth2 key-generation feature: the property descriptors it returns are what
// POST /oauth2-keys validates against, so it lives with that service rather than
// with the key manager CRUD.
const oauth2KeyService = require('../../../services/oauth2KeyService');

module.exports = {
    createKeyManager: keyManagerService.createKeyManager,
    getKeyManagers: keyManagerService.getKeyManagers,
    getKeyManager: keyManagerService.getKeyManager,
    updateKeyManager: keyManagerService.updateKeyManager,
    deleteKeyManager: keyManagerService.deleteKeyManager,
    getKeyManagerMetadata: oauth2KeyService.getKeyManagerMetadata,
};
