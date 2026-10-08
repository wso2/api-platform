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
const { compose } = require('./compose');
const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');

module.exports = {
    // CSRF-guarded: this operation also accepts multipart/form-data (a KeyManager
    // YAML upload), and multipart is one of the three content types a plain HTML
    // form can send — so a cross-site form needs no CORS preflight to reach it.
    // Without this, a page the admin merely visits could create a key manager in
    // their organization pointing at endpoints the attacker chose.
    //
    // PUT and DELETE below need no such guard: a form cannot issue those methods,
    // and a scripted request that does is preflighted and refused — the portal
    // sends no CORS headers at all.
    createKeyManager: compose(requireCsrfForMutatingApi, keyManagerService.createKeyManager),
    getKeyManagers: keyManagerService.getKeyManagers,
    getKeyManager: keyManagerService.getKeyManager,
    updateKeyManager: keyManagerService.updateKeyManager,
    deleteKeyManager: keyManagerService.deleteKeyManager,
    // CSRF-guarded: this is the one operation here that makes the portal dial a
    // host named in the request body, so a cross-site POST riding an admin's
    // session cookie would be an SSRF probe of the deployment's own network.
    // The session cookie sets no SameSite of its own, so the browser default is
    // all that would otherwise stand in the way.
    discoverKeyManagerEndpoints:
        compose(requireCsrfForMutatingApi, keyManagerService.discoverKeyManagerEndpoints),
    getKeyManagerMetadata: oauth2KeyService.getKeyManagerMetadata,
};
