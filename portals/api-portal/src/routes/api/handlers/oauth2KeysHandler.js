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
 * Tag: OAuth2 Keys
 *
 * Mutating ops are CSRF-protected.
 *
 * The service behind these is still a stub — see src/services/oauth2KeyService.js.
 */
const oauth2KeyService = require('../../../services/oauth2KeyService');
const { requireCsrfForMutatingApi } = require('../../../middlewares/csrfProtection');
const { compose } = require('./compose');

module.exports = {
    createOAuth2Key: compose(requireCsrfForMutatingApi, oauth2KeyService.createOAuth2Key),
    listOAuth2Keys: oauth2KeyService.listOAuth2Keys,
    getOAuth2Key: oauth2KeyService.getOAuth2Key,
    updateOAuth2Key: compose(requireCsrfForMutatingApi, oauth2KeyService.updateOAuth2Key),
    deleteOAuth2Key: compose(requireCsrfForMutatingApi, oauth2KeyService.deleteOAuth2Key),
    // CSRF-protected like every other POST here: it is a state-changing request
    // that carries a credential, even though it creates nothing in the portal.
    generateOAuth2KeyToken: compose(
        requireCsrfForMutatingApi,
        oauth2KeyService.generateOAuth2KeyToken
    ),
    associateOAuth2KeyApplication: compose(
        requireCsrfForMutatingApi,
        oauth2KeyService.associateOAuth2KeyApplication
    ),
    dissociateOAuth2KeyApplication: compose(
        requireCsrfForMutatingApi,
        oauth2KeyService.dissociateOAuth2KeyApplication
    ),
    listApplicationOAuth2Keys: oauth2KeyService.listApplicationOAuth2Keys,
};
